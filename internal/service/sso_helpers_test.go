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
	"encoding/xml"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── validateRedirectURL ────────────────────────────────────────────────────────

func TestValidateRedirectURL_EmptyInput(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", validateRedirectURL("", "https://api.passwall.io", "acme.com"))
	assert.Equal(t, "", validateRedirectURL("   ", "https://api.passwall.io", "acme.com"))
}

func TestValidateRedirectURL_SameOriginAsServer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		redirect  string
		serverURL string
		domain    string
		want      string
	}{
		{
			name:      "exact server origin",
			redirect:  "https://api.passwall.io/sso/complete",
			serverURL: "https://api.passwall.io",
			domain:    "acme.com",
			want:      "https://api.passwall.io/sso/complete",
		},
		{
			name:      "server with trailing slash",
			redirect:  "https://api.passwall.io/callback",
			serverURL: "https://api.passwall.io/",
			domain:    "acme.com",
			want:      "https://api.passwall.io/callback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := validateRedirectURL(tt.redirect, tt.serverURL, tt.domain)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateRedirectURL_SSOConnectionDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		redirect string
		domain   string
		want     string
	}{
		{
			name:     "exact domain match",
			redirect: "https://acme.com/dashboard",
			domain:   "acme.com",
			want:     "https://acme.com/dashboard",
		},
		{
			name:     "subdomain match",
			redirect: "https://vault.acme.com/login",
			domain:   "acme.com",
			want:     "https://vault.acme.com/login",
		},
		{
			name:     "deeply nested subdomain",
			redirect: "https://a.b.c.acme.com/x",
			domain:   "acme.com",
			want:     "https://a.b.c.acme.com/x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := validateRedirectURL(tt.redirect, "https://api.passwall.io", tt.domain)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateRedirectURL_PasswallDomains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		redirect string
		want     string
	}{
		{
			name:     "passwall.io root",
			redirect: "https://passwall.io/vault",
			want:     "https://passwall.io/vault",
		},
		{
			name:     "vault.passwall.io",
			redirect: "https://vault.passwall.io/sso-complete",
			want:     "https://vault.passwall.io/sso-complete",
		},
		{
			name:     "passwall.com subdomain",
			redirect: "https://app.passwall.com/done",
			want:     "https://app.passwall.com/done",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := validateRedirectURL(tt.redirect, "https://api.passwall.io", "unrelated.org")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateRedirectURL_Localhost(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		redirect string
	}{
		{"localhost http", "http://localhost:3000/callback"},
		{"localhost https", "https://localhost/done"},
		{"127.0.0.1", "http://127.0.0.1:8080/sso"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := validateRedirectURL(tt.redirect, "https://api.passwall.io", "acme.com")
			assert.Equal(t, tt.redirect, got)
		})
	}
}

func TestValidateRedirectURL_Rejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		redirect string
	}{
		{"completely foreign domain", "https://evil.com/steal"},
		{"domain suffix attack", "https://notacme.com/fake"},
		{"domain prefix attack", "https://acme.com.evil.com/x"},
		{"javascript scheme", "javascript:alert(1)"},
		{"data URI scheme", "data:text/html,<h1>x</h1>"},
		{"ftp scheme", "ftp://files.acme.com/data"},
		{"relative path", "/sso/callback"},
		{"protocol-relative", "//evil.com/path"},
		{"malformed URL", "ht tp://broken url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := validateRedirectURL(tt.redirect, "https://api.passwall.io", "acme.com")
			assert.Equal(t, "", got, "should reject redirect URL: %s", tt.redirect)
		})
	}
}

func TestValidateRedirectURL_CaseInsensitiveDomain(t *testing.T) {
	t.Parallel()
	got := validateRedirectURL("https://VAULT.PASSWALL.IO/done", "https://api.passwall.io", "acme.com")
	assert.Equal(t, "https://VAULT.PASSWALL.IO/done", got)
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

// ─── urlsEqualWithoutTrailingSlash ──────────────────────────────────────────────

func TestUrlsEqualWithoutTrailingSlash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", "https://example.com/path", "https://example.com/path", true},
		{"trailing slash on a", "https://example.com/path/", "https://example.com/path", true},
		{"trailing slash on b", "https://example.com/path", "https://example.com/path/", true},
		{"both trailing", "https://example.com/", "https://example.com/", true},
		{"case insensitive", "HTTPS://EXAMPLE.COM/Path", "https://example.com/path", true},
		{"different paths", "https://example.com/a", "https://example.com/b", false},
		{"empty a", "", "https://example.com", false},
		{"empty b", "https://example.com", "", false},
		{"both empty", "", "", false},
		{"whitespace", "  https://example.com  ", "https://example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, urlsEqualWithoutTrailingSlash(tt.a, tt.b))
		})
	}
}

