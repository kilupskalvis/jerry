package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLock(t *testing.T) {
	dir := t.TempDir()
	lock := `
version: 1
runtimes:
  pi:
    package: "@mariozechner/pi-coding-agent"
    version: "0.42.1"
    checksum: "sha512-abc"
`
	if err := os.WriteFile(filepath.Join(dir, "jerry.lock"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}
	rt, ok := l.Runtimes["pi"]
	if !ok || rt.Version != "0.42.1" {
		t.Errorf("lock parsed wrong: %+v", l)
	}
}

func TestLockfileSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	l := &Lockfile{
		Version: 1,
		Runtimes: map[string]LockedRuntime{
			"pi": {Package: "@mariozechner/pi-coding-agent", Version: "0.73.1"},
		},
	}
	if err := l.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}
	if got.Runtimes["pi"].Version != "0.73.1" || got.Version != 1 {
		t.Errorf("round trip wrong: %+v", got)
	}
}

func TestLoadLockAbsent(t *testing.T) {
	l, err := LoadLock(t.TempDir())
	if err != nil {
		t.Fatalf("absent lock should not error: %v", err)
	}
	if l != nil {
		t.Errorf("want nil lock when absent, got %+v", l)
	}
}
