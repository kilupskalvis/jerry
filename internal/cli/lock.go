// jerry lock: pins installed runtime versions into jerry.lock.

package cli

import (
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/spec"
)

// @lattice:flow lock
func newLockCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "lock",
		Short: "Pin installed runtime versions into jerry.lock",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return RunLock(app)
		},
	}
}

// RunLock detects the installed pi version and writes jerry.lock.
func RunLock(app *App) error {
	if app.JerryDir == "" {
		return jerrerr.New(jerrerr.CodeJerryDirNotFound,
			"not in a Jerry project (no .jerry/ directory found)")
	}
	out, err := exec.Command("pi", "--version").Output()
	if err != nil {
		return jerrerr.Wrap(jerrerr.CodeConfigInvalid,
			"cannot detect pi version (is pi installed? `npm i -g @mariozechner/pi-coding-agent`)", err)
	}
	version := strings.TrimSpace(string(out))

	lock := &spec.Lockfile{
		Version: 1,
		Runtimes: map[string]spec.LockedRuntime{
			"pi": {Package: "@mariozechner/pi-coding-agent", Version: version},
		},
	}
	if err := lock.Save(app.JerryDir); err != nil {
		return err
	}
	app.Printer.Info("pinned pi %s in jerry.lock", version)
	return nil
}
