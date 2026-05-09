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
<b>Turn your CI into an autonomous development platform</b><br>
Define AI agents in Markdown, wire them into workflows, run them as a CI step. No new infrastructure.
</p>

<p align="center">
<a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/kilupskalvis/jerry" alt="Go Version"></a>
<a href="https://github.com/kilupskalvis/jerry/releases"><img src="https://img.shields.io/github/v/release/kilupskalvis/jerry" alt="Release"></a>
<a href="LICENSE"><img src="https://img.shields.io/github/license/kilupskalvis/jerry" alt="License"></a>
</p>

## The Idea

You already have an orchestrator. GitHub Actions, GitLab CI, Jenkins — they have triggers, runners, secrets management, permissions, artifact storage, and job coordination. You don't need another one.

What you need is a way to make a CI step say: "an AI agent understands this codebase, plans changes, writes code, and runs the tests" — without deploying a separate platform, standing up a queue, or running an agent server.

Jerry is a **CLI binary that runs as a single CI step.** Your CI defines _when_ to run. Jerry defines _what the AI does._

```yaml
# .github/workflows/feature.yml — CI defines the trigger
on:
  issues:
    types: [labeled]
jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: jerry run feature --trigger-file "$GITHUB_EVENT_PATH"
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

```yaml
# .jerry/feature/workflow.yaml — Jerry defines the work
steps:
  - agent: plan              # Analyze codebase and plan changes
  - agent: generate          # Implement the plan
  - run: go test ./...       # Run tests
```

**No new infrastructure.** The workflow definition, agent instructions, and tool constraints all live in the repo — versioned, reviewed in PRs, and evolving alongside the code they operate on. The same `.jerry/` directory works on GitHub, GitLab, or locally from the terminal.

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
jerry run example "Add a GET /health endpoint"      # Generate code
jerry logs                                          # See what happened
```

## How It Works

Jerry sits between your CI trigger and the actual code changes. When a CI event fires (issue labeled, PR opened, push, manual dispatch), Jerry receives the webhook payload, normalizes it into a trigger (intent + metadata), and runs a workflow of steps against your codebase.

**Workflows** are YAML files that define a sequence of steps. Each step is either an agent (autonomous AI) or a shell command. Each workflow is a self-contained directory under `.jerry/`:

```
.jerry/
  feature/
    workflow.yaml         # Step sequence
    plan.md               # Planning agent
    generate.md           # Code generation agent
```

**Agents** are Markdown files with YAML frontmatter. The frontmatter declares the model and tools. The body contains the agent's instructions — written in plain English, version-controlled like code, reviewable in PRs.

```markdown
---
name: code-generator
model: claude-sonnet-4-6
tools:
  - read_file
  - write_file
  - run_command:
      allow: [go test, go build]
---

# Code Generator

Read the plan. For each file to create, read the pattern file first,
then write new code that follows the same conventions exactly.
After writing all files, run the build and test suite.
```

**Context flows automatically between steps.** Each step's output is available to every subsequent step — no manual wiring. The agent's system prompt is constructed with previous step outputs injected before the agent's instructions:

```
Step 1 (plan):   sees trigger
Step 2 (generate): sees trigger + plan output       ← automatic
Step 3 (run):      sees trigger + plan + generate   ← automatic
```

**The trigger system** normalizes GitHub and GitLab webhook payloads automatically. A GitHub issue becomes `{type: "ticket", intent: "Add dark mode support", source: "github"}`. A GitLab MR becomes `{type: "pull_request", intent: "Fix auth timeout", source: "gitlab"}`. Your CI passes the event JSON, Jerry extracts what the agent needs to know.

**The runtime** handles tool execution, retries, state persistence, context window management, and structured logging. It supports Anthropic (Claude) and OpenAI (GPT) via their official SDKs, with automatic provider selection based on the model name.

## Core Agents

`jerry init` ships two agents in an example workflow:

| Agent | Purpose | Tools |
|-------|---------|-------|
| `plan.md` | Explore the codebase and produce an ordered implementation plan | read_file, search_codebase, glob, list_directory, git_log |
| `generate.md` | Implement the plan, run build and tests, fix failures | read_file, write_file, glob, search_codebase, run_command, list_directory, git_log |

These are starting points. Add your team's conventions to the Markdown body, split into more steps, or replace with your own agents.

## Commands

| Command | Description |
|---------|-------------|
| `jerry init` | Scaffold `.jerry/` with an example workflow |
| `jerry init --ci github` | Also generate GitHub Actions workflow |
| `jerry run <workflow> [intent]` | Execute a workflow |
| `jerry run <workflow> --dry-run` | Preview and validate without executing |
| `jerry run --resume <run-id>` | Resume a failed run from the last checkpoint |
| `jerry validate [workflow]` | Validate workflows and agent definitions |
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

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | API key for Claude models |
| `OPENAI_API_KEY` | API key for GPT and O-series models |
| `JERRY_DEFAULT_MODEL` | Fallback model when agent doesn't specify one |
| `JERRY_SECRET_*` | Passed to shell step environments |

A `.env` file in the repository root is loaded automatically. Process environment takes precedence.

### Model Selection

Provider is selected from the model name prefix (`claude-*` → Anthropic, `gpt-*`/`o1-*`/`o3-*`/`o4-*` → OpenAI). For custom models, set `provider` in the agent frontmatter:

```yaml
provider: openai
model: ft:gpt-4o:my-org:custom-model
```

## CI Integration

Jerry plugs into the CI you already run. It consumes the same webhook payloads your CI already provides — no adapter services, no webhook receivers, no additional infrastructure.

### GitHub Actions

```bash
jerry init --ci github
```

Generates a workflow that runs when issues are labeled `jerry`:

```yaml
on:
  issues:
    types: [labeled]

jobs:
  jerry:
    if: contains(github.event.issue.labels.*.name, 'jerry')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: jerry run feature --trigger-file "$GITHUB_EVENT_PATH"
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

### GitLab CI

```bash
jerry init --ci gitlab
```

Generates a GitLab CI job triggered via the pipeline API or web UI.

## Runtime Features

- **Context window management**: automatic compaction when conversations exceed the model's context limit
- **Resumable workflows**: state checkpointed after every step, resume from the failure point with `jerry run --resume <run-id>`
- **Structured logging**: every LLM call, tool call, and decision logged to JSONL with timestamps and token counts
- **Retry**: configurable per-step retry with fixed backoff
- **Sensitive file protection**: agents are blocked from reading `.env` and other secret-bearing files
- **Provider-agnostic**: swap between Anthropic and OpenAI per agent via the `model` field

## Project Structure

```
.jerry/
  feature/                # One workflow
    workflow.yaml         # Step sequence
    plan.md               # Agent definition
    generate.md           # Agent definition
  hotfix/                 # Another workflow
    workflow.yaml
    quick-fix.md
  runs/                   # Execution state and logs (gitignored)
    <run-id>/
      state.json          # Context snapshot for resume
      log.jsonl           # Structured event log
```

## Development

```bash
go test ./...           # Run tests
go test -race ./...     # Run with race detector
go build ./cmd/jerry    # Build
golangci-lint run       # Lint
lefthook install        # Install git hooks
```

## License

MIT
