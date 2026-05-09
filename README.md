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
<b>The agent runtime for CI/CD</b><br>
Define AI agents in Markdown. They run in your pipeline — reviewing code, scanning for vulnerabilities, generating features — using the infrastructure you already have.
</p>

<p align="center">
<a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/kilupskalvis/jerry" alt="Go Version"></a>
<a href="https://github.com/kilupskalvis/jerry/releases"><img src="https://img.shields.io/github/v/release/kilupskalvis/jerry" alt="Release"></a>
<a href="LICENSE"><img src="https://img.shields.io/github/license/kilupskalvis/jerry" alt="License"></a>
</p>

## The Idea

You already have an orchestrator. GitHub Actions, GitLab CI, Jenkins — they handle triggers, runners, secrets, permissions, and job coordination. You don't need another one.

What you need is a way to make a CI step say: "an AI agent does this task" — reviews code, scans for vulnerabilities, generates documentation, or implements a feature — without deploying a separate platform.

Jerry is a **CLI binary that runs as a single CI step.** Your CI defines _when_ to run. Jerry defines _what the AI does._

```yaml
# .github/workflows/review.yml — CI defines the trigger
on:
  pull_request:
    types: [opened, synchronize]
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: jerry run review --trigger-file "$GITHUB_EVENT_PATH"
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

```yaml
# .jerry/review/workflow.yaml — Jerry defines the work
steps:
  - agent: reviewer
```

```markdown
# .jerry/review/reviewer.md — The agent's instructions
---
name: reviewer
model: claude-sonnet-4-6
tools:
  - read_file
  - search_codebase
  - glob
  - git_diff
  - post_pr_comment
---

Read the changed files in this pull request. Check for bugs, security
issues, and violations of project conventions. Post your findings as
PR comments.
```

**No new infrastructure.** The workflow, agent instructions, and tool constraints all live in the repo — versioned, reviewed in PRs, evolving alongside the code they operate on. The same `.jerry/` directory works on GitHub, GitLab, or locally from the terminal.

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
jerry init                                          # Scaffold .jerry/ with example workflow
export ANTHROPIC_API_KEY=sk-ant-...                 # Set API key
jerry run review "Check for common issues"          # Run the review workflow locally
```

## How It Works

Jerry sits between your CI trigger and the task you want automated. When a CI event fires (PR opened, issue labeled, push, manual dispatch), Jerry receives the context and runs a workflow of steps.

**Workflows** are YAML files defining a sequence of steps. Each step is either an agent (AI-powered) or a shell command. Each workflow is a self-contained directory under `.jerry/`:

```
.jerry/
  review/
    workflow.yaml         # Step sequence
    reviewer.md           # Review agent
  feature/
    workflow.yaml
    plan.md               # Planning agent
    generate.md           # Code generation agent
```

**Agents** are Markdown files with YAML frontmatter. The frontmatter declares the model and tools. The body is the agent's instructions — plain English, version-controlled, reviewable in PRs.

**Context flows automatically between steps.** Each step's output is available to every subsequent step — no manual wiring:

```
Step 1 (plan):     sees trigger
Step 2 (generate): sees trigger + plan output       ← automatic
Step 3 (test):     sees trigger + plan + generate   ← automatic
```

**The trigger system** normalizes GitHub and GitLab webhook payloads automatically. A GitHub issue becomes `{type: "ticket", intent: "Add dark mode support"}`. A GitLab MR becomes `{type: "pull_request", intent: "Fix auth timeout"}`. Your CI passes the event JSON, Jerry extracts what the agent needs.

**Agent activity streams to your CI logs.** Every tool call, every decision — visible in your CI UI in real-time, same as any other CI step. No separate log viewer needed.

## Use Cases

Jerry supports any task where an AI agent adds value in a CI pipeline:

| Use Case | Trigger | Example |
|----------|---------|---------|
| **Code review** | PR opened | Agent reads diff, posts review comments |
| **Security scan** | PR opened | Agent audits changes, reports vulnerabilities |
| **Feature generation** | Issue labeled | Agent plans, writes code, runs tests |
| **Documentation** | Push to main | Agent updates docs from code changes |
| **Dependency audit** | Schedule | Agent checks for vulnerable packages |
| **Compliance** | PR opened | Agent verifies regulatory requirements |

## Commands

