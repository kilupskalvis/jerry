# jerry run

Execute a workflow locally.

## Usage

```
jerry run <workflow> [intent] [flags]
```

## Description

Runs every step of a workflow sequentially on the local machine. This is a for-loop around `jerry exec` — not an engine. It uses the same step semantics as compiled CI: same timeouts, same retry logic, same exit-code rules.

CI action steps (`post_pr_comment`, `add_check_status`, etc.) run in **preview mode** by default: they print the fully resolved payload instead of calling the API. This lets you rehearse the entire pipeline locally — including what would be posted — before anything runs in CI.

## Arguments

| Argument | Required | Description |
|---|---|---|
| `<workflow>` | yes | Workflow name (directory name under `.jerry/`) |
| `[intent]` | no | Manual trigger intent string. Default: `"manual local run"` |

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--keep-ctx` | bool | false | Keep the `.jerry-run/` context directory after the run for inspection |
| `--ci-live` | bool | false | Execute ci: steps for real (calls the GitHub API) |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | All steps succeeded |
| 1 | A step failed |

The underlying `jerry exec` exit codes (2 = config, 3 = runtime, 4 = budget) are surfaced in the error message.

## Retry behavior

Steps with `retries: N` are retried up to N additional times. Only exit codes 1 (step failure) and 3 (runtime error) trigger retries. Exit codes 2 (config error) and 4 (budget exceeded) are terminal — retrying them is pointless or wasteful.

## Example

```bash
$ jerry run review "check the auth module"
▸ review (pi)
✓ review
▸ report — ci preview: post_pr_comment
  body:
    ## Jerry Review
    No critical findings.
✓ report (preview)
▸ gate — ci preview: add_check_status
  status:
    success
✓ gate (preview)
workflow "review" completed
```

## Inspecting the context directory

With `--keep-ctx`, the `.jerry-run/` directory is preserved:

```
.jerry-run/
  trigger.json             # normalized trigger data
  ledger.json              # usage: per-step + run totals
  steps/
    review/
      output.txt           # agent's text output
      outputs.json          # structured outputs
      diff.patch            # workspace changes
      usage.json            # tokens + cost
```

These files are the handoff contract between steps. Shell steps can read them via `$JERRY_STEP_<NAME>_OUTPUT_FILE` environment variables.
