# Tools

Tools are capabilities that agents can use during execution. Jerry provides built-in tools, supports custom YAML tools, and lets agents invoke other agents as subagent tools.

## Built-in Tools

### Always-on

Every agent gets these automatically. Do not declare them in `tools:`.

#### bash

Run a shell command. Returns combined stdout and stderr.

```json
{"command": "go test ./..."}
```

- Working directory: repository root
- Clean environment: only `PATH`, `HOME`, and `JERRY_SECRET_*`
- Timeout: 120 seconds
- Process env (including API keys) is not leaked

#### read_file

Read a file with line numbers. Supports partial reads for large files.

```json
{"path": "main.go"}
{"path": "main.go", "offset": 50, "limit": 30}
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `path` | string | required | Path relative to repository root |
| `offset` | int | `1` | Line number to start reading from (1-based) |
| `limit` | int | `200` | Maximum number of lines to read |

The agent sees the tool schema and uses `offset`/`limit` naturally when working with large files.

#### write_file

Write content to a file. Creates parent directories automatically.

```json
{"path": "src/handler.go", "content": "package main\n..."}
```

### CI Tools

Declare in `tools:` to use. Require `GITHUB_TOKEN` (GitHub) or `GITLAB_TOKEN` (GitLab).

| Tool | Parameters | Description |
|------|-----------|-------------|
| `post_pr_comment` | `body` | Post a comment on the triggering PR/MR |
| `post_review_comment` | `path`, `line`, `body` | Post an inline review comment |
| `add_check_status` | `name`, `status`, `summary` | Report a check result |
| `create_pull_request` | `title`, `body`, `branch` | Create a branch, commit, open a PR |

```yaml
tools:
  - post_pr_comment
  - create_pull_request
```

CI tools use the trigger's data to determine the target repository and PR number. They require a CI trigger (not a manual CLI trigger) to function.

## Custom Tools

Define project-specific tools as YAML files in `.jerry/tools/`. Jerry discovers them automatically.

### Format

```yaml
# .jerry/tools/deploy.yaml
description: Deploy a service to the specified environment
parameters:
  service:
    type: string
    description: Service name
    required: true
  environment:
    type: string
    description: Target environment
run: |
  curl -X POST "https://deploy.internal/api/v1/deploy" \
    -H "Authorization: Bearer $JERRY_SECRET_DEPLOY_TOKEN" \
    -d "{\"service\": \"$TOOL_SERVICE\", \"env\": \"$TOOL_ENVIRONMENT\"}"
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `description` | yes | Explains to the LLM when and how to use this tool |
| `parameters` | no | Input parameters (see below) |
| `run` | yes | Shell command executed via `/bin/sh -c` |

### Parameters

| Property | Description |
|----------|-------------|
| `type` | `string`, `integer`, `number`, or `boolean`. Default: `string` |
| `description` | Explains the parameter to the LLM |
| `required` | If `true`, the LLM must provide this parameter |

### How Arguments Are Passed

1. **Environment variables** — each parameter as `TOOL_<UPPER_NAME>`. Hyphens become underscores.
2. **Stdin** — full input JSON piped to stdin.
3. **Secrets** — `JERRY_SECRET_*` env vars available.
4. **Working directory** — repository root.

### Using Custom Tools

Declare by name in the agent's frontmatter:

```yaml
tools:
  - deploy
  - create_ticket
```

Jerry resolves `deploy` → `.jerry/tools/deploy.yaml`.

## Subagent Tools

Any agent `.md` file in the same workflow directory can be used as a tool by another agent. This enables dynamic delegation — a triage agent decides at runtime which specialist to invoke.

### How It Works

Place agent files in the workflow directory:

```
.jerry/review/
  workflow.yaml
  reviewer.md             # main agent
  security_reviewer.md    # subagent
  performance_reviewer.md # subagent
```

Declare the subagent in the parent's `tools:`:

```yaml
# reviewer.md
---
name: reviewer
model: claude-sonnet-4-6
tools:
  - post_pr_comment
  - security_reviewer
  - performance_reviewer
---

Triage the PR. If it touches auth, delegate to security_reviewer.
If it touches hot paths, delegate to performance_reviewer.
```

The parent calls the subagent like any tool:

```json
{"name": "security_reviewer", "input": {"task": "Check token validation in auth.go"}}
```

### Subagent Behavior

- The subagent uses its **own** model, tools, and permissions from its `.md` frontmatter.
- The subagent receives the **trigger data** (PR info, ticket data) in its system prompt.
- The subagent's **task** comes from the parent's tool call — what to focus on.
- The subagent's text output is returned to the parent as the tool result.
- Subagents are **one level deep** — a subagent cannot invoke other subagents.
- Multiple subagent calls run **in parallel** (since all tool calls execute concurrently).

### Subagent vs Workflow Step

The same `.md` file can be used both ways:

```yaml
# As a workflow step (always runs):
steps:
  - agent: security_reviewer

# As a subagent tool (parent decides when to invoke):
# reviewer.md → tools: [security_reviewer]
```

Workflow steps are static — every run executes them. Subagent tools are dynamic — the parent agent decides at runtime whether to invoke them.

### Verbose Output

Subagent activity is indented in verbose output:

```
    -> security_reviewer({"task":"Check auth changes"})
      ▸ security_reviewer ...
        -> read_file({"path":"auth.go"})
        <- read_file (45 lines):
           1: package auth
           ...
      ✓ security_reviewer (4.2s)
    <- security_reviewer (12 lines):
       ## Findings
       ...
```

## Tool Resolution Order

When an agent declares a tool name, Jerry resolves it in this order:

1. **Built-in base tools** — `bash`, `read_file`, `write_file` (silently skipped, always available)
2. **Built-in CI tools** — `post_pr_comment`, etc.
3. **Custom tools** — `.jerry/tools/<name>.yaml`
4. **Agent tools** — `<workflow_dir>/<name>.md`

If the name doesn't match any of these, `jerry validate` reports an error.
