# Triggers

Triggers tell Jerry what initiated the workflow — a PR, a ticket, a push, or a manual command. Jerry normalizes trigger data from any source into a standard format that agents consume.

## Input Methods

### 1. Trigger File (GitHub Actions)

```bash
jerry run review --trigger-file "$GITHUB_EVENT_PATH"
```

Reads a JSON file and normalizes it. Jerry auto-detects GitHub and GitLab webhook formats. Supports:
- GitHub: issues, pull_request, push, repository_dispatch
- GitLab: issue, merge_request, push
- Pre-normalized: any JSON with `type` and `source` fields

### 2. Trigger Flags (GitLab CI, any platform)

```bash
jerry run review \
  --trigger type=pull_request \
  --trigger source=gitlab \
  --trigger intent="Fix auth timeout" \
  --trigger number=42 \
  --trigger head_sha=abc123
```

Set trigger fields directly. Repeatable flag. Works with any CI platform by mapping platform-specific variables.

### 3. Manual (local development)

```bash
jerry run review "Check for common issues"
```

Positional argument becomes the intent. Type defaults to `manual`, source to `cli`.

### 4. Stdin

```bash
echo '{"type":"ticket","source":"jira","intent":"Add search"}' | jerry run feature --trigger-stdin
```

Reads JSON from stdin. Same normalization as `--trigger-file`.

## Trigger Fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `pull_request`, `ticket`, `push`, `manual`, `webhook` |
| `source` | string | `github`, `gitlab`, `jira`, `linear`, `cli` |
| `intent` | string | Human-readable description (PR title, ticket summary) |
| `number` | int | PR/MR/issue number |
| `head_sha` | string | Commit SHA |
| `repo_owner` | string | Repository owner or namespace |
| `repo_name` | string | Repository name |
| `author` | string | User who triggered the event |
| `url` | string | Link to the PR/MR/issue |
| `raw_payload` | object | Full original webhook JSON (only via `--trigger-file` / `--trigger-stdin`) |

## How Agents See Triggers

Trigger data appears in the agent's system prompt:

```
## Trigger

Type: pull_request
Source: github
Intent: Fix null pointer in auth handler
URL: https://github.com/org/repo/pull/42
Author: alice

---

<agent instructions>
```

Empty fields are omitted.

## Pre-Normalized Triggers

Any JSON with `type` and `source` at the top level is used as-is — no platform-specific normalization. This lets any system feed Jerry directly:

```json
{
  "type": "ticket",
  "source": "jira",
  "intent": "Add dark mode support",
  "raw_payload": {
    "key": "PROJ-123",
    "description": "Users want a dark theme"
  }
}
```

GitHub's `repository_dispatch` uses this pattern — Jira sends pre-normalized data inside `client_payload`.

## Mutual Exclusivity

`--trigger-file`, `--trigger-stdin`, `--trigger key=value`, and the positional intent argument are mutually exclusive. Using more than one is an error.
