package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── validateRedirectURL ────────────────────────────────────────────────────────

func redirectTestService(allowLocalhost bool) *ssoService {
	return &ssoService{
		redirectOrigins: []string{"https://vault.passwall.io", "https://vault.acme-selfhost.com/"},
		allowLocalhost:  allowLocalhost,
	}
}

func TestValidateRedirectURL_Allowed(t *testing.T) {
	t.Parallel()
	svc := redirectTestService(false)
	for _, redirect := range []string{
		"https://vault.passwall.io/sso-complete",
		"https://VAULT.PASSWALL.IO/done",
		"https://vault.acme-selfhost.com/x",
	} {
		assert.Equal(t, redirect, svc.validateRedirectURL(redirect), redirect)
	}
}

func TestValidateRedirectURL_Rejected(t *testing.T) {
	t.Parallel()
	svc := redirectTestService(false)
	for _, redirect := range []string{
		"",
		"   ",
		"https://evil.com/steal",
		"https://passwall.io/vault",            // not a configured client origin
		"https://other.passwall.io/x",          // sibling host
		"https://vault.passwall.io.evil.com/x", // suffix attack
		"https://vault.passwall.io:8443/x",     // other port
		"http://vault.passwall.io/x",           // plain http
		"https://user@vault.passwall.io/x",     // userinfo
		"http://localhost:3000/callback",       // localhost outside dev
		"javascript:alert(1)",
		"data:text/html,<h1>x</h1>",
		"/sso/callback",
		"//evil.com/path",
		"ht tp://broken url",
	} {
		assert.Equal(t, "", svc.validateRedirectURL(redirect), "should reject %q", redirect)
	}
}

func TestValidateRedirectURL_LocalhostInDev(t *testing.T) {
	t.Parallel()
	svc := redirectTestService(true)
	for _, redirect := range []string{"http://localhost:3000/callback", "https://localhost/done", "http://127.0.0.1:8080/sso"} {
		assert.Equal(t, redirect, svc.validateRedirectURL(redirect))
	}
}

// ─── matchesDomain ──────────────────────────────────────────────────────────────

func TestMatchesDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		email  string
		domain string
		want   bool
	}{
		{"user@acme.com", "acme.com", true},
		{"user@ACME.COM", "acme.com", false}, // matchesDomain is case-sensitive; caller normalizes
		{"user@acme.com", "ACME.COM", true},  // domain is lowercased inside matchesDomain
		{"user@sub.acme.com", "acme.com", false},
		{"user@notacme.com", "acme.com", false},
		{"noemail", "acme.com", false},
		{"", "acme.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.email+"@"+tt.domain, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, matchesDomain(tt.email, tt.domain))
		})
	}
}

// ─── parseIdPCertificate ────────────────────────────────────────────────────────

// generateSelfSignedCert returns a freshly generated self-signed RSA certificate
// usable both as an IdP signing certificate (PEM form) and for signing SAML
// fixtures in tests (private key + parsed cert).
func generateSelfSignedCert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, string) {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test IdP"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return cert, privKey, string(pemBlock)
}

func TestParseIdPCertificate_ValidPEM(t *testing.T) {
	t.Parallel()
	_, _, pemStr := generateSelfSignedCert(t)

	cert, err := parseIdPCertificate(pemStr)
	require.NoError(t, err)
	assert.Equal(t, "Test IdP", cert.Subject.CommonName)
}

func TestParseIdPCertificate_RawBase64(t *testing.T) {
	t.Parallel()
	_, _, pemStr := generateSelfSignedCert(t)

	// Strip PEM headers to get raw base64
	block, _ := pem.Decode([]byte(pemStr))
	require.NotNil(t, block)
	rawB64 := base64.StdEncoding.EncodeToString(block.Bytes)

	cert, err := parseIdPCertificate(rawB64)
	require.NoError(t, err)
	assert.Equal(t, "Test IdP", cert.Subject.CommonName)
}

