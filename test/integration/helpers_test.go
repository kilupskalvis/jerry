package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/agent"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/tool"
	"github.com/kilupskalvis/jerry/internal/trigger"
	"github.com/kilupskalvis/jerry/internal/workflow"
)

// scriptedProvider returns pre-configured LLM responses from a queue.
// When the queue is exhausted, it returns a fallback response that ends
// the agent loop. Thread-safe. Every call is recorded for assertion.
type scriptedProvider struct {
	mu        sync.Mutex
	responses []llm.CompleteResponse
	fallback  llm.CompleteResponse
	calls     []llm.CompleteParams
	callIndex int
}

func newScriptedProvider() *scriptedProvider {
	return &scriptedProvider{
		fallback: llm.CompleteResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Content: "done"},
			StopReason: llm.StopReasonEndTurn,
			Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
		},
	}
}

func (p *scriptedProvider) Complete(_ context.Context, params llm.CompleteParams) (*llm.CompleteResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, params)
	if p.callIndex >= len(p.responses) {
		resp := p.fallback
		return &resp, nil
	}
	resp := p.responses[p.callIndex]
	p.callIndex++
	return &resp, nil
}

func (p *scriptedProvider) getCalls() []llm.CompleteParams {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]llm.CompleteParams, len(p.calls))
	copy(result, p.calls)
	return result
}

// --- Response Helpers ---

func textResponse(content string) llm.CompleteResponse {
	return llm.CompleteResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Content: content},
		StopReason: llm.StopReasonEndTurn,
		Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
	}
}

func toolCallResponse(name, inputJSON string) llm.CompleteResponse {
	return llm.CompleteResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: fmt.Sprintf("tc_%s", name), Name: name, Input: json.RawMessage(inputJSON)},
			},
		},
		StopReason: llm.StopReasonToolUse,
		Usage:      llm.Usage{InputTokens: 100, OutputTokens: 30},
	}
}

// --- Captured HTTP Request ---

type capturedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

// --- Test Environment ---

type testEnv struct {
	t           *testing.T
	repoRoot    string
	jerryDir    string
	wfName      string
	triggerData *trigger.TriggerData
	provider    *scriptedProvider
	githubURL   string
	githubToken string
	githubReqs  []capturedRequest
	githubMu    sync.Mutex
}

type testResult struct {
	runState *run.RunState
	err      error
	llmCalls []llm.CompleteParams
	httpReqs []capturedRequest
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	repoRoot := t.TempDir()
	jerryDir := filepath.Join(repoRoot, ".jerry")
	if err := os.MkdirAll(jerryDir, 0o755); err != nil {
		t.Fatalf("create .jerry dir: %v", err)
	}
	return &testEnv{
		t:        t,
		repoRoot: repoRoot,
		jerryDir: jerryDir,
		provider: newScriptedProvider(),
	}
}

func (e *testEnv) withWorkflow(name, yamlContent string) *testEnv {
	e.t.Helper()
	e.wfName = name
	wfDir := filepath.Join(e.jerryDir, name)
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		e.t.Fatalf("create workflow dir %q: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(yamlContent), 0o644); err != nil {
		e.t.Fatalf("write workflow.yaml: %v", err)
	}
	return e
}

func (e *testEnv) withAgent(wfName, filename, content string) *testEnv {
	e.t.Helper()
	wfDir := filepath.Join(e.jerryDir, wfName)
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		e.t.Fatalf("create workflow dir %q: %v", wfName, err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, filename), []byte(content), 0o644); err != nil {
		e.t.Fatalf("write agent file %q: %v", filename, err)
	}
	return e
}

func (e *testEnv) withFile(path, content string) *testEnv {
	e.t.Helper()
	fullPath := filepath.Join(e.repoRoot, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		e.t.Fatalf("create dirs for %q: %v", path, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		e.t.Fatalf("write file %q: %v", path, err)
	}
	return e
}

func (e *testEnv) withTrigger(td trigger.TriggerData) *testEnv {
	e.t.Helper()
	e.triggerData = &td
	return e
}

func (e *testEnv) withTriggerFile(jsonContent string) *testEnv {
	e.t.Helper()
	tmpFile := filepath.Join(e.t.TempDir(), "trigger.json")
	if err := os.WriteFile(tmpFile, []byte(jsonContent), 0o644); err != nil {
		e.t.Fatalf("write trigger file: %v", err)
	}
	td, err := trigger.FromFile(tmpFile)
	if err != nil {
		e.t.Fatalf("parse trigger file: %v", err)
	}
	e.triggerData = td
	return e
}

func (e *testEnv) withLLMResponses(responses ...llm.CompleteResponse) *testEnv {
	e.t.Helper()
	e.provider.responses = responses
	return e
}

func (e *testEnv) withGitHubAPI() *testEnv {
	e.t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var bodyMap map[string]any
		_ = json.Unmarshal(body, &bodyMap)

		e.githubMu.Lock()
		e.githubReqs = append(e.githubReqs, capturedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Body:   bodyMap,
		})
		e.githubMu.Unlock()

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"id": 1}`)
	}))
	e.t.Cleanup(server.Close)
	e.githubURL = server.URL
	e.githubToken = "test-token"
	return e
}

func (e *testEnv) run() testResult {
	e.t.Helper()

	if e.wfName == "" {
		e.t.Fatal("withWorkflow not called before run()")
	}

	runsDir := filepath.Join(e.jerryDir, "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		e.t.Fatalf("create runs dir: %v", err)
	}

	printer := output.NewPrinter(io.Discard, io.Discard)
	stateStore := run.NewFileStateStore(runsDir)
	loader := workflow.NewLoader(e.jerryDir)
	reg := tool.NewRegistry(e.repoRoot, nil)
	toolsDir := filepath.Join(e.jerryDir, "tools")
	_ = reg.LoadCustomTools(toolsDir, e.repoRoot, nil)
	agentLoader := agent.NewLoader("claude-sonnet-4-6")
	resolver := llm.NewProviderResolver()

	if e.githubURL != "" {
		reg.SetGitHubConfig(e.githubURL, e.githubToken)
	}

	agentExec := workflow.NewAgentExecutor(agentLoader, reg, printer, resolver)
	agentExec.ProviderOverride = e.provider
	scriptExec := workflow.NewScriptExecutor(e.repoRoot, nil)

	engine := workflow.NewEngine(
		[]workflow.StepExecutor{agentExec, scriptExec},
		stateStore,
		printer,
		10*time.Minute,
	)

	engine.OnStoreCreated = func(store *run.ContextStore) {
		agentExec.SetStore(store)
		scriptExec.SetStore(store)
		reg.SetTrigger(store.Trigger())
	}

	wf, err := loader.Load(e.wfName)
	if err != nil {
		e.t.Fatalf("load workflow %q: %v", e.wfName, err)
	}

	td := trigger.TriggerData{Type: "manual", Source: "cli"}
	if e.triggerData != nil {
		td = *e.triggerData
	}

	runState, runErr := engine.Run(context.Background(), *wf, td)

	e.githubMu.Lock()
	reqs := make([]capturedRequest, len(e.githubReqs))
	copy(reqs, e.githubReqs)
	e.githubMu.Unlock()

	return testResult{
		runState: runState,
		err:      runErr,
		llmCalls: e.provider.getCalls(),
		httpReqs: reqs,
	}
}
