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
<b>The agent runtime for CI/CD.</b><br>
AI agents that live in your repo and run in your pipeline. Review PRs, implement features, scan for vulnerabilities — no new infrastructure.
</p>

<p align="center">
<a href="https://github.com/kilupskalvis/jerry/releases"><img src="https://img.shields.io/github/v/release/kilupskalvis/jerry" alt="Release"></a>
<a href="LICENSE"><img src="https://img.shields.io/github/license/kilupskalvis/jerry" alt="License"></a>
</p>

---

**A Jira ticket gets assigned. Two minutes later, a PR appears.**

```yaml
# .jerry/feature/workflow.yaml — three steps, that's it
steps:
  - agent: planner
  - agent: generator
  - name: test
    run: go test ./...
```

```markdown
# .jerry/feature/generator.md — the agent is a Markdown file
---
name: generator
model: claude-sonnet-4-6
tools:
  - create_pull_request
---

Implement the plan from the previous step.
Read existing code, match conventions, build, test, open a PR.
```

```
$ jerry run feature "Add pagination to the users endpoint"

jerry: Running workflow: feature (3 steps)
  ▸ planner ...
    -> bash({"command":"find . -type f -name '*.go' | head -20"})
    -> read_file({"path":"main.go"})
    [response] Plan: add offset/limit params to handleGetUsers...
  ✓ planner (18.2s)
  ▸ generator ...
    -> read_file({"path":"main.go"})
    -> write_file({"path":"main.go","content":"..."})
    -> bash({"command":"go build ./..."})
    -> bash({"command":"go test ./..."})
    -> create_pull_request({"title":"Add pagination to users endpoint"})
  ✓ generator (34.1s)
  ▸ test ...
  ✓ test (0.4s)
jerry: Workflow completed in 52.7s
```

---

## Get Started

```bash
curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh
cd your-project
jerry init                          # creates .jerry/review/ + CI config
jerry run review "Check for issues" # try it locally
```

Add `ANTHROPIC_API_KEY` to your CI secrets. Push a PR. Jerry reviews it.

Want feature generation? Add the feature workflow:

```bash
jerry init --template feature       # adds .jerry/feature/ + CI config
jerry setup jira                    # prints Jira integration config
```

Assign a Jira ticket to Jerry. It plans, implements, tests, and opens a PR.

---

## How It Works

Every agent gets three tools automatically: `bash`, `read_file`, `write_file`. Declare additional tools in the frontmatter:

```markdown
---
name: reviewer
model: claude-sonnet-4-6
tools:
  - post_pr_comment
---

Review the PR for bugs and security issues.
Post findings as a PR comment.
```

Context flows between steps — each agent sees the output of every previous step. No wiring needed.

**Built-in CI tools:** `post_pr_comment`, `post_review_comment`, `add_check_status`, `create_pull_request`

**Custom tools** — define your own as YAML in `.jerry/tools/`:

```yaml
# .jerry/tools/deploy.yaml
description: Deploy a service
parameters:
  service:
    type: string
    required: true
run: |
  curl -X POST "https://deploy.internal/api/v1/deploy" \
    -d "{\"service\": \"$TOOL_SERVICE\"}"
```

---

## CI Integration

Works with GitHub Actions and GitLab CI. `jerry init` auto-detects your platform and generates the config.

<details>
<summary>GitHub Actions</summary>

```yaml
on:
  pull_request:
    types: [opened, synchronize]
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh
      - run: jerry run review --trigger-file "$GITHUB_EVENT_PATH" --verbose
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```
</details>

<details>
<summary>GitLab CI</summary>

```yaml
jerry-review:
  script:
    - curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh
    - >
      jerry run review
      --trigger type=pull_request
      --trigger source=gitlab
      --trigger intent="$CI_MERGE_REQUEST_TITLE"
      --trigger number=$CI_MERGE_REQUEST_IID
      --verbose
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
```
</details>

<details>
<summary>Jira Integration</summary>

```bash
jerry setup jira
```

Prints step-by-step instructions tailored to your repo and CI platform. The flow: Jira ticket assigned → automation fires → CI runs Jerry → PR opens.
</details>

---

## Install

```bash
curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh   # recommended
brew install kilupskalvis/tap/jerry                                                     # or Homebrew
go install github.com/kilupskalvis/jerry/cmd/jerry@latest                               # or Go
```

## Commands

| Command | Description |
|---------|-------------|
| `jerry init` | Scaffold review workflow + CI config |
| `jerry init --template feature` | Add feature development workflow |
| `jerry run <workflow> [intent]` | Execute a workflow |
| `jerry validate` | Validate workflows and agents |
| `jerry setup jira` | Generate Jira integration config |
| `jerry logs` | Show recent local runs |

## Configuration

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Claude models |
| `OPENAI_API_KEY` | GPT / O-series models |
| `GITHUB_TOKEN` | CI tools (auto-provided in GitHub Actions) |
| `JERRY_DEFAULT_MODEL` | Fallback model when agents don't specify one |
| `JERRY_SECRET_*` | Passed to custom tools, shell steps, and hooks |

## Features

- **Guardrails** — deny/allow rules in `settings.yaml` control what tools can do. Block dangerous commands, protect sensitive files. Per-project and per-agent. [→ Permissions](docs/configuration/permissions.md)
- **Subagents** — agents can invoke other agents as tools at runtime. A triage agent delegates to specialists. [→ Tools](docs/configuration/tools.md#subagent-tools)
- **Lifecycle hooks** — shell commands that fire on workflow, step, and tool events. Slack notifications, audit logging, downstream triggers. [→ Hooks](docs/configuration/hooks.md)
- **Deep validation** — `jerry validate` catches typos with "did you mean?" suggestions, verifies tool references, checks types, validates hooks. [→ FAQ](docs/faq.md)
- **Parallel tool execution** — multiple tool calls in one LLM turn execute concurrently.
- **Prompt caching** — Anthropic prompt caching on system prompts and tool definitions. Automatic cost reduction.

## Documentation

### Configuration Reference

- [Workflows](docs/configuration/workflows.md) — workflow.yaml format, steps, context flow
- [Agents](docs/configuration/agents.md) — frontmatter fields, model selection, writing instructions
- [Tools](docs/configuration/tools.md) — built-in tools, custom tools, subagent tools
- [Permissions](docs/configuration/permissions.md) — deny/allow rules, glob patterns, resolution chain
- [Hooks](docs/configuration/hooks.md) — lifecycle events, environment variables, examples

### Guides

- [CI Setup](docs/guides/ci-setup.md) — GitHub Actions + GitLab CI
- [Triggers](docs/guides/triggers.md) — trigger methods, fields, platform examples
- [Jira Integration](docs/guides/jira-integration.md) — full setup walkthrough

### Other

- [FAQ](docs/faq.md) — cost, safety, debugging, common questions

## License

MIT