| Command | Description |
|---------|-------------|
| `jerry init` | Scaffold `.jerry/` with an example workflow |
| `jerry run <workflow> [intent]` | Execute a workflow |
| `jerry run <workflow> --trigger-file <path>` | Execute with a CI webhook payload |
| `jerry run <workflow> --dry-run` | Validate and preview without executing |
| `jerry validate [workflow]` | Validate workflows and agent definitions |
| `jerry logs` | Show recent local runs |
| `jerry version` | Show Jerry version |

Global flags: `--verbose`, `--quiet`.

## Agent Tools

Agents declare which tools they can use. The runtime enforces access.

**Codebase tools:**

| Tool | Description |
|------|-------------|
| `read_file` | Read file contents with line numbers |
| `write_file` | Write content to a file |
| `glob` | Find files matching a pattern |
| `search_codebase` | Regex search across file contents |
| `run_command` | Execute a shell command |
| `list_directory` | List directory contents |
| `git_log` | View recent commits |
| `git_diff` | View diff against a ref |
| `git_blame` | View line-by-line attribution |

**Output routing tools:**

| Tool | Description |
|------|-------------|
| `post_pr_comment` | Post a comment on the triggering PR |
| `post_review_comment` | Post an inline review comment |
| `create_issue` | Create a new issue |
| `add_check_status` | Report a status check result |

### Tool Constraints

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
| `GITHUB_TOKEN` | Required for output routing tools |
| `JERRY_SECRET_*` | Passed to shell step environments |

A `.env` file in the repository root is loaded automatically. Process environment takes precedence.

### Model Selection

Provider is auto-detected from the model name (`claude-*` → Anthropic, `gpt-*`/`o1-*`/`o3-*`/`o4-*` → OpenAI). For custom models, set `provider` in the agent frontmatter:

```yaml
provider: openai
model: ft:gpt-4o:my-org:custom-model
```

## CI Integration

Jerry plugs into the CI you already run. No adapter services, no webhook receivers, no additional infrastructure.

### GitHub Actions

```yaml
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go install github.com/kilupskalvis/jerry/cmd/jerry@latest
      - run: jerry run review --trigger-file "$GITHUB_EVENT_PATH"
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### GitLab CI

```yaml
review:
  script:
    - go install github.com/kilupskalvis/jerry/cmd/jerry@latest
    - >
      echo '{"object_kind":"merge_request","object_attributes":{"title":"'"$CI_MERGE_REQUEST_TITLE"'","iid":'"$CI_MERGE_REQUEST_IID"'}}' |
      jerry run review --trigger-stdin
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
```

> **Note:** GitLab CI doesn't provide a single event payload file like GitHub's `$GITHUB_EVENT_PATH`. The example above constructs a minimal trigger from CI variables. For richer context, use a GitLab webhook to write the full payload to a file and pass it via `--trigger-file`.

### External Ticket Systems (Jira, Linear, etc.)

Jerry works with any ticket system that can fire a webhook. The pattern: platform automation triggers CI dispatch, ticket data travels with the event.

**Jira example — assign a ticket to Jerry, it starts working instantly:**

1. **Jira Automation rule** (one-time setup):

```
Trigger: When assignee = "Jerry"
Action: Send HTTP request
  POST https://api.github.com/repos/{owner}/{repo}/dispatches
  Body: {
    "event_type": "jerry-ticket",
    "client_payload": {
      "type": "ticket",
      "source": "jira",
      "intent": "{{issue.summary}}",
      "raw_payload": {
        "key": "{{issue.key}}",
        "summary": "{{issue.summary}}",
        "description": "{{issue.description}}"
      }
    }
  }
```

2. **GitHub Actions workflow:**

```yaml
on:
  repository_dispatch:
    types: [jerry-ticket]
jobs:
  feature:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: jerry run feature --trigger-file "$GITHUB_EVENT_PATH"
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

No middleware, no servers. Ticket assigned → Jira fires automation → CI starts → Jerry runs. Under a minute.

The same pattern works with Linear, Shortcut, Notion, or any platform with outbound webhooks. Jerry accepts any trigger JSON with `type` and `source` fields as a pre-normalized trigger — no platform-specific adapter needed.

## Development

```bash
go test ./...           # Run tests
go test -race ./...     # Run with race detector
go build ./cmd/jerry    # Build
```

## License

MIT
