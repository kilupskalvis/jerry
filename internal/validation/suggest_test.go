package validation_test

import (
	"testing"

	"github.com/kilupskalvis/jerry/internal/validation"
)

func TestLevenshtein(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"tools", "tols", 1},
		{"temperature", "temprature", 1},
		{"retries", "retry", 3},
		{"abc", "xyz", 3},
	}
	for _, tt := range tests {
		if got := validation.Levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("Levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSuggest(t *testing.T) {
	t.Parallel()
	valid := []string{"name", "model", "temperature", "max_iterations", "tools", "provider"}

	tests := []struct {
		input string
		want  string
	}{
		{"tols", "tools"},
		{"temprature", "temperature"},
		{"modle", "model"}, //nolint:misspell // intentional typo for testing
		{"nam", "name"},
		{"xyzzy", ""},
		{"providers", "provider"},
	}
	for _, tt := range tests {
		got := validation.Suggest(tt.input, valid)
		if got != tt.want {
			t.Errorf("Suggest(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
