# Workflows

A workflow is a sequence of steps — agents and shell scripts — that Jerry executes in order. Each workflow lives in its own directory inside `.jerry/` with a `workflow.yaml` file and any agent `.md` files it references.

## Quick Example

```yaml
# .jerry/review/workflow.yaml
steps:
  - agent: reviewer
  - name: test
    run: go test ./...
```

```bash
jerry run review "Check for issues"
```

---

## Directory Layout

```
.jerry/
  settings.yaml              # project-wide permissions (see Permissions)
  settings.local.yaml        # user overrides (gitignored)
  .gitignore                 # ignores runs/ and settings.local.yaml
  runs/                      # local run state (gitignored)
  tools/                     # custom tool definitions
    deploy.yaml
  review/                    # ← a workflow
    workflow.yaml
    reviewer.md
  feature/                   # ← another workflow
    workflow.yaml
    planner.md
    generator.md
    security_reviewer.md     # can be a step OR a subagent tool
```

Each workflow directory is self-contained. Agent `.md` files live alongside their `workflow.yaml`. The same agent file can be used as a workflow step or as a [subagent tool](tools.md#subagent-tools).

---

## Reference

### Top-Level Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `steps` | list | yes | — | Ordered list of steps to execute |
| `hooks` | map | no | — | Lifecycle hooks (see [Hooks](hooks.md)) |
| `description` | string | no | — | Human-readable description. Not used at runtime. |

### Step Fields

Each step must have exactly one of `agent` or `run`. Having both, or neither, is a validation error.

#### Agent Steps

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `agent` | string | yes* | — | Name of the agent `.md` file (without extension) in the workflow directory. Mutually exclusive with `run`. |
| `name` | string | no | agent filename | Display name shown in output and logs |
| `retries` | int | no | `0` | Number of retry attempts after failure. Uses exponential backoff (2s, 4s, 8s, ..., max 30s). |
| `timeout` | duration | no | `10m` | Maximum execution time. Accepts Go duration format: `30s`, `5m`, `1h`. |

```yaml
- agent: reviewer
  name: code-review
  retries: 1
  timeout: 5m
```

#### Script Steps

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `run` | string | yes* | — | Shell command executed via `/bin/sh -c`. Mutually exclusive with `agent`. |
| `name` | string | no | `step-N` | Display name shown in output and logs |
| `retries` | int | no | `0` | Number of retry attempts after failure |
| `timeout` | duration | no | `10m` | Maximum execution time |

```yaml
- name: test
  run: go test ./... && go vet ./...
  timeout: 2m
  retries: 1
```

#### Step Name Rules

- Names default to the agent filename (for agent steps) or `step-1`, `step-2`, etc. (for script steps).
- Names must be unique within a workflow. Duplicate names are a validation error.
- Names appear in logs, `jerry logs` output, hook env vars, and error messages.

---

## Execution Model

### Step Sequencing

Steps execute in the order they appear in `steps:`. Each step completes before the next begins. There is no parallel step execution — but tool calls *within* an agent step execute concurrently.

### Retries

When a step fails and has `retries > 0`:

1. Wait with exponential backoff (base: 2s, max: 30s)
2. Re-execute the step from scratch
3. Repeat up to `retries` times
4. If all attempts fail, the workflow aborts

The total `timeout` applies to each individual attempt, not to all attempts combined.

### Timeout

The `timeout` applies per-attempt. If a step has `retries: 2` and `timeout: 5m`, each of the three attempts (initial + 2 retries) can run for up to 5 minutes.

Default timeout is 10 minutes. Overridable per-step in `workflow.yaml`.

### Failure Behavior

When a step fails (after all retries exhausted):

1. Step is marked as `failed` in run state
2. Workflow status set to `failed`
3. Run state saved to `.jerry/runs/`
4. `on_step_failure` hook fires
5. `on_workflow_failure` hook fires
6. Workflow aborts — remaining steps do not execute
7. Jerry exits with code 1

### Context Flow

Each step's text output is accumulated and made available to subsequent steps. This is how agents communicate — a planner's output becomes the generator's input.

**For agent steps**, previous outputs are prepended to the system prompt:

```
## Trigger
Type: pull_request
Source: github
Intent: Fix auth timeout
URL: https://github.com/org/repo/pull/42
Author: alice

## Previous Steps

### planner
Plan: add timeout parameter to the auth middleware...

### generator
Implemented the changes in auth.go and added tests...

---

<this agent's instructions from its .md file>
```

Empty trigger fields are omitted. If there's no trigger and no previous steps, the agent sees only its own instructions.

**For script steps**, the context is written to a temporary JSON file. The path is in `$JERRY_CONTEXT_FILE`:

```json
{
  "protocol_version": "1.0",
  "run_id": "run_a1b2c3d4e5f6",
  "trigger": {
    "type": "pull_request",
    "source": "github",
    "intent": "Fix auth timeout",
    "number": 42,
    "url": "https://github.com/org/repo/pull/42",
    "author": "alice"
  },
  "steps": [
    {"name": "planner", "output": "Plan: add timeout parameter..."},
    {"name": "generator", "output": "Implemented the changes..."}
  ]
}
```

### Script Step Environment

Shell scripts run with a restricted set of environment variables:

| Variable | Description |
|----------|-------------|
| `PATH` | System PATH |
| `HOME` | User home directory |
| `JERRY_RUN_ID` | Unique run identifier |
| `JERRY_INTENT` | Trigger intent string |
| `JERRY_STEP_NAME` | Current step name |
| `JERRY_CONTEXT_FILE` | Path to JSON context file |
| `JERRY_SECRET_*` | All declared secrets |

Working directory is the repository root. Shell is `/bin/sh` (POSIX-compatible).

---

## State and Logging

### Run State

Each workflow execution creates a run directory in `.jerry/runs/<run-id>/`:

```
.jerry/runs/run_a1b2c3d4/
  state.json     # run metadata, step results, status
  log.jsonl      # structured execution log (NDJSON)
  context.json   # cumulative context snapshot
```

Run state is saved:
- At initialization (before first step)
- After each step completes (checkpoint)
- At workflow completion or failure (final)

### Run State Fields

| Field | Type | Description |
|-------|------|-------------|
| `run_id` | string | Unique identifier |
| `workflow_name` | string | Workflow that was executed |
| `workflow_file` | string | Absolute path to workflow.yaml |
| `status` | string | `running`, `completed`, or `failed` |
| `started_at` | timestamp | When the run started |
| `completed_at` | timestamp | When the run ended (null if running) |
| `current_step` | int | Index of current/last step |
| `total_steps` | int | Total steps in workflow |
| `step_results` | array | Outcome of each step |

### Step Result Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Step name |
| `type` | string | `agent` or `script` |
| `status` | string | `success`, `failed`, or `skipped` |
| `started_at` | timestamp | When the step started |
| `completed_at` | timestamp | When the step ended |
| `duration_ms` | int | Execution time in milliseconds |
| `retries_used` | int | Number of retries consumed |
| `tokens_input` | int | Input tokens used (agent steps) |
| `tokens_output` | int | Output tokens generated (agent steps) |
| `stdout` | string | Captured output |
| `error` | object | Error details if failed (`code` + `message`) |

### NDJSON Log

The `log.jsonl` file contains structured entries for every event:

| Type | Description |
|------|-------------|
| `workflow_start` | Run ID, workflow name, trigger intent, step list |
| `workflow_end` | Status, duration, steps completed/failed/skipped |
| `step_start` | Step name, type, agent file |
| `step_end` | Status, duration |
| `llm_call` | Model, tokens in/out, stop reason, duration |
| `tool_call` | Tool name, input, result, duration |
| `compaction` | Context compaction event |
| `retry` | Step retry attempt |

View with `jerry logs <run-id> --json` or parse directly.

---

## Resume

Failed runs can be resumed from the point of failure:

```bash
jerry logs                              # find the run ID
jerry run review --resume run_abc123    # resume from failed step
```

Jerry:
1. Loads the saved run state
2. Validates the workflow hasn't changed
3. Restores the context from previous steps
4. Re-executes from the failed step onward

If the run status shows `running` (from a crashed process), use `--force`:

```bash
jerry run review --resume run_abc123 --force
```

---

## Examples

### Single-step review

```yaml
steps:
  - agent: reviewer
```

### Multi-step feature pipeline

```yaml
steps:
  - agent: planner
  - agent: generator
    retries: 2
    timeout: 10m
  - name: build
    run: go build ./...
  - name: test
    run: go test ./...
    timeout: 3m
```

### Review with triage and subagents

```yaml
steps:
  - agent: triage_reviewer
```

Where `triage_reviewer.md` delegates to `security_reviewer.md` and `performance_reviewer.md` via [subagent tools](tools.md#subagent-tools).

### Full pipeline with hooks

```yaml
description: Feature implementation pipeline triggered by Jira tickets

steps:
  - agent: planner
  - agent: generator
    retries: 2
    timeout: 10m
  - name: test
    run: go test ./... && go vet ./...
    timeout: 3m

hooks:
  on_workflow_start:
    - run: |
        curl -s -X POST $JERRY_SECRET_SLACK_WEBHOOK \
          -H 'Content-Type: application/json' \
          -d "{\"text\":\"▸ Jerry working on: $JERRY_HOOK_TRIGGER_INTENT\"}"
  on_workflow_complete:
    - run: |
        curl -s -X POST $JERRY_SECRET_SLACK_WEBHOOK \
          -H 'Content-Type: application/json' \
          -d "{\"text\":\"✓ Jerry completed in ${JERRY_HOOK_DURATION_MS}ms\"}"
  on_workflow_failure:
    - run: |
        curl -s -X POST $JERRY_SECRET_SLACK_WEBHOOK \
          -H 'Content-Type: application/json' \
          -d "{\"text\":\"✗ Jerry failed at $JERRY_HOOK_FAILED_STEP: $JERRY_HOOK_ERROR\"}"
  after_tool_call:
    - tools: [write_file]
      run: echo "$(date -Iseconds) WRITE $JERRY_HOOK_TOOL_INPUT" >> .jerry/runs/audit.log
```
