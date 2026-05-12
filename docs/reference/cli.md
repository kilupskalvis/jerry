# CLI Reference

Jerry is a single binary with five commands. Global flags apply to all commands.

## Global Flags

| Flag | Description |
|------|-------------|
| `--verbose` | Show detailed output: tool arguments, results, LLM turn summaries, token counts, cache stats |
| `--quiet` | Show only errors and the final result |
| `--version` | Print Jerry version and exit |

Verbosity levels are mutually exclusive. Default shows step progress and tool names.

---

## jerry init

Scaffold a `.jerry/` directory with a workflow template and CI configuration.

```bash
jerry init                          # creates .jerry/review/ + settings.yaml + CI config
jerry init --template feature       # adds .jerry/feature/ workflow
jerry init --path /path/to/repo     # initialize in a specific directory
jerry init --ci github              # generate CI config only (no workflow scaffold)
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--path` | string | current directory | Directory to initialize in |
| `--template` | string | `review` | Workflow template: `review` or `feature` |
| `--ci` | string | — | Generate CI config only, without scaffolding. Values: `github`, `gitlab` |

### What it creates

**First run** (no `--template`):

```
.jerry/
  review/
    workflow.yaml        # single-step review workflow
    reviewer.md          # code review agent
  settings.yaml          # default deny rules (rm -rf, .env, .pem, etc.)
  .gitignore             # ignores runs/ and settings.local.yaml
  runs/                  # local run state directory
```

**With `--template feature`** (adds to existing `.jerry/`):

```
.jerry/
  feature/
    workflow.yaml        # plan → generate → test pipeline
    planner.md           # planning agent
    generator.md         # code generation agent
```

**CI auto-detection:** If a `.github/` directory exists, generates `.github/workflows/jerry-<template>.yml`. If `.gitlab-ci.yml` exists, generates `.jerry-<template>-ci.yml`. Use `--ci` to override detection.

### Errors

| Error | Cause |
|-------|-------|
| `JERRY_DIR_EXISTS` | `.jerry/` already exists (first init). Use `--template` to add workflows. |

---

## jerry run

Execute a workflow. The primary command — this is what runs in CI.

```bash
jerry run review "Check for issues"                           # manual trigger with intent
jerry run review --trigger-file "$GITHUB_EVENT_PATH"          # CI trigger from webhook file
jerry run review --trigger type=pull_request --trigger source=gitlab  # key-value trigger
jerry run feature --dry-run "Add search endpoint"             # validate without executing
jerry run feature --resume run_abc123                         # resume a failed run
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `<workflow>` | yes | Workflow name (directory name under `.jerry/`) |
| `[intent]` | no | Positional intent string. Sets trigger type to `manual`, source to `cli`. |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--intent` | string | — | Trigger intent (alternative to positional argument) |
| `--dry-run` | bool | `false` | Validate and preview the workflow without executing |
| `--trigger-file` | string | — | Path to a JSON trigger file (e.g., `$GITHUB_EVENT_PATH`) |
| `--trigger-stdin` | bool | `false` | Read trigger JSON from stdin |
| `--trigger` | string[] | — | Set trigger fields as `key=value` pairs. Repeatable. |
| `--resume` | string | — | Resume a failed run by run ID |
| `--force` | bool | `false` | Force resume even if run status is `running` |

### Trigger sources

Trigger sources are mutually exclusive — use exactly one:

| Method | When to use |
|--------|-------------|
| Positional `[intent]` | Local development |
| `--trigger-file <path>` | GitHub Actions (`$GITHUB_EVENT_PATH`) |
| `--trigger key=value` | GitLab CI, Jenkins, any platform |
| `--trigger-stdin` | Pipe JSON from another command |

If none specified, trigger defaults to `{type: "manual", source: "cli"}`.

### Dry run

`--dry-run` validates the workflow and all agents without executing:

