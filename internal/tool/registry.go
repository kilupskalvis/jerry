package tool

import (
	"fmt"
	"strings"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

type githubCfg struct {
	BaseURL string
	Token   string
}

// Registry manages built-in, CI, custom, and agent tools.
type Registry struct {
	baseTools  []Tool
	ciTools    map[string]Tool
	custom     map[string]Tool
	agents     map[string]Tool
	triggerRef *trigger.TriggerData
	ghCfg      *githubCfg
}

// NewRegistry creates a registry with always-on base tools and CI tools.
func NewRegistry(repoRoot string, secretEnv map[string]string) *Registry {
	envSlice := make([]string, 0, len(secretEnv))
	for k, v := range secretEnv {
		envSlice = append(envSlice, k+"="+v)
	}

	r := &Registry{
		ciTools: make(map[string]Tool),
		custom:  make(map[string]Tool),
		agents:  make(map[string]Tool),
		ghCfg:   &githubCfg{},
	}

	r.baseTools = []Tool{
		NewBashTool(repoRoot, envSlice),
		NewReadFileTool(repoRoot),
		NewWriteFileTool(repoRoot),
	}

	r.triggerRef = &trigger.TriggerData{}
	r.registerCI(NewPostPRCommentTool(r.triggerRef, r.ghCfg))
	r.registerCI(NewPostReviewCommentTool(r.triggerRef, r.ghCfg))
	r.registerCI(NewAddCheckStatusTool(r.triggerRef, r.ghCfg))
	r.registerCI(NewCreatePullRequestTool(repoRoot, r.triggerRef, r.ghCfg))

	return r
}

func (r *Registry) registerCI(t Tool) {
	r.ciTools[t.Name()] = t
}

// SetTrigger injects trigger data for output routing tools.
func (r *Registry) SetTrigger(t trigger.TriggerData) {
	*r.triggerRef = t
}

// SetGitHubConfig injects GitHub API base URL and token.
func (r *Registry) SetGitHubConfig(baseURL, token string) {
	r.ghCfg.BaseURL = baseURL
	r.ghCfg.Token = token
}

// LoadCustomTools discovers and registers custom tools from a directory.
func (r *Registry) LoadCustomTools(toolsDir, repoRoot string, secretEnv []string) error {
	tools, err := LoadCustomTools(toolsDir, repoRoot, secretEnv)
	if err != nil {
		return err
	}
	for _, t := range tools {
		r.custom[t.Name()] = t
	}
	return nil
}

// RegisterAgentTool registers an agent as a callable tool.
func (r *Registry) RegisterAgentTool(t Tool) {
	r.agents[t.Name()] = t
}

// ClearAgentTools removes all registered agent tools.
func (r *Registry) ClearAgentTools() {
	r.agents = make(map[string]Tool)
}

// BaseTools returns the always-on tools injected into every agent.
func (r *Registry) BaseTools() []Tool {
	result := make([]Tool, len(r.baseTools))
	copy(result, r.baseTools)
	return result
}

// Resolve resolves opt-in tool declarations (CI + custom tools).
// Names matching base tools are silently skipped to avoid duplicates.
func (r *Registry) Resolve(toolAccess []ToolAccess) ([]Tool, error) {
	baseNames := make(map[string]struct{}, len(r.baseTools))
	for _, t := range r.baseTools {
		baseNames[t.Name()] = struct{}{}
	}

	resolved := make([]Tool, 0, len(toolAccess))
	for _, ta := range toolAccess {
		if _, isBase := baseNames[ta.Name]; isBase {
			continue
		}

		if t, ok := r.ciTools[ta.Name]; ok {
			resolved = append(resolved, t)
			continue
		}

		if t, ok := r.custom[ta.Name]; ok {
			resolved = append(resolved, t)
			continue
		}

		if t, ok := r.agents[ta.Name]; ok {
			resolved = append(resolved, t)
			continue
		}

		return nil, fmt.Errorf("unknown tool %q (available: %s)",
			ta.Name, strings.Join(r.KnownToolNames(), ", "))
	}

	return resolved, nil
}

// KnownToolNames returns names of all registered tools.
func (r *Registry) KnownToolNames() []string {
	names := make([]string, 0, len(r.baseTools)+len(r.ciTools)+len(r.custom)+len(r.agents))
	for _, t := range r.baseTools {
		names = append(names, t.Name())
	}
	for name := range r.ciTools {
		names = append(names, name)
	}
	for name := range r.custom {
		names = append(names, name)
	}
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}