func TestParseIdPCertificate_Base64WithNewlines(t *testing.T) {
	t.Parallel()
	_, _, pemStr := generateSelfSignedCert(t)

	block, _ := pem.Decode([]byte(pemStr))
	require.NotNil(t, block)
	rawB64 := base64.StdEncoding.EncodeToString(block.Bytes)

	// Insert newlines (common in IdP config exports)
	withNewlines := ""
	for i, c := range rawB64 {
		if i > 0 && i%64 == 0 {
			withNewlines += "\n"
		}
		withNewlines += string(c)
	}

	cert, err := parseIdPCertificate(withNewlines)
	require.NoError(t, err)
	assert.Equal(t, "Test IdP", cert.Subject.CommonName)
}

func TestParseIdPCertificate_Empty(t *testing.T) {
	t.Parallel()
	_, err := parseIdPCertificate("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty certificate")
}

func TestParseIdPCertificate_Whitespace(t *testing.T) {
	t.Parallel()
	_, err := parseIdPCertificate("   \n\t  ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty certificate")
}

func TestParseIdPCertificate_InvalidData(t *testing.T) {
	t.Parallel()
	_, err := parseIdPCertificate("this is definitely not a certificate")
	assert.Error(t, err)
}

func TestParseIdPCertificate_WrongKeyType(t *testing.T) {
	t.Parallel()
	// PEM block that is a private key, not a certificate
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	_, err = parseIdPCertificate(string(pemBlock))
	assert.Error(t, err, "should reject non-certificate PEM blocks")
}

// ─── defaultScopes ──────────────────────────────────────────────────────────────

func TestDefaultScopes(t *testing.T) {
	t.Parallel()

	t.Run("nil returns defaults", func(t *testing.T) {
		assert.Equal(t, []string{"openid", "email", "profile"}, defaultScopes(nil))
	})

	t.Run("empty returns defaults", func(t *testing.T) {
		assert.Equal(t, []string{"openid", "email", "profile"}, defaultScopes([]string{}))
	})

	t.Run("custom preserved", func(t *testing.T) {
		custom := []string{"openid", "email", "groups"}
		assert.Equal(t, custom, defaultScopes(custom))
	})
}

// ─── generateRandomState ────────────────────────────────────────────────────────

func TestGenerateRandomState_Uniqueness(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, err := generateRandomState()
		require.NoError(t, err)
		assert.NotEmpty(t, s)
		assert.False(t, seen[s], "duplicate state generated")
		seen[s] = true
	}
}

func TestGenerateRandomState_Length(t *testing.T) {
	t.Parallel()
	s, err := generateRandomState()
	require.NoError(t, err)
	// 32 bytes base64-raw-url encoded → ~43 chars
	assert.GreaterOrEqual(t, len(s), 40)
}

// ─── generatePKCE ───────────────────────────────────────────────────────────────

func TestGeneratePKCE(t *testing.T) {
	t.Parallel()
	v, c, err := generatePKCE()
	require.NoError(t, err)
	assert.NotEmpty(t, v)
	assert.NotEmpty(t, c)
	assert.NotEqual(t, v, c, "verifier and challenge should differ")
}

func TestGeneratePKCE_Uniqueness(t *testing.T) {
	t.Parallel()
	v1, c1, err := generatePKCE()
	require.NoError(t, err)
	v2, c2, err := generatePKCE()
	require.NoError(t, err)
	assert.NotEqual(t, v1, v2)
	assert.NotEqual(t, c1, c2)
}

// ─── callbackURL ────────────────────────────────────────────────────────────────

func TestCallbackURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		baseURL string
		want    string
	}{
		{"https://api.passwall.io", "https://api.passwall.io/sso/callback"},
		{"https://api.passwall.io/", "https://api.passwall.io/sso/callback"},
		{"http://localhost:8080", "http://localhost:8080/sso/callback"},
	}
	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			t.Parallel()
			svc := &ssoService{baseURL: tt.baseURL}
			assert.Equal(t, tt.want, svc.callbackURL())
		})
	}
}
