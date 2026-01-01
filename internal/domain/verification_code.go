package domain

import "time"

// VerificationCode represents email verification code in database
type VerificationCode struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	Email     string    `json:"email" gorm:"type:varchar(255);not null;index:idx_verification_code_email"`
	Code      string    `json:"code" gorm:"type:varchar(6);not null;index:idx_verification_code_email"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null;index:idx_verification_code_expires"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for VerificationCode
func (VerificationCode) TableName() string {
	return "verification_codes"
}

// IsExpired checks if the verification code has expired
func (v *VerificationCode) IsExpired() bool {
	return time.Now().After(v.ExpiresAt)
}

// VerificationCodeRequest represents a request to send verification code
type VerificationCodeRequest struct {
	Name  string `json:"name" binding:"required,max=100"`
	Email string `json:"email" binding:"required,email"`
}

// VerificationCodeResponse represents verification code response
type VerificationCodeResponse struct {
	Message string `json:"message"`
}

// VerifyEmailRequest represents email verification request
type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// VerifyEmailResponse represents email verification response
type VerifyEmailResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// ResendVerificationRequest represents resend verification email request
type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}
