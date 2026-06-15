// Command jerry compiles portable agent-pipeline specs into native CI
// config and runs them through pluggable agent runtimes.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kilupskalvis/jerry/internal/cli"
	"github.com/kilupskalvis/jerry/internal/config"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/runtime"
)

var Version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	printer := output.NewPrinter(os.Stdout, os.Stderr)
	app := buildApp(printer)
	rootCmd := cli.NewRootCmd(app)
	rootCmd.Version = Version

	if execErr := rootCmd.ExecuteContext(ctx); execErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "jerry: error: %s\n", execErr.Error())

		var jerryErr *jerrerr.Error
		if errors.As(execErr, &jerryErr) {
			return jerryErr.ExitCode()
		}
		return 1
	}
	return 0
}

// buildApp wires the CLI dependencies. The runtime registry is empty until
// the pi adapter lands; agent steps error with "unknown runtime" until then.
func buildApp(printer *output.Printer) *cli.App {
	app := &cli.App{Printer: printer, Registry: runtime.NewRegistry()}

	cwd, err := os.Getwd()
	if err != nil {
		return app
	}
	jerryDir, repoRoot, findErr := config.FindJerryDir(cwd)
	if findErr != nil {
		return app
	}
	app.JerryDir = jerryDir
	app.RepoRoot = repoRoot

	loadDotEnvIntoProcess(repoRoot, printer)
	return app
}

// loadDotEnvIntoProcess loads .env values into the process environment so
// agent runtimes and shell steps — which read from a strict allowlist of
// process env vars — can see locally-declared secrets. Real environment
// values take precedence.
func loadDotEnvIntoProcess(repoRoot string, printer *output.Printer) {
	dotEnv, err := config.LoadDotEnv(repoRoot, ".env")
	if err != nil {
		printer.Warning("failed to load .env: %s", err)
		return
	}
	for k, v := range dotEnv {
		if _, ok := os.LookupEnv(k); !ok {
			_ = os.Setenv(k, v)
		}
	}
}
