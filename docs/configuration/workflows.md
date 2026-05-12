# Workflows

Workflows define a sequence of steps — agents and scripts — that Jerry executes in order. Each workflow lives in its own directory inside `.jerry/`.

## Quick Example

```yaml
# .jerry/review/workflow.yaml
steps:
  - agent: reviewer
  - name: test
    run: go test ./...
```

Run it:

```bash
jerry run review "Check for issues"
```

## Directory Structure

```
.jerry/
  review/
    workflow.yaml        # this file
    reviewer.md          # agent referenced by the workflow
  feature/
    workflow.yaml
    planner.md
    generator.md
```

Each workflow is a self-contained directory. Agent `.md` files live alongside their `workflow.yaml`.

## Reference

### steps

A list of steps executed in order. Each step is either an **agent step** or a **script step**.

#### Agent Step

Runs an AI agent defined in a Markdown file.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `agent` | string | yes | — | Name of the agent `.md` file (without extension) in the workflow directory |
| `name` | string | no | agent filename | Display name for the step |
| `retries` | int | no | `0` | Number of retry attempts after failure |
| `timeout` | duration | no | `10m` | Maximum execution time (e.g., `30s`, `5m`, `1h`) |

```yaml
steps:
  - agent: reviewer
    retries: 1
    timeout: 5m
```

#### Script Step

Runs a shell command.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `run` | string | yes | — | Shell command executed via `/bin/sh -c` |
| `name` | string | no | `step-N` | Display name for the step |
| `retries` | int | no | `0` | Number of retry attempts after failure |
| `timeout` | duration | no | `10m` | Maximum execution time |

```yaml
steps:
  - name: test
    run: go test ./...
    timeout: 2m
```

#### Step Rules

- Each step must have either `agent` or `run`, not both.
- Steps execute sequentially. Each step sees the output of all previous steps.
- If a step fails (after retries), the workflow aborts.
- Step names must be unique within a workflow.

### hooks

Lifecycle hooks — shell commands that fire at workflow, step, and tool events. See [Hooks](hooks.md) for the complete reference.

```yaml
hooks:
  on_workflow_complete:
    - run: curl -X POST $JERRY_SECRET_SLACK_WEBHOOK -d '{"text":"Done"}'
```

### description

Optional text description of the workflow. Not used at runtime.

```yaml
description: Reviews pull requests for bugs and style issues
steps:
  - agent: reviewer
```

## Context Flow

Each step's text output is passed to subsequent steps automatically.

**Agents** receive previous outputs in their system prompt:

```
## Previous Steps

### planner
Plan: add /search endpoint with query parameter...

---

<agent's own instructions>
```

**Scripts** receive previous outputs via a JSON file at `$JERRY_CONTEXT_FILE`:

```json
{
  "run_id": "run_abc123",
  "trigger": {"type": "manual", "source": "cli", "intent": "Add search"},
  "steps": [
    {"name": "planner", "output": "Plan: add /search endpoint..."}
  ]
}
```

## Script Environment

Script steps run with these environment variables:

| Variable | Description |
|----------|-------------|
| `JERRY_RUN_ID` | Current run identifier |
| `JERRY_INTENT` | Trigger intent description |
| `JERRY_STEP_NAME` | Current step name |
| `JERRY_CONTEXT_FILE` | Path to JSON file with previous step outputs |
| `JERRY_SECRET_*` | Secrets from environment or `.env` file |

## Examples

### Code review with tests

```yaml
steps:
  - agent: reviewer
  - name: test
    run: go test ./...
    timeout: 2m
```

### Plan → generate → test with retries

```yaml
steps:
  - agent: planner
  - agent: generator
    retries: 2
    timeout: 10m
  - name: test
    run: go test ./... && go vet ./...
```

### Review with Slack notification

```yaml
steps:
  - agent: reviewer

hooks:
  on_workflow_complete:
    - run: |
        curl -s -X POST $JERRY_SECRET_SLACK_WEBHOOK \
          -H 'Content-Type: application/json' \
          -d "{\"text\":\"✓ Review complete\"}"
  on_workflow_failure:
    - run: |
        curl -s -X POST $JERRY_SECRET_SLACK_WEBHOOK \
          -H 'Content-Type: application/json' \
          -d "{\"text\":\"✗ Review failed at $JERRY_HOOK_FAILED_STEP\"}"
```
