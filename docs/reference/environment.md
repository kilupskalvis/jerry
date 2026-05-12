# Environment Variables

Jerry uses environment variables for API keys, secrets, and configuration. Variables can be set in the shell environment, in CI platform secrets, or in a `.env` file at the repository root.

## Precedence

1. Shell environment (highest)
2. `.env` file at repository root (loaded automatically)

If a variable is set in both, the shell environment wins.

---

## LLM Provider Keys

| Variable | Required for | Description |
|----------|-------------|-------------|
| `ANTHROPIC_API_KEY` | Claude models (`claude-*`) | Anthropic API key. Required when any agent uses a Claude model. |
| `OPENAI_API_KEY` | GPT/O-series models (`gpt-*`, `o1-*`, `o3-*`, `o4-*`) | OpenAI API key. Required when any agent uses an OpenAI model. |

Only the key for the provider your agents use needs to be set. If all agents use Claude, `OPENAI_API_KEY` is not needed.

## Jerry Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `JERRY_DEFAULT_MODEL` | — | Fallback model when an agent's frontmatter doesn't specify `model`. If unset and the agent has no model, validation fails. |

## Git Platform Tokens

| Variable | Required for | Description |
|----------|-------------|-------------|
| `GITHUB_TOKEN` | GitHub CI tools | Token for GitHub API. Auto-provided in GitHub Actions as `${{ secrets.GITHUB_TOKEN }}`. Required by `post_pr_comment`, `post_review_comment`, `add_check_status`, `create_pull_request`. |
| `GITLAB_TOKEN` | GitLab CI tools | Token for GitLab API. Set as a CI variable. Required by output routing tools on GitLab. |

## Secrets

| Variable | Description |
|----------|-------------|
| `JERRY_SECRET_*` | Any variable prefixed with `JERRY_SECRET_` is treated as a secret. Secrets are available to: script steps, custom tools (as environment variables), hooks (as environment variables). Secrets are NOT available to agent LLM calls or the `bash` tool's process environment beyond the `JERRY_SECRET_*` namespace. |

Example:

```bash
# .env file
JERRY_SECRET_SLACK_WEBHOOK=https://hooks.slack.com/services/T.../B.../xxx
JERRY_SECRET_DEPLOY_TOKEN=sk-deploy-abc123
```

Usage in a hook:

```yaml
hooks:
  on_workflow_complete:
    - run: curl -X POST $JERRY_SECRET_SLACK_WEBHOOK -d '{"text":"Done"}'
```

Usage in a custom tool:

```yaml
# .jerry/tools/deploy.yaml
run: curl -H "Authorization: Bearer $JERRY_SECRET_DEPLOY_TOKEN" https://deploy.internal/api
```

---

## Script Step Variables

These are injected into the environment of script steps (`run:` in workflow.yaml):

| Variable | Description | Example |
|----------|-------------|---------|
| `JERRY_RUN_ID` | Unique identifier for the current run | `run_a1b2c3d4e5f6` |
| `JERRY_INTENT` | Trigger intent description | `Fix auth timeout` |
| `JERRY_STEP_NAME` | Name of the current step | `test` |
| `JERRY_CONTEXT_FILE` | Path to a temporary JSON file containing previous step outputs and trigger data | `/tmp/jerry-ctx-abc123.json` |

The context file structure:

```json
{
  "protocol_version": "1.0",
  "run_id": "run_a1b2c3d4",
  "trigger": {
    "type": "pull_request",
    "source": "github",
    "intent": "Fix auth timeout"
  },
  "steps": [
    {"name": "planner", "output": "Plan: add timeout parameter..."},
    {"name": "generator", "output": "Implemented the changes..."}
  ]
}
```

---

## Hook Variables

These are injected into the environment of lifecycle hooks. See [Hooks](../configuration/hooks.md) for the complete reference.

### Base (all hooks)

| Variable | Description |
|----------|-------------|
| `JERRY_HOOK_EVENT` | Event name (e.g., `on_workflow_complete`) |
| `JERRY_HOOK_WORKFLOW` | Workflow name |
| `JERRY_HOOK_RUN_ID` | Current run ID |

### Workflow events

| Variable | Available on | Description |
|----------|-------------|-------------|
| `JERRY_HOOK_STATUS` | `on_workflow_complete`, `on_workflow_failure` | `completed` or `failed` |
| `JERRY_HOOK_DURATION_MS` | `on_workflow_complete`, `on_workflow_failure` | Duration in milliseconds |
| `JERRY_HOOK_TRIGGER_TYPE` | all workflow events | Trigger type |
| `JERRY_HOOK_TRIGGER_INTENT` | all workflow events | Trigger intent |
| `JERRY_HOOK_ERROR` | `on_workflow_failure` | Error message |
| `JERRY_HOOK_FAILED_STEP` | `on_workflow_failure` | Step that failed |

### Step events

| Variable | Available on | Description |
|----------|-------------|-------------|
| `JERRY_HOOK_STEP_NAME` | all step events | Step name |
| `JERRY_HOOK_STEP_STATUS` | `on_step_complete`, `on_step_failure` | `success` or `failed` |
| `JERRY_HOOK_DURATION_MS` | `on_step_complete`, `on_step_failure` | Duration in milliseconds |
| `JERRY_HOOK_ERROR` | `on_step_failure` | Error message |

### Tool events

| Variable | Available on | Description |
|----------|-------------|-------------|
| `JERRY_HOOK_STEP_NAME` | `before_tool_call`, `after_tool_call` | Current step |
| `JERRY_HOOK_TOOL_NAME` | `before_tool_call`, `after_tool_call` | Tool name |
| `JERRY_HOOK_TOOL_INPUT` | `before_tool_call`, `after_tool_call` | Tool input JSON |
| `JERRY_HOOK_TOOL_OUTPUT` | `after_tool_call` | Tool result string |
| `JERRY_HOOK_TOOL_IS_ERROR` | `after_tool_call` | `true` or `false` |

---

## Custom Tool Variables

When a custom tool is invoked, its parameters are available as environment variables with the `TOOL_` prefix:

| Parameter name | Environment variable |
|---------------|---------------------|
| `service` | `TOOL_SERVICE` |
| `channel-name` | `TOOL_CHANNEL_NAME` |
| `count` | `TOOL_COUNT` |

Hyphens are converted to underscores. Names are uppercased.

The full input JSON is also piped to stdin, so tools can parse it directly with `jq` or `cat`.

---

## Bash Tool Environment

The `bash` tool runs commands in a **clean environment** to prevent secret leakage:

| Available | Not available |
|-----------|---------------|
| `PATH` | `ANTHROPIC_API_KEY` |
| `HOME` | `OPENAI_API_KEY` |
| `JERRY_SECRET_*` | Any other process env var |

This means agents cannot exfiltrate API keys or other sensitive environment variables via shell commands.
