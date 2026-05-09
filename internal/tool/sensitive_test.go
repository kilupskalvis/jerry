package tool

import "testing"

func TestIsSensitivePath(t *testing.T) {
	tests := []struct {
		path      string
		sensitive bool
	}{
		{".env", true},
		{".env.local", true},
		{".env.production", true},
		{"config/.env", true},
		{"deploy/.env.staging", true},
		{"main.go", false},
		{"internal/config/loader.go", false},
		{".envrc", false},
		{"environment.go", false},
		{"docs/env-setup.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsSensitivePath(tt.path)
			if got != tt.sensitive {
				t.Errorf("IsSensitivePath(%q) = %v, want %v", tt.path, got, tt.sensitive)
			}
		})
	}
}
