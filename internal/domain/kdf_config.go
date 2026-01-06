package domain

import "errors"

// KdfConfig represents KDF (Key Derivation Function) configuration
type KdfConfig struct {
	Type        KdfType `json:"kdf_type"`
	Iterations  int     `json:"kdf_iterations"`
	Memory      *int    `json:"kdf_memory,omitempty"`      // For Argon2id (MB)
	Parallelism *int    `json:"kdf_parallelism,omitempty"` // For Argon2id (threads)
	Salt        string  `json:"kdf_salt,omitempty"`        // hex-encoded random salt
}

// PBKDF2 defaults (OWASP 2023 recommendations)
const (
	PBKDF2MinIterations     = 600000  // Minimum (security)
	PBKDF2DefaultIterations = 600000  // Default (balanced)
	PBKDF2MaxIterations     = 2000000 // Maximum (high security)
)

// Argon2id defaults (industry standard)
const (
	Argon2DefaultIterations  = 3  // Time cost
	Argon2DefaultMemory      = 64 // 64 MB
	Argon2DefaultParallelism = 4  // 4 threads

	Argon2MinIterations  = 2
	Argon2MaxIterations  = 10
	Argon2MinMemory      = 16   // 16 MB
	Argon2MaxMemory      = 1024 // 1 GB
	Argon2MinParallelism = 1
	Argon2MaxParallelism = 16
)

// NewDefaultKdfConfig returns default PBKDF2 configuration
func NewDefaultKdfConfig() *KdfConfig {
	return &KdfConfig{
		Type:       KdfTypePBKDF2,
		Iterations: PBKDF2DefaultIterations,
	}
}

// NewArgon2KdfConfig returns default Argon2id configuration
func NewArgon2KdfConfig() *KdfConfig {
	mem := Argon2DefaultMemory
	par := Argon2DefaultParallelism
	return &KdfConfig{
		Type:        KdfTypeArgon2id,
		Iterations:  Argon2DefaultIterations,
		Memory:      &mem,
		Parallelism: &par,
	}
}

// Validate validates the KDF configuration
func (c *KdfConfig) Validate() error {
	switch c.Type {
	case KdfTypePBKDF2:
		return c.validatePBKDF2()
	case KdfTypeArgon2id:
		return c.validateArgon2()
	default:
		return errors.New("unsupported KDF type")
	}
}

// validatePBKDF2 validates PBKDF2 configuration
func (c *KdfConfig) validatePBKDF2() error {
	if c.Iterations < PBKDF2MinIterations {
		return errors.New("PBKDF2 iterations too low (minimum 600,000)")
	}
	if c.Iterations > PBKDF2MaxIterations {
		return errors.New("PBKDF2 iterations too high (maximum 2,000,000)")
	}
	return nil
}

// validateArgon2 validates Argon2id configuration
func (c *KdfConfig) validateArgon2() error {
	if c.Memory == nil {
		return errors.New("Argon2 memory is required")
	}
	if c.Parallelism == nil {
		return errors.New("Argon2 parallelism is required")
	}

	// Validate iterations
	if c.Iterations < Argon2MinIterations || c.Iterations > Argon2MaxIterations {
		return errors.New("Argon2 iterations must be between 2 and 10")
	}

	// Validate memory
	if *c.Memory < Argon2MinMemory || *c.Memory > Argon2MaxMemory {
		return errors.New("Argon2 memory must be between 16 MB and 1024 MB")
	}

	// Validate parallelism
	if *c.Parallelism < Argon2MinParallelism || *c.Parallelism > Argon2MaxParallelism {
		return errors.New("Argon2 parallelism must be between 1 and 16")
	}

	return nil
}

// ValidateForPrelogin validates KDF config against downgrade attacks
// Server should never allow iterations below minimum
func (c *KdfConfig) ValidateForPrelogin() error {
	if c.Type == KdfTypePBKDF2 {
		if c.Iterations < PBKDF2MinIterations {
			return errors.New("possible KDF downgrade attack detected")
		}
	}

	if c.Type == KdfTypeArgon2id {
		if c.Iterations < Argon2MinIterations {
			return errors.New("possible KDF downgrade attack detected")
		}
		if c.Memory != nil && *c.Memory < Argon2MinMemory {
			return errors.New("possible KDF downgrade attack detected")
		}
		if c.Parallelism != nil && *c.Parallelism < Argon2MinParallelism {
			return errors.New("possible KDF downgrade attack detected")
		}
	}

	return nil
}

// GetKdfConfig extracts KDF config from user
func (u *User) GetKdfConfig() *KdfConfig {
	return &KdfConfig{
		Type:        u.KdfType,
		Iterations:  u.KdfIterations,
		Memory:      u.KdfMemory,
		Parallelism: u.KdfParallelism,
		Salt:        u.KdfSalt,
	}
}