```
Dry run: review
Trigger: manual (intent: "Check for issues")

Steps:
  1. reviewer          agent   .jerry/review/reviewer.md

Validation: workflow and agents are valid

To execute: jerry run review "Check for issues"
```

### Resume

Resume a failed run from the step that failed:

```bash
jerry run review --resume run_abc123
jerry run review --resume run_abc123 --force   # if status shows 'running' (crashed process)
```

Jerry validates that the workflow structure hasn't changed since the original run. If steps were added, removed, or reordered, resume is rejected.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Workflow completed successfully |
| 1 | Workflow failed (step error, agent max iterations) |
| 2 | Configuration error (invalid YAML, missing agent, missing API key) |
| 3 | Runtime error (LLM unreachable, rate limited, server error) |

---

## jerry validate

Validate all workflows, agents, tools, permissions, and hooks. Run this in CI before `jerry run` to catch errors early.

```bash
jerry validate              # validate everything
jerry validate review       # validate a specific workflow
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `[workflow]` | no | Specific workflow to validate. Validates all if omitted. |

### What it checks

| Check | Example error |
|-------|---------------|
| Workflow YAML structure | `unknown field "step" (did you mean "steps"?)` |
| Step fields and types | `unknown field "retrys" (did you mean "retries"?)` |
| Agent frontmatter fields | `unknown field "temprature" (did you mean "temperature"?)` |
| Agent frontmatter types | `"temperature" must be a number, got string` |
| Provider values | `"provider" must be "anthropic" or "openai", got "deepseek"` |
| Tool resolution | `tool "deploy" not found (available: bash, read_file, ...)` |
| Custom tool structure | `deploy.yaml: missing required field "description"` |
| Custom tool param types | `deploy.yaml: parameter "count" has invalid type "map"` |
| Settings YAML structure | `invalid settings YAML in settings.yaml: ...` |
| Hook event names | `unknown field "on_complet" (did you mean "on_step_complete"?)` |
| Hook definitions | `hooks.on_workflow_complete[0]: missing required field "run"` |
| Hook tool filter scope | `hooks.on_workflow_complete[0]: "tools" filter is only valid on before_tool_call and after_tool_call` |
| Hook tool filter names | `hooks.before_tool_call[0]: unknown tool "nonexistent" in tools filter` |

**"Did you mean?" suggestions** are shown when a typo is within 2 edits of a valid field name.

### Output format

```
  ✓ settings — valid
  ✓ review — valid
  ✗ feature — step "generator": unknown field "temprature" (did you mean "temperature"?)
  ✗ feature — step "generator": tool "deploy" not found (available: ...)
```

---

## jerry logs

View local run history and execution details. For local development only — in CI, logs go to stdout/stderr and are captured by the CI platform.

```bash
jerry logs                          # list recent runs
jerry logs run_abc123               # detailed view of a specific run
jerry logs --last                   # show the most recent run
jerry logs run_abc123 --step planner  # details for a specific step
jerry logs run_abc123 --tools       # all tool calls in the run
jerry logs run_abc123 --llm         # all LLM calls in the run
jerry logs run_abc123 --json        # raw NDJSON log output
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `[run-id]` | no | Run ID to inspect. Lists all runs if omitted. |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--last` | bool | `false` | Show the most recent run |
| `--step` | string | — | Show details for a specific step |
| `--tools` | bool | `false` | Show all tool calls |
| `--llm` | bool | `false` | Show all LLM calls |
| `--json` | bool | `false` | Output raw NDJSON log entries |

---

## jerry setup

Interactive wizards for configuring external integrations.

### jerry setup jira

Generates copy-paste-ready configuration for connecting Jira to Jerry via CI.

```bash
jerry setup jira
```

Auto-detects your repository (from git remote) and CI platform (from `.github/` or `.gitlab-ci.yml`). Prompts for any missing information. Prints step-by-step instructions including:

- GitHub PAT or GitLab trigger token creation
- CI secret configuration
- Jira Automation rule with webhook URL and body template
- Testing instructions
