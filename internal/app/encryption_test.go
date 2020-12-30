package app

import (
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/passwall/passwall-server/model"
	"github.com/stretchr/testify/assert"
)

func TestFallbackInsecureKey(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{name: "FBIK Length 1", length: 0, wantErr: true},
		{name: "FBIK Length 8", length: 8, wantErr: false},
		{name: "FBIK Length 30", length: 30, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insecure, err := FallbackInsecureKey(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("FallbackInsecureKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(insecure) != tt.length {
				t.Errorf("FallbackInsecureKey() got length = %v, want length %v", len(insecure), tt.length)
			}
		})
	}
}

func TestGenerateSecureKey(t *testing.T) {

	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{name: "Short length secure key", length: 0, wantErr: true},
		{name: "Meets with min requirements", length: 8, wantErr: false},
		{name: "Static length value", length: 30, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSecureKey(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSecureKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) < tt.length {
				t.Errorf("GenerateSecureKey() got = %v, should be bigger than %d", got, tt.length)
			}

		})
	}
}

func TestEncryptModel(t *testing.T) {

	login := &model.Login{
		ID:        uint(1),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: nil,
		Title:     "Baslik",
		URL:       "yakuter.com",
		Username:  "yakuter",
		Password:  "123456",
	}

	encLogin, err := EncryptModel(login)

	if err != nil {
		t.Error(err)
	}

	decLogin, err := DecryptModel(encLogin)

	if err != nil {
		t.Error(err)
	}

	assert.Nil(t, deep.Equal(login, decLogin))
}

func TestDecryptJSON(t *testing.T) {

	// Define tests
	tests := []struct {
		name     string
		key      string
		input    []byte
		v        interface{}
		expected interface{}
	}{
		{name: "Basic JSON Decrypt 1", key: "secret key for test 1", v: make(map[string]string), expected: map[string]string{"name": "Passwall User", "password": "password"}},
		{name: "Basic JSON Decrypt 2", key: "secret key for test 2", v: make(map[string]string), expected: map[string]string{"id": "213921407", "itemName": "Keyboard", "color": "#f345ff"}},
		{name: "UserDTO JSON Decrypt 1", key: "secret key for user 1", v: model.UserDTO{}, expected: model.UserDTO{Email: "yusufturhanp@gmail.com", MasterPassword: "password", Name: "Yusuf"}},
		{name: "UserDTO JSON Decrypt 2", key: "secret key for user 2", v: model.UserDTO{}, expected: model.UserDTO{Email: "passwall@passwall.io", MasterPassword: "passwall", Name: "Passwall"}},
	}

	// Set test inputs
	for i := range tests {
		res, err := EncryptJSON(tests[i].key, tests[i].expected)
		if err != nil {
			t.Errorf("Error in creating inputs: %v", err)
		}
		tests[i].input = res
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := DecryptJSON(test.key, test.input, &test.v); err != nil {
				t.Errorf("Decrypt Error: %v", err)
			}
		})
	}
}

func TestEncryptJSON(t *testing.T) {

	// Define tests
	tests := []struct {
		name string
		key  string
		v    interface{}
	}{
		{name: "Basic JSON Encrypt 1", key: "secret key for test 1", v: map[string]string{"name": "Passwall User", "password": "password"}},
		{name: "Basic JSON Encrypt 2", key: "secret key for test 2", v: map[string]string{"id": "213921407", "itemName": "Keyboard", "color": "#f345ff"}},
		{name: "UserDTO JSON Encrypt 1", key: "secret key for test 1", v: model.UserDTO{Email: "yusufturhanp@gmail.com", MasterPassword: "password", Name: "Yusuf"}},
		{name: "UserDTO JSON Encrypt 2", key: "secret key for test 2", v: model.UserDTO{Email: "passwall@passwall.io", MasterPassword: "passwall", Name: "Passwall"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := EncryptJSON(test.key, test.v)
			if err != nil {
				t.Errorf("Encrypt Error: %v", err)
			}
		})
	}
}
