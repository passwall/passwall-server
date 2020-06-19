package app

import "testing"

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
