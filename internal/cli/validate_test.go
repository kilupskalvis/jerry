package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/cli"
)

func TestValidateCmd_VersionFlag(t *testing.T) {
	app := &cli.App{Version: "1.2.3"}
	rootCmd := cli.NewRootCmd(app)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"validate", "--version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "1.2.3") {
		t.Errorf("expected output to contain version %q, got: %q", "1.2.3", output)
	}
}

func TestValidateCmd_VersionFlag_Empty(t *testing.T) {
	// When App.Version is empty, cobra does not register a --version flag on the
	// validate subcommand. Invoking --version should therefore return an error.
	app := &cli.App{}
	rootCmd := cli.NewRootCmd(app)
	rootCmd.SetArgs([]string{"validate", "--version"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected an error when --version flag is not registered (empty App.Version), got nil")
	}
}
