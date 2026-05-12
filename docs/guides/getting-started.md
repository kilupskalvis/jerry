# Getting Started

Get Jerry running in 5 minutes. By the end, you'll have an AI agent reviewing code in your project.

## Prerequisites

- An Anthropic API key ([get one here](https://console.anthropic.com/))
- A git repository to work in

## Install

```bash
curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh
```

Or via Go:

```bash
go install github.com/kilupskalvis/jerry/cmd/jerry@latest
```

## Set your API key

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

Or create a `.env` file in your project root:

```bash
echo "ANTHROPIC_API_KEY=sk-ant-..." > .env
```

## Initialize

```bash
cd your-project
jerry init
```

This creates:

```
.jerry/
  review/
    workflow.yaml     # single-step review workflow
    reviewer.md       # code review agent
  settings.yaml       # default safety rules
  .gitignore
  runs/
```

## Run your first workflow

```bash
jerry run review "Check for common issues"
```

You'll see the agent work in real-time:

```
jerry: Running workflow: review (1 steps)
  ▸ reviewer ...
    -> bash({"command":"find . -name '*.go' | head -20"})
    -> read_file({"path":"main.go"})
    [response] Found 3 potential issues...
  ✓ reviewer (12.4s)
jerry: Workflow completed in 12.4s (run: run_abc123)
```

Use `--verbose` for detailed output including tool results and token counts.

## Validate your configuration

```bash
jerry validate
```

```
  ✓ review — valid
```

This catches typos, missing tools, invalid settings — everything that would cause a failed run.

## What's next

### Add to CI

```bash
jerry init --ci github    # generates .github/workflows/jerry-review.yml
```

Add `ANTHROPIC_API_KEY` to your repository secrets. Push a PR — Jerry reviews it.

See [CI Setup](ci-setup.md) for detailed GitHub Actions and GitLab CI configuration.

### Add a feature workflow

```bash
jerry init --template feature
```

Creates a plan → generate → test pipeline. See [Workflows](../configuration/workflows.md) for how to customize it.

### Customize the reviewer

Edit `.jerry/review/reviewer.md` to match your team's review standards. The Markdown body is the agent's instructions — tell it what to focus on.

See [Agents](../configuration/agents.md) for the complete frontmatter reference.

### Add safety rules

Edit `.jerry/settings.yaml` to control what agents can and cannot do:

```yaml
permissions:
  deny:
    - bash: ["rm -rf *", "curl * | sh"]
    - write_file: ["*.env", "*.key"]
```

See [Permissions](../configuration/permissions.md) for glob patterns and per-agent rules.

### Add notifications

Add hooks to your workflow to get notified:

```yaml
# .jerry/review/workflow.yaml
steps:
  - agent: reviewer

hooks:
  on_workflow_complete:
    - run: curl -X POST $JERRY_SECRET_SLACK_WEBHOOK -d '{"text":"Review done"}'
```

See [Hooks](../configuration/hooks.md) for all lifecycle events.

### Connect Jira

```bash
jerry setup jira
```

Generates copy-paste-ready Jira Automation rules. See [Jira Integration](jira-integration.md).