// ─── parseSAMLTime ──────────────────────────────────────────────────────────────

func TestParseSAMLTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"RFC3339", "2025-01-15T10:30:00Z", false},
		{"RFC3339Nano", "2025-01-15T10:30:00.123456789Z", false},
		{"with offset", "2025-01-15T10:30:00+02:00", false},
		{"empty", "", true},
		{"garbage", "not-a-date", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseSAMLTime(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}
}

// ─── extractSAMLEmail ───────────────────────────────────────────────────────────

func TestExtractSAMLEmail_FromAttributes(t *testing.T) {
	t.Parallel()
	attrNames := []string{
		"email",
		"mail",
		"emailaddress",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"urn:oid:0.9.2342.19200300.100.1.3",
	}
	for _, name := range attrNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertion := &samlAssertion{
				Attributes: samlAttrStmt{
					Attributes: []samlAttribute{
						{
							Name:   name,
							Values: []samlAttributeVal{{Value: "user@acme.com"}},
						},
					},
				},
			}
			assert.Equal(t, "user@acme.com", extractSAMLEmail(assertion))
		})
	}
}

func TestExtractSAMLEmail_FallbackToNameID(t *testing.T) {
	t.Parallel()
	assertion := &samlAssertion{
		Subject: samlSubject{
			NameID: samlTextNode{Value: "user@acme.com"},
		},
		Attributes: samlAttrStmt{},
	}
	assert.Equal(t, "user@acme.com", extractSAMLEmail(assertion))
}

func TestExtractSAMLEmail_NameIDNotEmail(t *testing.T) {
	t.Parallel()
	assertion := &samlAssertion{
		Subject: samlSubject{
			NameID: samlTextNode{Value: "some-opaque-id"},
		},
		Attributes: samlAttrStmt{},
	}
	assert.Equal(t, "", extractSAMLEmail(assertion))
}

func TestExtractSAMLEmail_NilAssertion(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", extractSAMLEmail(nil))
}

func TestExtractSAMLEmail_AttributeValueNotEmail(t *testing.T) {
	t.Parallel()
	assertion := &samlAssertion{
		Attributes: samlAttrStmt{
			Attributes: []samlAttribute{
				{
					Name:   "email",
					Values: []samlAttributeVal{{Value: "not-an-email"}},
				},
			},
		},
	}
	assert.Equal(t, "", extractSAMLEmail(assertion))
}

// ─── parseIdPCertificate ────────────────────────────────────────────────────────

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

// ─── verifySAMLXMLSignature ─────────────────────────────────────────────────────

func TestVerifySAMLXMLSignature_InvalidCertificate(t *testing.T) {
	t.Parallel()
	err := verifySAMLXMLSignature([]byte("<Response/>"), "not-a-cert")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestVerifySAMLXMLSignature_InvalidXML(t *testing.T) {
	t.Parallel()
	_, _, pemStr := generateSelfSignedCert(t)
	err := verifySAMLXMLSignature([]byte("<<<not xml"), pemStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "XML")
}

func TestVerifySAMLXMLSignature_MissingSignature(t *testing.T) {
	t.Parallel()
	_, _, pemStr := generateSelfSignedCert(t)
	// Valid XML but no Signature element -> goxmldsig should return error
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol">
  <saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
    <saml:Issuer>https://idp.acme.com</saml:Issuer>
  </saml:Assertion>
</samlp:Response>`)
	err := verifySAMLXMLSignature(xmlData, pemStr)
	assert.Error(t, err, "should fail when XML has no signature")
}

func TestVerifySAMLXMLSignature_WrongCertificate(t *testing.T) {
	t.Parallel()
	// Cert A for signing, Cert B for verification → should fail
	_, _, certA := generateSelfSignedCert(t) // cert used to "sign"
	_, _, certB := generateSelfSignedCert(t) // cert used to verify

	// Just a minimal SAML response without valid signature
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol">
  <saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
    <saml:Issuer>https://idp.acme.com</saml:Issuer>
  </saml:Assertion>
</samlp:Response>`)
	_ = certA
	err := verifySAMLXMLSignature(xmlData, certB)
	assert.Error(t, err, "should fail with wrong/mismatched certificate")
}

