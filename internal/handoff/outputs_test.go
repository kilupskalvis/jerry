package handoff

import (
	"strings"
	"testing"
)

func TestStructuredOutputDirective(t *testing.T) {
	d := StructuredOutputDirective(map[string]string{"verdict": "string", "findings": "string"})
	if d == "" {
		t.Fatal("directive empty")
	}
	for _, want := range []string{"verdict", "findings", "JSON"} {
		if !strings.Contains(d, want) {
			t.Errorf("directive missing %q:\n%s", want, d)
		}
	}
}

func TestStructuredOutputDirectiveEmpty(t *testing.T) {
	if StructuredOutputDirective(nil) != "" {
		t.Error("no declared outputs → no directive")
	}
}

func TestParseStructuredText(t *testing.T) {
	cases := []struct{ name, in string }{
		{"bare", `{"verdict":"success","findings":"none"}`},
		{"fenced", "```json\n{\"verdict\":\"success\",\"findings\":\"none\"}\n```"},
		{"preamble", "Here is the result:\n{\"verdict\":\"success\",\"findings\":\"none\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseStructuredText(tc.in)
			if err != nil {
				t.Fatalf("ParseStructuredText: %v", err)
			}
			if got["verdict"] != "success" || got["findings"] != "none" {
				t.Errorf("parsed = %v", got)
			}
		})
	}
}

func TestParseStructuredTextFailure(t *testing.T) {
	if _, err := ParseStructuredText("no json here at all"); err == nil {
		t.Fatal("want error when no JSON object present")
	}
}
