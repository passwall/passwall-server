package domain

// AdminCreateUserRequest represents an admin-created user provisioning request.
//
// This is similar to SignUpRequest, but:
// - It is only accepted on authenticated admin endpoints (/api/users)
// - The server will mark the created user as verified (IsVerified=true) and will not send verification email
// - It allows setting the user's role via role_id
type AdminCreateUserRequest struct {
	Name               string     `json:"name" validate:"max=100"`
	Email              string     `json:"email" validate:"required,email"`
	MasterPasswordHash string     `json:"master_password_hash" validate:"required"` // HKDF(masterKey, info="auth"), base64
	ProtectedUserKey   string     `json:"protected_user_key" validate:"required"`   // EncString: "2.iv|ct|mac"
	KdfConfig          *KdfConfig `json:"kdf_config" validate:"required"`
	KdfSalt            string     `json:"kdf_salt" validate:"required"` // hex-encoded random salt from client
	RoleID             uint       `json:"role_id"`                      // optional (defaults to member)
}

func (r *AdminCreateUserRequest) Validate() error {
	// Reuse the signup validation rules for the cryptographic fields
	su := &SignUpRequest{
		Name:               r.Name,
		Email:              r.Email,
		MasterPasswordHash: r.MasterPasswordHash,
		ProtectedUserKey:   r.ProtectedUserKey,
		KdfConfig:          r.KdfConfig,
		KdfSalt:            r.KdfSalt,
	}
	return su.Validate()
}

// AdminInviteUserRequest represents an admin invitation request.
// This does not create the user; it only sends an invitation email.
type AdminInviteUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required"` // "admin" | "member" (UI hint)
	Desc  string `json:"desc,omitempty"`           // optional note included in email
}
