package permissions_test

import (
	"testing"

	"github.com/kilupskalvis/jerry/internal/permissions"
)

func TestMatchGlob_ExactMatch(t *testing.T) {
	t.Parallel()
	if !permissions.MatchGlob("rm -rf /", "rm -rf /") {
		t.Error("exact match should succeed")
	}
}

func TestMatchGlob_Star(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern, input string
		want           bool
	}{
		{"rm -rf *", "rm -rf /", true},
		{"rm -rf *", "rm -rf .", true},
		{"go test *", "go test ./...", true},
		{"go test *", "go build ./...", false},
		{"npm *", "npm test", true},
		{"npm *", "npm run build", true},
		{"*.env", ".env", true},
		{"*.env", "prod.env", true},
		{"*.env", ".environment", false},
		{"*.pem", "server.pem", true},
		{"*.pem", "server.pemx", false},
	}
	for _, tt := range tests {
		if got := permissions.MatchGlob(tt.pattern, tt.input); got != tt.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
		}
	}
}

func TestMatchGlob_DoubleStar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern, input string
		want           bool
	}{
		{"src/**", "src/main.go", true},
		{"src/**", "src/pkg/foo.go", true},
		{"src/**", "tests/main.go", false},
		{"**", "anything/at/all.go", true},
		{".jerry/**", ".jerry/settings.yaml", true},
		{".jerry/**", ".jerry/review/workflow.yaml", true},
	}
	for _, tt := range tests {
		if got := permissions.MatchGlob(tt.pattern, tt.input); got != tt.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
		}
	}
}

func TestMatchGlob_EmptyPattern(t *testing.T) {
	t.Parallel()
	if permissions.MatchGlob("", "anything") {
		t.Error("empty pattern should not match")
	}
}

func TestMatchGlob_EmptyInput(t *testing.T) {
	t.Parallel()
	if permissions.MatchGlob("*", "") {
		t.Error("star should not match empty input")
	}
}
