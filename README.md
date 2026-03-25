<table align="center"><tr><td>
<pre>
     ██╗███████╗██████╗ ██████╗ ██╗   ██╗
     ██║██╔════╝██╔══██╗██╔══██╗╚██╗ ██╔╝
     ██║█████╗  ██████╔╝██████╔╝ ╚████╔╝
██   ██║██╔══╝  ██╔══██╗██╔══██╗  ╚██╔╝
╚█████╔╝███████╗██║  ██║██║  ██║   ██║
 ╚════╝ ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝
</pre>
</td></tr></table>

<p align="center">
<b>The open protocol for composable AI code generation</b><br>
GitHub Actions brought composability to CI/CD. Jerry brings it to AI-powered software development.
</p>

<p align="center">
<a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/kilupskalvis/jerry" alt="Go Version"></a>
<a href="https://github.com/kilupskalvis/jerry/releases"><img src="https://img.shields.io/github/v/release/kilupskalvis/jerry" alt="Release"></a>
<a href="LICENSE"><img src="https://img.shields.io/github/license/kilupskalvis/jerry" alt="License"></a>
</p>

## The Problem

AI code generation tools — Devin, SWE-agent, OpenHands, Factory — are monolithic. Each one builds the full stack: codebase analysis, planning, code generation, validation, and publishing, all tightly coupled into a single product. This means:

- **Vendor lock-in.** Switching how code generation works means replacing the entire tool.
- **No customization.** A fintech team and a game studio have different conventions, but every agent works the same way for everyone.
- **Wasted ecosystem effort.** If someone builds a great codebase analyzer, it only works inside their tool. Nobody else benefits.

## The Solution

Jerry defines a protocol and runtime that makes AI code generation composable. Teams define pipelines in YAML, configure agents in Markdown, and compose steps from any source. Swap the analyzer, swap the generator, swap the model — without replacing anything else.

```yaml
# .jerry/pipelines/feature.yaml
name: feature
steps:
  - name: plan
    agent: ./agents/plan.md           # Analyze codebase and plan changes
  - name: generate
    agent: ./agents/generate.md       # Implement the plan
  - name: validate
    script: go test ./...             # Run tests
```

This is not another AI coding agent. It is the infrastructure layer that agents run on — like GitHub Actions is for CI/CD.

## Installation

### Go Install

```bash
go install github.com/kilupskalvis/jerry/cmd/jerry@latest
```

### Build from Source

```bash
git clone https://github.com/kilupskalvis/jerry.git
cd jerry
go build -o jerry ./cmd/jerry
```

## Quick Start

```bash
jerry init                                          # Scaffold project
export ANTHROPIC_API_KEY=sk-ant-...                 # Set API key
jerry run feature "Add a GET /health endpoint"      # Generate code
jerry logs                                          # See what happened
```

## How It Works

**Pipelines** are YAML files that define a sequence of steps. Each step is either an agent (autonomous AI) or a script (deterministic shell command).

**Agents** are Markdown files with YAML frontmatter. The frontmatter declares the model, tools, and output schema. The body contains the agent's instructions — written in plain English, version-controlled like code, reviewable in PRs.

```markdown
---
name: code-generator
model: claude-sonnet-4-6
tools:
  - read_file
  - write_file
  - run_command:
      allow: [go test, go build]
context_access:
  - trigger
  - codebase
  - plan
output_key: generation
output_schema:
  artifacts:
    type: array
    items:
      path: string
      action: string
  tests_passed: boolean
---

# Code Generator

Read the plan. For each file to create, read the pattern file first,
then write new code that follows the same conventions exactly.
After writing all files, run the build and test suite.
```

**Steps communicate through a shared context object.** Each step reads from keys written by previous steps and writes to its own `output_key`. No step talks to another directly — the context is the only interface:

```
trigger → plan agent → generate agent → validate script
           reads:        reads:           reads:
           trigger       trigger          generation
           writes:       plan
           plan          writes:
                         generation
```

**The runtime** handles orchestration, tool execution, retries, state persistence, context window management, and structured logging. It supports Anthropic (Claude) and OpenAI (GPT) via their official SDKs, with automatic provider selection based on the model name.

## Core Agents

`jerry init` ships two agents:

| Agent | Purpose | Tools | Iteration Budget |
|-------|---------|-------|-----------------|
| `plan.md` | Explore the codebase and produce an ordered implementation plan | read_file, search_codebase, glob, list_directory, git_log | 30 |
| `generate.md` | Implement the plan, run build and tests, fix failures | read_file, write_file, glob, search_codebase, run_command, list_directory, git_log | 50 |

