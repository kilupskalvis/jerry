// Command jerry is the Jerry CLI — a runtime for composable AI code generation workflows.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kilupskalvis/jerry/internal/agent"
	"github.com/kilupskalvis/jerry/internal/cli"
	"github.com/kilupskalvis/jerry/internal/config"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/output"
	jerryrun "github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/tool"
	"github.com/kilupskalvis/jerry/internal/workflow"
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
	app.Version = Version
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

	dotEnv, dotEnvErr := config.LoadDotEnv(repoRoot, ".env")
	if dotEnvErr != nil {
		printer.Warning("failed to load .env: %s", dotEnvErr)
		dotEnv = map[string]string{}
	}

	secretEnv := make(map[string]string)
	for _, entry := range os.Environ() {
		key, val, found := strings.Cut(entry, "=")
		if found && strings.HasPrefix(key, "JERRY_SECRET_") {
			secretEnv[key] = val
		}
	}
	for key, val := range dotEnv {
		if _, exists := secretEnv[key]; !exists {
			secretEnv[key] = val
		}
	}

	defaultModel := os.Getenv("JERRY_DEFAULT_MODEL")

	loader := workflow.NewLoader(jerryDir)
	runsDir := filepath.Join(jerryDir, "runs")
	stateStore := jerryrun.NewFileStateStore(runsDir)
	scriptExec := workflow.NewScriptExecutor(repoRoot, secretEnv)

	toolRegistry := tool.NewRegistry(repoRoot, secretEnv)
	toolsDir := filepath.Join(jerryDir, "tools")
	if loadErr := toolRegistry.LoadCustomTools(toolsDir, repoRoot, envSliceFromMap(secretEnv)); loadErr != nil {
		printer.Warning("failed to load custom tools: %s", loadErr)
	}
	agentLoader := agent.NewLoader(defaultModel)

	resolver := llm.NewProviderResolver()
	resolver.SetKey("anthropic", envOrSecret("ANTHROPIC_API_KEY", secretEnv))
	resolver.SetKey("openai", envOrSecret("OPENAI_API_KEY", secretEnv))

	agentExec := workflow.NewAgentExecutor(agentLoader, toolRegistry, printer, resolver)

	engine := workflow.NewEngine(
		[]workflow.StepExecutor{agentExec, scriptExec},
		stateStore,
		printer,
		config.DefaultStepTimeoutValue,
	)

	engine.OnStoreCreated = func(store *jerryrun.ContextStore) {
		agentExec.SetStore(store)
		scriptExec.SetStore(store)
		toolRegistry.SetTrigger(store.Trigger())
	}

	app.Engine = engine
	app.Loader = loader
	app.AgentLoader = agentLoader
	app.StateStore = stateStore

	return app
}

func envSliceFromMap(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, k+"="+v)
	}
	return s
}

func envOrSecret(key string, secrets map[string]string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return secrets[key]
}
