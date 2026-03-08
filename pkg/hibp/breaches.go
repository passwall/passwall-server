package hibp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	baseURL   = "https://haveibeenpwned.com/api/v3"
	userAgent = "Passwall-Server"
)

// Breach represents a single data breach from the HIBP API.
type Breach struct {
	Name         string   `json:"Name"`
	Title        string   `json:"Title"`
	Domain       string   `json:"Domain"`
	BreachDate   string   `json:"BreachDate"`
	AddedDate    string   `json:"AddedDate"`
	ModifiedDate string   `json:"ModifiedDate"`
	PwnCount     int      `json:"PwnCount"`
	Description  string   `json:"Description"`
	LogoPath     string   `json:"LogoPath"`
	DataClasses  []string `json:"DataClasses"`
	IsVerified   bool     `json:"IsVerified"`
	IsFabricated bool     `json:"IsFabricated"`
	IsSensitive  bool     `json:"IsSensitive"`
	IsRetired    bool     `json:"IsRetired"`
	IsSpamList   bool     `json:"IsSpamList"`
	IsMalware    bool     `json:"IsMalware"`
}

// Client communicates with the HIBP v3 API.
type Client struct {
	apiKey     string
	httpClient *http.Client
	mu         sync.Mutex
	lastCall   time.Time
	rateDelay  time.Duration
	maxRetries int
}

// NewClient creates a new HIBP API client.
// apiKey is required for the breached-account endpoint.
// rateDelayMs controls the minimum delay between API calls (HIBP recommends >= 1500ms).
func NewClient(apiKey string, rateDelayMs int, maxRetries int) *Client {
	if rateDelayMs < 1500 {
		rateDelayMs = 1500
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		rateDelay:  time.Duration(rateDelayMs) * time.Millisecond,
		maxRetries: maxRetries,
	}
}

// CheckBreachedAccount queries HIBP for breaches associated with the given email.
// Returns an empty slice (not an error) if no breaches are found (HTTP 404).
func (c *Client) CheckBreachedAccount(email string) ([]Breach, error) {
	endpoint := fmt.Sprintf("%s/breachedaccount/%s?truncateResponse=false",
		baseURL, url.PathEscape(email))

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		c.rateLimit()

		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("hibp: failed to create request: %w", err)
		}
		req.Header.Set("hibp-api-key", c.apiKey)
		req.Header.Set("User-Agent", userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == c.maxRetries {
				return nil, fmt.Errorf("hibp: request failed after retries: %w", err)
			}
			time.Sleep(backoffDelay(attempt))
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			var breaches []Breach
			decodeErr := json.NewDecoder(resp.Body).Decode(&breaches)
			resp.Body.Close()
			if decodeErr != nil {
				return nil, fmt.Errorf("hibp: failed to decode response: %w", decodeErr)
			}
			return breaches, nil
		case http.StatusNotFound:
			resp.Body.Close()
			return []Breach{}, nil
		case http.StatusTooManyRequests:
			waitFor := parseRetryAfter(resp.Header.Get("Retry-After"), backoffDelay(attempt))
			resp.Body.Close()
			if attempt == c.maxRetries {
				return nil, fmt.Errorf("hibp: rate limited after retries (retry-after: %s)", waitFor.String())
			}
			time.Sleep(waitFor)
			continue
		case http.StatusUnauthorized:
			resp.Body.Close()
			return nil, fmt.Errorf("hibp: unauthorized — check API key")
		default:
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			if isRetryableStatus(resp.StatusCode) && attempt < c.maxRetries {
				time.Sleep(backoffDelay(attempt))
				continue
			}
			return nil, fmt.Errorf("hibp: unexpected status %d: %s", resp.StatusCode, string(body))
		}
	}

	return nil, fmt.Errorf("hibp: exhausted retries")
}

// Enabled reports whether the client has an API key configured.
func (c *Client) Enabled() bool {
	return c.apiKey != ""
}

// rateLimit ensures we don't exceed the HIBP API rate limit.
func (c *Client) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	elapsed := time.Since(c.lastCall)
	if elapsed < c.rateDelay {
		time.Sleep(c.rateDelay - elapsed)
	}
	c.lastCall = time.Now()
}

func backoffDelay(attempt int) time.Duration {
	// 1s, 2s, 4s ... capped at 30s
	delay := time.Second * time.Duration(1<<attempt)
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}

func parseRetryAfter(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}

	if secs, err := strconv.Atoi(header); err == nil {
		d := time.Duration(secs) * time.Second
		if d <= 0 {
			return fallback
		}
		return d
	}

	if t, err := http.ParseTime(header); err == nil {
		d := time.Until(t)
		if d <= 0 {
			return fallback
		}
		return d
	}

	return fallback
}

func isRetryableStatus(code int) bool {
	return code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || (code >= 500 && code <= 599)
}