These are starting points. Add your team's conventions to the Markdown body, split into more steps, or replace with your own agents. See [docs/customizing-agents.md](docs/customizing-agents.md).

## Commands

| Command | Description |
|---------|-------------|
| `jerry init` | Scaffold `.jerry/` with agents, pipelines, and config |
| `jerry init --ci github` | Also generate GitHub Actions workflow |
| `jerry run <pipeline> [intent]` | Execute a pipeline |
| `jerry run <pipeline> --dry-run` | Preview without executing |
| `jerry run --resume <run-id>` | Resume a failed run from the last checkpoint |
| `jerry logs` | Show project overview and recent runs |
| `jerry logs <run-id>` | Run details with step breakdown |
| `jerry logs <run-id> --step <name>` | Tool calls for a specific step |
| `jerry logs <run-id> --tools` | All tool calls across steps |
| `jerry logs <run-id> --llm` | All LLM calls with token counts |
| `jerry logs --last` | Most recent run |

Global flags: `--verbose`, `--quiet`.

## Agent Tools

Agents declare which tools they can use. The runtime enforces access.

| Tool | Description |
|------|-------------|
| `read_file` | Read file contents with line numbers |
| `write_file` | Write content to a file |
| `glob` | Find files matching a pattern |
| `search_codebase` | Regex search across file contents |
| `run_command` | Execute a shell command |
| `list_directory` | List directory contents |
| `git_log` | View recent commits |
| `git_diff` | View uncommitted changes |
| `git_blame` | View line-by-line attribution |

### Constraints

Restrict what tools can do per agent:

```yaml
tools:
  - read_file
  - write_file:
      restrict_to: [src/, tests/]
  - run_command:
      allow: [go test, go build, go vet]
      deny: [rm, curl, wget]
```

## Configuration

### `.jerry/config.yaml`

Project-wide defaults. Agent frontmatter takes precedence.

```yaml
defaults:
  model: claude-sonnet-4-6
  timeout: 600s
  max_iterations: 50
  context_window: 200000
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | API key for Claude models |
| `OPENAI_API_KEY` | API key for GPT and O-series models |
| `JERRY_DEFAULT_MODEL` | Override default model |
| `JERRY_SECRET_*` | Passed to script step environments |

A `.env` file in the repository root is loaded automatically. Process environment takes precedence.

### Model Selection

Provider is selected from the model name prefix (`claude-*` → Anthropic, `gpt-*`/`o1-*`/`o3-*`/`o4-*` → OpenAI). For custom models, set `provider` in the agent frontmatter:

```yaml
provider: openai
model: ft:gpt-4o:my-org:custom-model
```

## CI Integration

### GitHub Actions

```bash
jerry init --ci github
```

Generates a workflow that runs the feature pipeline when issues are labeled `jerry`. Or use the action directly in any workflow:

```yaml
- uses: jerry-protocol/runner@v1
  with:
    pipeline: feature
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

The action handles checkout, install, pipeline execution, and PR creation — one step, no boilerplate.

### GitLab CI

```bash
jerry init --ci gitlab
```

Generates a GitLab CI job triggered via the pipeline API or web UI.

## Runtime Features

- **Context window management**: automatic compaction when conversations exceed the model's context limit — reactive (on API error) and proactive (at 80% of configured limit)
- **Resumable pipelines**: state checkpointed after every step, resume from the failure point with `jerry run --resume <run-id>`
- **Structured logging**: every LLM call, tool call, and decision logged to JSONL with timestamps and token counts
- **Output validation**: agent output validated against JSON Schema translated from the simplified frontmatter notation
- **Retry and fallback**: configurable per-step retry with fixed or exponential backoff, optional fallback steps
- **Sensitive file protection**: agents are blocked from reading `.env` and other secret-bearing files

## Project Structure

```
.jerry/
  pipelines/            # Pipeline definitions (YAML)
  agents/               # Agent definitions (Markdown)
  scripts/              # Shell scripts for deterministic steps
  config.yaml           # Project defaults
  runs/                 # Execution state and logs (gitignored)
    <run-id>/
      state.json        # Context snapshot for resume
      log.jsonl         # Structured event log
```

## Development

```bash
go test ./...           # Run tests
go test -race ./...     # Run with race detector
go build ./cmd/jerry    # Build
golangci-lint run       # Lint
```

## License

MIT
