# jerry exec

Run a single workflow step. The per-step entry point that compiled CI invokes.

## Usage

```
jerry exec <workflow>/<step> [flags]
```

## Description

Executes one step of a workflow and exits with that step's exact status code. The compiled GitHub Actions workflow calls `jerry exec` once per step — CI owns sequencing, `exec` owns the single-step lifecycle:

1. Load and validate the spec
2. Resolve the trigger (from `$GITHUB_EVENT_PATH`, flags, or context dir)
3. Build the run context from prior step records in `.jerry-run/`
4. Resolve `${{ }}` templates in the prompt or ci fields
5. Dispatch: agent → runtime adapter, shell → `/bin/sh`, ci → GitHub API or preview
6. Capture results: output text, structured outputs, workspace diff, usage
7. Enforce budget caps
8. Write step record to `.jerry-run/steps/<name>/`

## Arguments

| Argument | Required | Description |
|---|---|---|
| `<workflow>/<step>` | yes | Workflow name and step name separated by `/` |

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--trigger-file` | string | `""` | Path to a CI event payload JSON file |
| `--trigger` | string[] | `[]` | Trigger field as `key=value` (repeatable) |
| `--intent` | string | `""` | Manual trigger intent |
| `--ctx-dir` | string | `.jerry-run` | Context directory path |
| `--ci-live` | bool | false | Execute ci: steps for real instead of preview |

## CI environment detection

When `$GITHUB_ACTIONS=true` is set (standard in GitHub Actions runners), ci: steps automatically run in live mode without the `--ci-live` flag. The compiler does not emit `--ci-live` — it's the default in CI.

## Exit codes

| Code | Meaning | Retryable |
|---|---|---|
| 0 | Step succeeded | — |
| 1 | Step failed (agent error, schema mismatch, shell non-zero) | yes |
| 2 | Configuration error (spec invalid, drift, missing env/refs, version mismatch) | no |
| 3 | Runtime error (adapter spawn failure, runtime crash, unparseable output) | yes |
| 4 | Budget exceeded | no |

Generated retry loops and `jerry run` retry only on codes 1 and 3. Codes 2 and 4 are terminal.

## Trigger resolution

Priority order:

1. `--trigger-file` flag
2. `$GITHUB_EVENT_PATH` environment variable (set by GitHub Actions)
3. `--trigger key=value` flags
4. Manual trigger with `--intent`

The first step of a run writes `trigger.json` to the context dir. Later steps reuse it.

## Environment contract

Agent and shell steps receive a strict allowlist environment:

| Variable | Description |
|---|---|
| `JERRY_CTX_DIR` | Path to the context directory |
| `JERRY_RUN_ID` | Run identifier |
| `JERRY_STEP_NAME` | Current step name |
| `JERRY_INTENT` | Trigger intent string |
| `JERRY_STEP_<NAME>_OUTPUT_FILE` | Path to a prior step's output.txt |
| `PATH`, `HOME` | Standard system paths |
| Declared `env:` names | From workflow/step env with values from process env |

No ambient secrets leak. The runtime and shell never inherit the runner's full environment.

## Example

```bash
# Simulate what CI does: two sequential exec calls sharing .jerry-run
jerry exec review/review --intent "review my PR"
jerry exec review/report
```
