# Hooks

Hooks are shell commands that fire at lifecycle events during workflow execution. Use them for notifications, audit logging, or triggering downstream systems. Defined per-workflow in `workflow.yaml`.

## Quick Example

```yaml
steps:
  - agent: reviewer

hooks:
  on_workflow_complete:
    - run: curl -X POST $JERRY_SECRET_SLACK_WEBHOOK -d '{"text":"Review done"}'
```

## Events

| Event | When it fires | Use case |
|-------|--------------|----------|
| `on_workflow_start` | Before the first step executes | Log start, set status |
| `on_workflow_complete` | All steps succeeded | Slack notification, close ticket |
| `on_workflow_failure` | Any step failed and workflow aborted | PagerDuty alert, reopen ticket |
| `on_step_start` | Before a step executes | Log progress |
| `on_step_complete` | A step succeeded | Checkpoint notification |
| `on_step_failure` | A step failed (after retries exhausted) | Alert on specific step |
| `before_tool_call` | Before a tool executes (after guardrails) | Audit logging |
| `after_tool_call` | After a tool returns | Audit trail, file change tracking |

## Hook Format

Each event maps to a list of hook definitions. Each hook has a `run` field and an optional `tools` filter.

```yaml
hooks:
  on_workflow_complete:
    - run: echo "done"
    - run: curl -X POST $JERRY_SECRET_SLACK_WEBHOOK -d '{"text":"Done"}'

  before_tool_call:
    - tools: [write_file]
      run: echo "WRITE $JERRY_HOOK_TOOL_INPUT" >> /tmp/audit.log
    - tools: [bash]
      run: echo "BASH $JERRY_HOOK_TOOL_INPUT" >> /tmp/audit.log
    - run: echo "$JERRY_HOOK_TOOL_NAME" >> /tmp/all-tools.log
```

### Tool Filter

The `tools` field is only valid on `before_tool_call` and `after_tool_call`. If present, the hook fires only for matching tool calls. If omitted, the hook fires for all tool calls.

```yaml
before_tool_call:
  - tools: [write_file]          # only write_file calls
    run: echo "writing..."
  - tools: [bash, write_file]    # bash and write_file
    run: echo "mutating..."
  - run: echo "any tool..."      # all tool calls
```

## Environment Variables

Hooks receive context via environment variables.

### All hooks

| Variable | Description |
|----------|-------------|
| `JERRY_HOOK_EVENT` | Event name |
| `JERRY_HOOK_WORKFLOW` | Workflow name |
| `JERRY_HOOK_RUN_ID` | Current run ID |
| `JERRY_SECRET_*` | Secrets from environment or `.env` |

### Workflow events

Available on `on_workflow_start`, `on_workflow_complete`, `on_workflow_failure`:

| Variable | Description |
|----------|-------------|
| `JERRY_HOOK_STATUS` | `completed` or `failed` |
| `JERRY_HOOK_DURATION_MS` | Total duration in milliseconds |
| `JERRY_HOOK_TRIGGER_TYPE` | Trigger type (e.g., `pull_request`) |
| `JERRY_HOOK_TRIGGER_INTENT` | Trigger intent text |
| `JERRY_HOOK_ERROR` | Error message (failure only) |
| `JERRY_HOOK_FAILED_STEP` | Name of the failed step (failure only) |

### Step events

Available on `on_step_start`, `on_step_complete`, `on_step_failure`:

| Variable | Description |
|----------|-------------|
| `JERRY_HOOK_STEP_NAME` | Step name |
| `JERRY_HOOK_STEP_STATUS` | `success` or `failed` (not on start) |
| `JERRY_HOOK_DURATION_MS` | Step duration in milliseconds (not on start) |
| `JERRY_HOOK_ERROR` | Error message (failure only) |

### Tool events

Available on `before_tool_call`, `after_tool_call`:

| Variable | Description |
|----------|-------------|
| `JERRY_HOOK_STEP_NAME` | Current step name |
| `JERRY_HOOK_TOOL_NAME` | Tool name (e.g., `bash`, `write_file`) |
| `JERRY_HOOK_TOOL_INPUT` | Tool input JSON |
| `JERRY_HOOK_TOOL_OUTPUT` | Tool output (after only) |
| `JERRY_HOOK_TOOL_IS_ERROR` | `true` or `false` (after only) |

## Execution Model

- **Shell:** `/bin/sh -c` (same as script steps)
- **Working directory:** repository root
- **Timeout:** 10 seconds per hook
- **Fire-and-forget:** hook failure is logged as a warning, never affects the workflow exit code or step execution
- **Sequential:** multiple hooks on the same event run in list order
- **No blocking:** hooks cannot prevent tool execution or abort the workflow

## Examples

### Slack notification on complete/failure

```yaml
hooks:
  on_workflow_complete:
    - run: |
        curl -s -X POST $JERRY_SECRET_SLACK_WEBHOOK \
          -H 'Content-Type: application/json' \
          -d "{\"text\":\"✓ Jerry completed $JERRY_HOOK_WORKFLOW (${JERRY_HOOK_DURATION_MS}ms)\"}"
  on_workflow_failure:
    - run: |
        curl -s -X POST $JERRY_SECRET_SLACK_WEBHOOK \
          -H 'Content-Type: application/json' \
          -d "{\"text\":\"✗ Jerry failed at $JERRY_HOOK_FAILED_STEP: $JERRY_HOOK_ERROR\"}"
```

### Audit log for file writes

```yaml
hooks:
  after_tool_call:
    - tools: [write_file]
      run: echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) WRITE $JERRY_HOOK_TOOL_INPUT" >> .jerry/runs/audit.log
```

### Log all tool activity

```yaml
hooks:
  before_tool_call:
    - run: echo "$JERRY_HOOK_TOOL_NAME $JERRY_HOOK_TOOL_INPUT" >> /tmp/jerry-tools.log
  after_tool_call:
    - run: echo "$JERRY_HOOK_TOOL_NAME result=$JERRY_HOOK_TOOL_IS_ERROR" >> /tmp/jerry-tools.log
```

### Step progress tracking

```yaml
hooks:
  on_step_start:
    - run: echo "▸ $JERRY_HOOK_STEP_NAME starting..."
  on_step_complete:
    - run: echo "✓ $JERRY_HOOK_STEP_NAME done (${JERRY_HOOK_DURATION_MS}ms)"
  on_step_failure:
    - run: echo "✗ $JERRY_HOOK_STEP_NAME failed: $JERRY_HOOK_ERROR"
```
