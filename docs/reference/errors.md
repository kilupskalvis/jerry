# Error Reference

Jerry uses structured error codes with three exit code categories. Every error message includes the error code, a human-readable description, and (when applicable) the step that failed.

## Exit Codes

| Code | Category | Description |
|------|----------|-------------|
| 0 | Success | Workflow completed successfully |
| 1 | Step failure | A step failed during execution |
| 2 | Configuration | Invalid YAML, missing files, bad config |
| 3 | Runtime | LLM errors, state write failures, external service issues |

---

## Configuration Errors (Exit 2)

These errors indicate problems with your `.jerry/` setup. Fix the configuration and retry.

| Code | Cause | Fix |
|------|-------|-----|
| `INVALID_WORKFLOW` | `workflow.yaml` has invalid YAML syntax, missing `steps`, or a step with neither `agent` nor `run` | Run `jerry validate` to see the specific issue. Check YAML syntax. |
| `WORKFLOW_NOT_FOUND` | The workflow name passed to `jerry run` doesn't match any directory in `.jerry/` | Check `.jerry/` for available workflows. Run `jerry validate` to list them. |
| `JERRY_DIR_NOT_FOUND` | No `.jerry/` directory found in the current directory or any parent | Run `jerry init` to create one, or `cd` to the correct project root. |
| `JERRY_DIR_EXISTS` | `jerry init` was run but `.jerry/` already exists | Use `jerry init --template <name>` to add a workflow, not re-initialize. |
| `CONFIG_INVALID` | `settings.yaml` or `settings.local.yaml` has invalid syntax | Check YAML syntax in the settings file. |
| `AGENT_LOAD_FAILED` | Agent `.md` file can't be parsed: missing frontmatter, missing `name`, empty instructions, bad YAML | Run `jerry validate` — it shows exactly what's wrong with "did you mean?" suggestions. |
| `TOOL_NOT_FOUND` | An agent's `tools:` list references a tool that doesn't exist as a built-in, custom tool, or agent file | Run `jerry validate` — it lists available tools. Check `.jerry/tools/` and the workflow directory. |
| `RUN_NOT_FOUND` | The run ID passed to `--resume` doesn't exist in `.jerry/runs/` | Check `jerry logs` for valid run IDs. |
| `RUN_NOT_RESUMABLE` | The run is already completed, or has status `running` without `--force` | Completed runs can't be resumed. For `running` status (crashed process), use `--force`. |
| `WORKFLOW_CHANGED` | The workflow structure changed since the saved run — steps were added, removed, or reordered | Can't safely resume. Start a new run instead. |

## Step Failure Errors (Exit 1)

These errors occur during workflow execution. The workflow aborts at the failed step.

| Code | Cause | Fix |
|------|-------|-----|
| `SCRIPT_FAILED` | A script step (`run:`) returned a non-zero exit code | Check the command output in the logs. Fix the script or the code it runs. |
| `SCRIPT_TIMEOUT` | A script step exceeded its configured `timeout` | Increase the `timeout` value or optimize the script. |
| `AGENT_MAX_ITERATIONS` | An agent hit its `max_iterations` limit without producing a final response | Increase `max_iterations` in the agent frontmatter, or improve the agent's instructions to be more focused. |
| `TOOL_CONSTRAINT_VIOLATION` | A tool call violated a permission rule | Check `settings.yaml` and agent `permissions`. The error message shows which pattern blocked the call. |
| `COMPACTION_FAILED` | Context compaction (summarizing older messages to fit the context window) failed after 3 attempts | The conversation may be too complex. Try reducing `max_iterations` or splitting the task across multiple agents. |
| `CONTEXT_TOO_LONG` | The conversation exceeded the model's context window even after compaction | Use a model with a larger context window, or break the task into smaller steps. |

## Runtime Errors (Exit 3)

These errors indicate external system failures. Usually transient — retry may succeed.

| Code | Cause | Fix |
|------|-------|-----|
| `LLM_AUTH_FAILED` | API key is invalid or missing | Check that `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` is set correctly. |
| `LLM_RATE_LIMITED` | The LLM provider rate-limited the request (after SDK-level retries exhausted) | Wait and retry. Consider reducing parallel usage or requesting higher rate limits. |
| `LLM_SERVER_ERROR` | The LLM provider returned a 500/502/503/504 error | Transient. Retry the workflow. If persistent, check the provider's status page. |
| `LLM_CALL_FAILED` | Any other LLM API error (400 Bad Request, unexpected response) | Check the error message. May be a model name typo or unsupported parameter. |
| `STATE_WRITE_FAILED` | Failed to write run state to `.jerry/runs/` | Check disk space and directory permissions. |
| `CONTEXT_WRITE_DENIED` | Failed to write context data | Check disk space and directory permissions. |
| `GIT_NOT_AVAILABLE` | A git tool was invoked but `git` is not installed or not in PATH | Install git or ensure it's on the CI runner's PATH. |

---

## Error Message Format

```
jerry: error: ERROR_CODE: description: cause
```

With step context:

```
jerry: error: AGENT_LOAD_FAILED [step: reviewer]: agent "/path/to/reviewer.md": missing required field 'name'
```

## Handling Errors in CI

In CI pipelines, use the exit code to control the workflow:

```yaml
# GitHub Actions
- run: jerry run review --trigger-file "$GITHUB_EVENT_PATH"
  continue-on-error: true  # optional: don't fail the CI job on Jerry errors
```

Use [hooks](../configuration/hooks.md) to get notified on failure regardless of exit code:

```yaml
hooks:
  on_workflow_failure:
    - run: curl -X POST $JERRY_SECRET_WEBHOOK -d '{"text":"Jerry failed: $JERRY_HOOK_ERROR"}'
```
