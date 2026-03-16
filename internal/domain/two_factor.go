package domain

// TwoFactorRequiredResponse is returned by SignIn when the user has 2FA enabled.
// Clients must call /auth/2fa/verify with the temporary token + TOTP code.
type TwoFactorRequiredResponse struct {
	RequiresTwoFactor bool   `json:"requires_two_factor"`
	TwoFactorToken    string `json:"two_factor_token"`
}

// TwoFactorVerifyRequest is sent by the client to complete 2FA during sign-in.
type TwoFactorVerifyRequest struct {
	TwoFactorToken string `json:"two_factor_token" binding:"required"`
	TOTPCode       string `json:"totp_code" binding:"required"`
}

// TwoFactorSetupResponse is returned when a user initiates 2FA setup.
type TwoFactorSetupResponse struct {
	Secret        string   `json:"secret"`
	QRCodeURL     string   `json:"qr_code_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// TwoFactorConfirmRequest is sent to finalize 2FA setup after scanning QR.
type TwoFactorConfirmRequest struct {
	TOTPCode string `json:"totp_code" binding:"required"`
}

// TwoFactorDisableRequest is sent to disable 2FA on the user's account.
type TwoFactorDisableRequest struct {
	MasterPasswordHash string `json:"master_password_hash" binding:"required"`
	TOTPCode           string `json:"totp_code" binding:"required"`
}

// TwoFactorStatusResponse returns the user's current 2FA status.
type TwoFactorStatusResponse struct {
	Enabled bool `json:"enabled"`
}
