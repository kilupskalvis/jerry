package handoff

import "testing"

func TestPathLookup(t *testing.T) {
	doc := map[string]any{
		"pull_request": map[string]any{"number": float64(42)},
		"commits": []any{
			map[string]any{"message": "first"},
			map[string]any{"message": "second"},
		},
	}
	cases := []struct {
		path    string
		want    string
		wantErr bool
	}{
		{"pull_request.number", "42", false},
		{"commits[0].message", "first", false},
		{"commits[1].message", "second", false},
		{"missing.key", "", true},
		{"commits[9].message", "", true},
		{"commits[x].message", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got, err := PathLookup(doc, tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("PathLookup: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
