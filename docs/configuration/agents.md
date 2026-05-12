# Agents

Agents are Markdown files with YAML frontmatter. The frontmatter configures runtime behavior — model, tools, permissions. The body is the agent's system prompt.

## Quick Example

```markdown
---
name: reviewer
model: claude-sonnet-4-6
tools:
  - post_pr_comment
---

Review the PR for bugs and security issues. Post findings as a comment.
```

## Frontmatter Reference

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | yes | — | Unique identifier for this agent |
| `model` | string | no | `$JERRY_DEFAULT_MODEL` | LLM model (e.g., `claude-sonnet-4-6`, `gpt-4o`) |
| `provider` | string | no | auto-detected | LLM provider: `anthropic` or `openai` |
| `temperature` | float | no | `0` | Sampling temperature (0.0–2.0) |
| `max_iterations` | int | no | `50` | Maximum tool-use cycles before stopping |
| `tools` | list | no | `[]` | Additional tools beyond the always-on set |
| `permissions` | object | no | — | Per-agent deny/allow rules (see [Permissions](permissions.md)) |
| `secrets` | list | no | `[]` | Required environment variables (validated at load time) |

## Model Selection

The provider is auto-detected from the model name:

| Prefix | Provider |
|--------|----------|
| `claude-*` | Anthropic |
| `gpt-*`, `o1-*`, `o3-*`, `o4-*` | OpenAI |

For custom or fine-tuned models, set `provider` explicitly:

```yaml
provider: openai
model: ft:gpt-4o:my-org:custom-model
```

Set `JERRY_DEFAULT_MODEL` as a fallback when agents don't specify a model.

## Tools

### Always-on tools

Every agent gets these automatically. Do not list them in `tools:`.

| Tool | Description |
|------|-------------|
| `bash` | Run a shell command. Returns stdout + stderr. |
| `read_file` | Read a file with line numbers. Supports `offset` and `limit` for partial reads (default: first 200 lines). |
| `write_file` | Write or create a file. Creates parent directories. |

### CI tools

Declare in `tools:` to use. Require `GITHUB_TOKEN` or `GITLAB_TOKEN`.

| Tool | Description |
|------|-------------|
| `post_pr_comment` | Post a comment on the triggering PR/MR |
| `post_review_comment` | Post an inline comment on a specific file and line |
| `add_check_status` | Report a check result (success/failure) |
| `create_pull_request` | Create a branch, commit changes, open a PR/MR |

### Custom tools

YAML files in `.jerry/tools/`. See [Tools](tools.md).

### Subagent tools

Other agent `.md` files in the same workflow directory. When you list another agent's name in `tools:`, the parent agent can invoke it as a tool at runtime.

```yaml
# reviewer.md — can delegate to security_reviewer
tools:
  - post_pr_comment
  - security_reviewer
```

See [Tools](tools.md#subagent-tools) for details.

## Permissions

Per-agent deny/allow rules that restrict tool behavior. Merged with project-level rules from `settings.yaml`. See [Permissions](permissions.md).

```yaml
name: reviewer
model: claude-sonnet-4-6
permissions:
  deny:
    - write_file: ["**"]
    - bash: ["git push *", "git commit *"]
```

## Writing Instructions

The Markdown body is the agent's system prompt. Write it like you're briefing a senior engineer.

**Do:**
- State the role: "You are a security auditor reviewing this PR."
- Describe the process: "1. Read the diff. 2. Check for vulnerabilities. 3. Post findings."
- Set constraints: "Do not modify files. Only report issues."

**Don't:**
- Repeat tool descriptions (the LLM already sees them in the tool definitions).
- Include project-specific file paths (discover them at runtime instead).
- Write multi-page instructions (shorter = more focused).

## Context

Agents in a multi-step workflow see previous step outputs prepended to their system prompt:

```
## Trigger
Type: pull_request
Source: github
Intent: Fix auth timeout

## Previous Steps
### planner
Plan: add /search endpoint with query parameter...

---

<your agent's instructions>
```

The trigger and previous outputs are prepended automatically. Your instructions go after the separator.

## Examples

### Minimal agent

```markdown
---
name: reviewer
model: claude-sonnet-4-6
---

Review the code for bugs and style issues.
```

### Read-only reviewer with permissions

```markdown
---
name: reviewer
model: claude-sonnet-4-6
tools:
  - post_pr_comment
permissions:
  deny:
    - write_file: ["**"]
    - bash: ["git push *", "git commit *"]
---

You are a senior engineer reviewing a pull request.

## Process

1. Read the PR diff using `bash` with `git diff`.
2. Read any files that need closer inspection.
3. Post your findings as a PR comment.

## Focus Areas

- Logic errors and edge cases
- Security vulnerabilities
- Missing error handling
- Test coverage gaps
```

### Triage agent with subagents

```markdown
---
name: triage
model: claude-sonnet-4-6
tools:
  - post_pr_comment
  - security_reviewer
  - performance_reviewer
---

You are a triage agent. Read the PR diff and delegate:
- If changes touch authentication or authorization, use `security_reviewer`.
- If changes touch database queries or hot paths, use `performance_reviewer`.
- Review everything else yourself.

Combine all findings into a single PR comment.
```

### Code generator with restricted tools

```markdown
---
name: generator
model: claude-sonnet-4-6
tools:
  - create_pull_request
permissions:
  allow:
    - write_file: ["src/**", "tests/**"]
    - bash: ["go *", "npm *", "git add *", "git commit *"]
---

Implement the plan from the previous step.
Read existing code, match conventions, build, test, open a PR.
```
