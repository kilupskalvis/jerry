package spec

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDurationUnmarshal(t *testing.T) {
	cases := []struct {
		name    string
		yamlSrc string
		want    time.Duration
		wantErr bool
	}{
		{"seconds", `timeout: 600s`, 600 * time.Second, false},
		{"minutes", `timeout: 10m`, 10 * time.Minute, false},
		{"empty", `timeout: ""`, 0, false},
		{"bare number", `timeout: 600`, 0, true},
		{"garbage", `timeout: soon`, 0, true},
		{"negative", `timeout: -5s`, -5 * time.Second, false}, // parses; validator rejects
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var s struct {
				Timeout Duration `yaml:"timeout"`
			}
			err := yaml.Unmarshal([]byte(tc.yamlSrc), &s)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (parsed %v)", s.Timeout.Duration)
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if s.Timeout.Duration != tc.want {
				t.Errorf("got %v, want %v", s.Timeout.Duration, tc.want)
			}
		})
	}
}
