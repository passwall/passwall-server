package app

import (
	"testing"

	"github.com/passwall/passwall-server/model"
)

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
