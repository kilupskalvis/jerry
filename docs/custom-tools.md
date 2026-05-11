# Custom Tools

Define project-specific tools as YAML files in `.jerry/tools/`. Jerry discovers them automatically and makes them available to any agent that declares them.

## Format

```yaml
# .jerry/tools/<name>.yaml
description: What this tool does (sent to the LLM)
parameters:
  param_name:
    type: string
    description: What this parameter is for
    required: true
run: |
  shell command here
```

Name is derived from the filename: `deploy.yaml` → tool name is `deploy`.

## Fields

| Field | Required | Description |
|-------|----------|-------------|
| `description` | yes | Explains to the LLM when and how to use this tool |
| `parameters` | no | Input parameters (see below) |
| `run` | yes | Shell command executed via `/bin/sh -c` |

## Parameters

Simplified format — Jerry converts to JSON Schema internally:

```yaml
parameters:
  service:
    type: string
    description: The service name
    required: true
  environment:
    type: string
```

Supported types: `string`, `integer`, `number`, `boolean`. Default type is `string` if omitted.

## How Arguments Are Passed

When the LLM calls your tool:

1. **Environment variables** — each parameter as `TOOL_<UPPER_NAME>`. Hyphens become underscores.
   - `service` → `$TOOL_SERVICE`
   - `channel-name` → `$TOOL_CHANNEL_NAME`

2. **Stdin** — full input JSON piped to stdin (for tools that want to parse JSON directly).

3. **Secrets** — `JERRY_SECRET_*` env vars available (from CI secrets or `.env`).

4. **Working directory** — repository root.

## Examples

### No parameters

```yaml
# .jerry/tools/run-tests.yaml
description: Run the project test suite
run: go test ./...
```

### One parameter

```yaml
# .jerry/tools/deploy.yaml
description: Deploy the current branch
parameters:
  service:
    type: string
    required: true
run: |
  curl -X POST "https://deploy.internal/api/v1/deploy" \
    -H "Authorization: Bearer $JERRY_SECRET_DEPLOY_TOKEN" \
    -d "{\"service\": \"$TOOL_SERVICE\"}"
```

### Multiple parameters

```yaml
# .jerry/tools/create-ticket.yaml
description: Create a Jira ticket
parameters:
  summary:
    type: string
    required: true
  description:
    type: string
run: |
  curl -X POST "https://jira.internal/rest/api/2/issue" \
    -H "Authorization: Bearer $JERRY_SECRET_JIRA_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"fields\":{\"project\":{\"key\":\"$JERRY_SECRET_JIRA_PROJECT\"},\"summary\":\"$TOOL_SUMMARY\",\"description\":\"$TOOL_DESCRIPTION\",\"issuetype\":{\"name\":\"Task\"}}}"
```

### Reading JSON from stdin

```yaml
# .jerry/tools/webhook.yaml
description: Send a webhook with arbitrary payload
parameters:
  url:
    type: string
    required: true
run: |
  curl -X POST "$TOOL_URL" \
    -H "Content-Type: application/json" \
    -d @-
```

The `@-` reads the full input JSON from stdin.

## Using Custom Tools

Agent declares by name:

```yaml
---
name: deployer
model: claude-sonnet-4-6
tools:
  - deploy
  - create-ticket
---
```

Jerry resolves `deploy` → `.jerry/tools/deploy.yaml`. If not found → error at validation time.

## Error Handling

- Non-zero exit code → error result sent to LLM (agent can adapt, workflow continues)
- Stdout + stderr captured together as the tool result
- Timeout inherited from the step timeout
- Missing `description` or `run` → error at workflow load time
