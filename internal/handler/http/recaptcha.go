package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const recaptchaVerifyURL = "https://www.google.com/recaptcha/api/siteverify"

// RecaptchaResponse represents Google's reCAPTCHA API response
type RecaptchaResponse struct {
	Success     bool      `json:"success"`
	Score       float64   `json:"score"`
	Action      string    `json:"action"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

// RecaptchaVerifier handles reCAPTCHA verification
type RecaptchaVerifier struct {
	secretKey string
	threshold float64
	client    *http.Client
}

// NewRecaptchaVerifier creates a new reCAPTCHA verifier
func NewRecaptchaVerifier(secretKey string, threshold float64) *RecaptchaVerifier {
	return &RecaptchaVerifier{
		secretKey: secretKey,
		threshold: threshold,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Verify verifies a reCAPTCHA token
func (r *RecaptchaVerifier) Verify(token string, remoteIP string) (*RecaptchaResponse, error) {
	if token == "" {
		return nil, fmt.Errorf("recaptcha token is empty")
	}

	// Prepare request
	reqBody := map[string]string{
		"secret":   r.secretKey,
		"response": token,
	}

	if remoteIP != "" {
		reqBody["remoteip"] = remoteIP
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make request to Google
	resp, err := r.client.Post(recaptchaVerifyURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to verify recaptcha: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read recaptcha response: %w", err)
	}

	// Parse response
	var recaptchaResp RecaptchaResponse
	if err := json.Unmarshal(body, &recaptchaResp); err != nil {
		return nil, fmt.Errorf("failed to parse recaptcha response: %w", err)
	}

	return &recaptchaResp, nil
}

// RecaptchaMiddleware creates a middleware that validates reCAPTCHA tokens
func RecaptchaMiddleware(secretKey string, threshold float64) gin.HandlerFunc {
	// If secret key is empty or disabled, skip verification
	if secretKey == "" || secretKey == "disabled" || secretKey == "your_secret_key" {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	verifier := NewRecaptchaVerifier(secretKey, threshold)

	return func(c *gin.Context) {
		// Try to get token from different sources
		var token string

		// 1. Try from request body (for JSON requests)
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err == nil {
			if t, ok := body["recaptcha_token"].(string); ok {
				token = t
			}
			// Rebind the body for the next handler
			c.Set("requestBody", body)
		}

		// 2. Try from header
		if token == "" {
			token = c.GetHeader("X-Recaptcha-Token")
		}

		// 3. Try from query parameter
		if token == "" {
			token = c.Query("recaptcha_token")
		}

		// If no token provided, allow request to continue (optional verification)
		// The handler can decide if it's required
		if token == "" {
			c.Next()
			return
		}

		// Verify token
		recaptchaResp, err := verifier.Verify(token, c.ClientIP())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "reCAPTCHA verification failed",
				"message": "Failed to verify reCAPTCHA token",
			})
			c.Abort()
			return
		}

		// Check if verification was successful
		if !recaptchaResp.Success {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":       "reCAPTCHA verification failed",
				"message":     "reCAPTCHA verification was not successful",
				"error_codes": recaptchaResp.ErrorCodes,
			})
			c.Abort()
			return
		}

		// Check score threshold (reCAPTCHA v3)
		if recaptchaResp.Score < threshold {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Suspicious activity detected",
				"message": "Your request appears to be automated. Please try again later.",
			})
			c.Abort()
			return
		}

		// Store score in context for logging/monitoring
		c.Set("recaptcha_score", recaptchaResp.Score)
		c.Set("recaptcha_verified", true)

		c.Next()
	}
}

// OptionalRecaptchaMiddleware creates a middleware that validates reCAPTCHA tokens but doesn't block requests
func OptionalRecaptchaMiddleware(secretKey string, threshold float64) gin.HandlerFunc {
	if secretKey == "" || secretKey == "disabled" || secretKey == "your_secret_key" {
		return func(c *gin.Context) {
			c.Set("recaptcha_verified", false)
			c.Next()
		}
	}

	verifier := NewRecaptchaVerifier(secretKey, threshold)

	return func(c *gin.Context) {
		// Try to get token
		token := c.GetHeader("X-Recaptcha-Token")
		if token == "" {
			token = c.Query("recaptcha_token")
		}

		if token == "" {
			c.Set("recaptcha_verified", false)
			c.Next()
			return
		}

		// Verify token
		recaptchaResp, err := verifier.Verify(token, c.ClientIP())
		if err != nil || !recaptchaResp.Success || recaptchaResp.Score < threshold {
			c.Set("recaptcha_verified", false)
			c.Set("recaptcha_score", 0.0)
		} else {
			c.Set("recaptcha_verified", true)
			c.Set("recaptcha_score", recaptchaResp.Score)
		}

		c.Next()
	}
}
