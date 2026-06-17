# workflow.yaml

Every Jerry pipeline is a directory under `.jerry/` containing a `workflow.yaml` and its prompt files. This page documents every field.

## Minimal example

```yaml
version: 1
on:
  pull_request: { types: [opened, synchronize] }
steps:
  - name: review
    prompt: reviewer.md
```

## Complete example

```yaml
version: 1

on:
  pull_request: { types: [opened, synchronize] }
  push: { branches: [main] }
  dispatch: { types: [jerry-ticket] }
  schedule: { cron: "0 6 * * 1" }

defaults:
  runtime: pi
  model: claude-sonnet-4-6

env:
  - ANTHROPIC_API_KEY
  - GITHUB_TOKEN

steps:
  - name: review
    prompt: reviewer.md
    model: claude-haiku-4-5
    runtime: pi
    context: ["trigger"]
    permissions:
      allow: ["read", "bash(go test:*)"]
      deny: ["bash(rm:*)"]
    budget:
      max_cost: 1.50
      max_tokens: 500000
    outputs:
      verdict: string
      findings: string
    timeout: 600s
    retries: 1
    env: []

  - name: test
    run: go test ./...
    timeout: 300s

  - name: report
    ci: post_pr_comment
    body: "${{ steps.review.outputs.findings }}"
```

## Field reference

### Top-level

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `version` | int | yes | — | Spec version. Must be `1`. Jerry refuses to process unknown versions. |
| `on` | object | yes | — | Trigger configuration. At least one trigger must be set. |
| `defaults` | object | no | — | Default `runtime` and `model` for agent steps. |
| `env` | string[] | no | `[]` | Secret names available to steps. Values live in CI; Jerry never sees them. |
| `steps` | Step[] | yes | — | Ordered list of pipeline steps. |

### on (triggers)

| Field | Compiled to (GitHub) |
|---|---|
| `on.pull_request.types` | `on: pull_request: types: [...]` |
| `on.push.branches` | `on: push: branches: [...]` |
| `on.dispatch.types` | `on: repository_dispatch: types: [...]` |
| `on.schedule.cron` | `on: schedule: - cron: ...` |

### defaults

| Field | Type | Default | Description |
|---|---|---|---|
| `defaults.runtime` | string | `"pi"` | Runtime for agent steps that don't set their own. |
| `defaults.model` | string | `""` | Model for agent steps that don't set their own. |

### steps[]

Every step requires a `name` and exactly one of `prompt`, `run`, or `ci`.

| Field | Type | Applies to | Description |
|---|---|---|---|
| `name` | string | all | Unique, kebab-case (`[a-z0-9]+(-[a-z0-9]+)*`). |
| `prompt` | string | agent | Path to `.md` file (relative to workflow dir) or inline prompt string. |
| `run` | string | shell | POSIX shell command. Runs under `/bin/sh -c`. |
| `ci` | string | ci | One of: `post_pr_comment`, `post_review_comment`, `add_check_status`, `create_pull_request`. |
| `runtime` | string | agent | Overrides `defaults.runtime`. |
| `model` | string | agent | Overrides `defaults.model`. |
| `context` | string[] | agent | Explicit context refs. Omit for default (trigger + all prior outputs). |
| `outputs` | map | agent | Declared structured output schema. Keys map to types. |
| `permissions` | object | agent | `allow` and `deny` string arrays. |
| `budget` | object | agent | `max_cost` (USD float) and `max_tokens` (int). |
| `timeout` | duration | agent, shell | Go duration string (`600s`, `10m`). Compiled to CI timeout + 1 minute. |
| `retries` | int | agent, shell | Retry count. Compiled to a shell loop retrying exit codes 1 and 3 only. |
| `env` | string[] | all | Narrows workflow `env` for this step. `null` = inherit all, `[]` = no secrets. |

### ci step fields

These fields are valid only when `ci` is set:

| Field | Type | Description |
|---|---|---|
| `body` | string | Comment body or check summary. Supports `${{ }}` templates. |
| `status` | string | Check status. Must resolve to `"success"` or `"failure"`. |
| `title` | string | PR title (for `create_pull_request`). |

### outputs

Declared output keys with type constraints. The runtime must produce all declared keys; extra keys are ignored.

| Type | Go equivalent | Example value |
|---|---|---|
| `string` | `string` | `"approve"` |
| `number` | `float64` | `42.5` |
| `boolean` | `bool` | `true` |
| `list` | `[]any` | `["a", "b"]` |
| `object` | `map[string]any` | `{"key": "value"}` |

### permissions

```yaml
permissions:
  allow: ["read", "bash(go test:*)"]
  deny: ["bash(rm:*)", "write(.env)"]
```

Patterns use the `noun(selector)` grammar. Org-level deny rules from `settings.yaml` are merged and cannot be overridden.

**Known limitation:** pi translates `allow` nouns to tool names (`read→read`, `bash(...)→bash`, etc.) but cannot enforce `deny` patterns at the pi flag level. Deny rules are advisory for pi. See [pi adapter](../adapters/pi.md).

### budget

```yaml
budget:
  max_cost: 1.50
  max_tokens: 500000
```

Enforced post-step by `jerry exec`. Cost is whatever the runtime reports — Jerry keeps no price tables. Every attempt counts (retried steps accumulate). Breach = exit 4 (budget exceeded).
