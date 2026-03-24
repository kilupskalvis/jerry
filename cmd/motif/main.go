// Command motif is the Motif CLI — a runtime for composable AI code generation pipelines.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kilupskalvis/motif/internal/agent"
	"github.com/kilupskalvis/motif/internal/cli"
	"github.com/kilupskalvis/motif/internal/config"
	"github.com/kilupskalvis/motif/internal/llm"
	"github.com/kilupskalvis/motif/internal/output"
	"github.com/kilupskalvis/motif/internal/pipeline"
	"github.com/kilupskalvis/motif/internal/script"
	"github.com/kilupskalvis/motif/internal/state"
	"github.com/kilupskalvis/motif/internal/tools"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Set up signal handling — Ctrl+C propagates cancellation through the pipeline.
	signalCtx, signalCancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer signalCancel()

	printer := output.NewPrinter(os.Stdout, os.Stderr)

	// Build the app with lazy initialization.
	// Some commands (like init) don't need .motif/ to exist.
	app := buildApp(printer)

	rootCmd := cli.NewRootCmd(app)

	if execErr := rootCmd.ExecuteContext(signalCtx); execErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "motif: error: %s\n", execErr.Error())
		return 1
	}
	return 0
}

// buildApp constructs the CLI app with all dependencies.
// If .motif/ is not found, Loader and Engine will be nil —
// commands that need them (run, validate) check for this.
func buildApp(printer *output.Printer) *cli.App {
	app := &cli.App{
		Printer: printer,
	}

	// Try to find .motif/ directory
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return app
	}

	motifDir, repoRoot, findErr := config.FindMotifDir(cwd)
	if findErr != nil {
		// Not in a Motif project — init command will still work
		return app
	}

	// Build config
	cfg := config.Config{
		MotifDir:           motifDir,
		RepoRoot:           repoRoot,
		Env:                collectSecretEnv(),
		DefaultStepTimeout: config.DefaultStepTimeoutValue,
		DefaultModel:       os.Getenv("MOTIF_DEFAULT_MODEL"),
	}

	// Build dependencies
	loader := pipeline.NewLoader(motifDir)

	runsDir := filepath.Join(motifDir, "runs")
	stateStore := state.NewFileStateStore(runsDir)

	scriptExec := script.NewExecutor(cfg.RepoRoot, cfg.Env)

	// Build agent executor with real LLM client if API key is available.
	toolRegistry := tools.NewRegistry(cfg.RepoRoot, cfg.Env)
	agentLoader := agent.NewLoader(toolRegistry.KnownToolNames(), cfg.DefaultModel, nil)

	var llmClient llm.Client
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		llmClient = llm.NewAnthropicClient(anthropicKey, cfg.DefaultModel)
	}

	agentExec := agent.NewExecutor(agentLoader, toolRegistry, llmClient, printer)

	engine := pipeline.NewEngine(
		[]pipeline.StepExecutor{agentExec, scriptExec},
		stateStore,
		printer,
		cfg.DefaultStepTimeout,
	)

	app.Engine = engine
	app.Loader = loader
	app.StateStore = stateStore

	return app
}

// collectSecretEnv collects environment variables with the MOTIF_SECRET_ prefix.
func collectSecretEnv() map[string]string {
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "MOTIF_SECRET_") {
			env[parts[0]] = parts[1]
		}
	}
	return env
}