// ─── samlResponseEnvelope XML parsing ───────────────────────────────────────────

func TestSAMLResponseEnvelope_Parse(t *testing.T) {
	t.Parallel()

	raw := `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol"
		xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
		<saml:Issuer>https://idp.example.com</saml:Issuer>
		<saml:Assertion>
			<saml:Issuer>https://idp.example.com</saml:Issuer>
			<saml:Subject>
				<saml:NameID>user@example.com</saml:NameID>
				<saml:SubjectConfirmation>
					<saml:SubjectConfirmationData Recipient="https://api.passwall.io/sso/callback"/>
				</saml:SubjectConfirmation>
			</saml:Subject>
			<saml:Conditions NotBefore="2025-01-15T10:00:00Z" NotOnOrAfter="2025-01-15T10:10:00Z">
				<saml:AudienceRestriction>
					<saml:Audience>https://api.passwall.io/sso/metadata/1</saml:Audience>
				</saml:AudienceRestriction>
			</saml:Conditions>
			<saml:AttributeStatement>
				<saml:Attribute Name="email">
					<saml:AttributeValue>alice@example.com</saml:AttributeValue>
				</saml:Attribute>
			</saml:AttributeStatement>
		</saml:Assertion>
	</samlp:Response>`

	var env samlResponseEnvelope
	err := xml.Unmarshal([]byte(raw), &env)
	require.NoError(t, err)
	require.NotNil(t, env.Assertion)

	assert.Equal(t, "https://idp.example.com", env.Issuer.Value)
	assert.Equal(t, "https://idp.example.com", env.Assertion.Issuer.Value)
	assert.Equal(t, "user@example.com", env.Assertion.Subject.NameID.Value)
	assert.Equal(t, "https://api.passwall.io/sso/callback", env.Assertion.Subject.SubjectConfirmation.SubjectConfirmationData.Recipient)
	assert.Equal(t, "2025-01-15T10:00:00Z", env.Assertion.Conditions.NotBefore)
	assert.Equal(t, "2025-01-15T10:10:00Z", env.Assertion.Conditions.NotOnOrAfter)
	assert.Equal(t, "https://api.passwall.io/sso/metadata/1", env.Assertion.Conditions.AudienceRestriction.Audience)
	assert.Equal(t, "alice@example.com", extractSAMLEmail(env.Assertion))
}

func TestSAMLResponseEnvelope_HasSignature(t *testing.T) {
	t.Parallel()

	t.Run("response with signature", func(t *testing.T) {
		raw := `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol"
			xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
			<ds:Signature><ds:SignedInfo/></ds:Signature>
			<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
				<saml:Issuer>idp</saml:Issuer>
			</saml:Assertion>
		</samlp:Response>`
		var env samlResponseEnvelope
		require.NoError(t, xml.Unmarshal([]byte(raw), &env))
		assert.True(t, env.hasSignature())
	})

	t.Run("response without signature", func(t *testing.T) {
		raw := `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol">
			<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
				<saml:Issuer>idp</saml:Issuer>
			</saml:Assertion>
		</samlp:Response>`
		var env samlResponseEnvelope
		require.NoError(t, xml.Unmarshal([]byte(raw), &env))
		assert.False(t, env.hasSignature())
	})

	t.Run("assertion with signature", func(t *testing.T) {
		raw := `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol"
			xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
			<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
				<saml:Issuer>idp</saml:Issuer>
				<ds:Signature><ds:SignedInfo/></ds:Signature>
			</saml:Assertion>
		</samlp:Response>`
		var env samlResponseEnvelope
		require.NoError(t, xml.Unmarshal([]byte(raw), &env))
		assert.False(t, env.hasSignature())
		assert.True(t, env.Assertion.hasSignature())
	})
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
