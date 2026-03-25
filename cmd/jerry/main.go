// Command jerry is the Jerry CLI — a runtime for composable AI code generation pipelines.
package main

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kilupskalvis/jerry/internal/agent"
	"github.com/kilupskalvis/jerry/internal/cli"
	"github.com/kilupskalvis/jerry/internal/config"
	"github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/pipeline"
	"github.com/kilupskalvis/jerry/internal/script"
	"github.com/kilupskalvis/jerry/internal/state"
	"github.com/kilupskalvis/jerry/internal/tools"
)

func main() {
	os.Exit(run())
}

func run() int {
	signalCtx, signalCancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer signalCancel()

	printer := output.NewPrinter(os.Stdout, os.Stderr)
	app := buildApp(printer)
	rootCmd := cli.NewRootCmd(app)

	if execErr := rootCmd.ExecuteContext(signalCtx); execErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "jerry: error: %s\n", execErr.Error())

		var jerryErr *errors.Error
		if stderrors.As(execErr, &jerryErr) {
			switch jerryErr.Code {
			case errors.CodeJerryDirNotFound:
				return 2
			case errors.CodeRunNotFound, errors.CodeRunNotResumable, errors.CodePipelineChanged:
				return 4
			}
		}
		return 1
	}
	return 0
}

// buildApp constructs the CLI app with all dependencies.
func buildApp(printer *output.Printer) *cli.App {
	app := &cli.App{
		Printer: printer,
	}

	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return app
	}

	jerryDir, repoRoot, findErr := config.FindJerryDir(cwd)
	if findErr != nil {
		return app
	}

	// Load .jerry/config.yaml.
	fileConfig, cfgErr := config.LoadFileConfig(jerryDir, "config.yaml")
	if cfgErr != nil {
		printer.Warning("failed to load config.yaml: %s", cfgErr)
		fileConfig = &config.FileConfig{}
	}

	// Load .env file from repo root.
	dotEnv, dotEnvErr := config.LoadDotEnv(repoRoot, ".env")
	if dotEnvErr != nil {
		printer.Warning("failed to load .env: %s", dotEnvErr)
		dotEnv = map[string]string{}
	}

	// Merge environments: process env takes precedence over .env values.
	secretEnv := collectSecretEnv()
	for key, val := range dotEnv {
		if _, exists := secretEnv[key]; !exists {
			secretEnv[key] = val
		}
	}

	// Resolve default model: env var → config.yaml.
	defaultModel := os.Getenv("JERRY_DEFAULT_MODEL")
	if defaultModel == "" {
		defaultModel = fileConfig.Defaults.Model
	}

	// Resolve default timeout.
	defaultTimeout := config.DefaultStepTimeoutValue
	if fileConfig.Defaults.Timeout.Duration > 0 {
		defaultTimeout = fileConfig.Defaults.Timeout.Duration
	}

	cfg := config.Config{
		JerryDir:           jerryDir,
		RepoRoot:           repoRoot,
		Env:                secretEnv,
		DefaultStepTimeout: defaultTimeout,
		DefaultModel:       defaultModel,
		FileConfig:         fileConfig,
	}

	loader := pipeline.NewLoader(jerryDir)
	runsDir := filepath.Join(jerryDir, "runs")
	stateStore := state.NewFileStateStore(runsDir)
	scriptExec := script.NewExecutor(cfg.RepoRoot, cfg.Env)

	// Build agent infrastructure.
	toolRegistry := tools.NewRegistry(cfg.RepoRoot, cfg.Env)
	agentLoader := agent.NewLoader(toolRegistry.KnownToolNames(), cfg.DefaultModel, fileConfig)

	// Resolve API keys from environment and .env.
	anthropicKey := resolveKey("ANTHROPIC_API_KEY", secretEnv)
	openaiKey := resolveKey("OPENAI_API_KEY", secretEnv)

	agentExec := agent.NewExecutor(agentLoader, toolRegistry, anthropicKey, openaiKey, printer)

	engine := pipeline.NewEngine(
		[]pipeline.StepExecutor{agentExec, scriptExec},
		stateStore,
		printer,
		cfg.DefaultStepTimeout,
	)

	app.Engine = engine
	app.Loader = loader
	app.AgentLoader = agentLoader
	app.StateStore = stateStore

	return app
}

// resolveKey returns the value of an env var, checking the process environment
// first, then the provided dotenv map.
func resolveKey(envVar string, dotEnv map[string]string) string {
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	return dotEnv[envVar]
}

// collectSecretEnv collects environment variables with the JERRY_SECRET_ prefix.
func collectSecretEnv() map[string]string {
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		key, val, found := strings.Cut(entry, "=")
		if found && strings.HasPrefix(key, "JERRY_SECRET_") {
			env[key] = val
		}
	}
	return env
}
