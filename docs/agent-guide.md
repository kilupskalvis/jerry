# Agent Guide

Agents are Markdown files with YAML frontmatter. The frontmatter configures runtime behavior. The body is the agent's instructions (system prompt).

## Minimal Agent

```markdown
---
name: reviewer
model: claude-sonnet-4-6
---

Review the code for bugs and security issues.
```

That's it. This agent has `bash`, `read_file`, and `write_file` automatically.

## Frontmatter Fields

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | yes | — | Unique identifier |
| `model` | no | `$JERRY_DEFAULT_MODEL` | LLM model (`claude-sonnet-4-6`, `gpt-4o`, etc.) |
| `temperature` | no | `0` | Sampling temperature (0.0–2.0) |
| `max_iterations` | no | `50` | Maximum tool-use cycles before stopping |
| `tools` | no | `[]` | Additional tools beyond the always-on set |

## Tools

### Always-On (every agent gets these)

| Tool | Description |
|------|-------------|
| `bash` | Run any shell command. Stdout + stderr captured. Clean env (secrets via `JERRY_SECRET_*`). |
| `read_file` | Read file with line numbers. Path relative to repo root. |
| `write_file` | Write or create a file. Creates parent directories. |

### CI Tools (declare in `tools:` to use)

| Tool | Description |
|------|-------------|
| `post_pr_comment` | Post a comment on the triggering PR/MR |
| `post_review_comment` | Post an inline comment on a specific file and line |
| `add_check_status` | Report a check result (success/failure) |
| `create_pull_request` | Create a branch, commit changes, open a PR/MR |

CI tools require `GITHUB_TOKEN` (GitHub) or `GITLAB_TOKEN` (GitLab).

### Custom Tools

See [Custom Tools](custom-tools.md).

## Writing Good Instructions

The body of the Markdown file is the agent's system prompt. Write it like you're briefing a senior engineer:

**Do:**
- State the role: "You are a security auditor reviewing this PR."
- Describe the process: "1. Read the diff. 2. Check for injection vulnerabilities. 3. Post findings."
- Set constraints: "Do not modify files. Only report issues."

**Don't:**
- Repeat tool descriptions (the LLM already sees them)
- Include project-specific file paths (read them at runtime instead)
- Write multi-page instructions (shorter = more focused agent)

## Context Flow

Agents in a multi-step workflow see previous step outputs in their system prompt:

```
## Trigger
Type: ticket
Source: jira
Intent: Add search endpoint

## Previous Steps
### planner
Plan: add /search endpoint with query parameter...

---
<your agent's instructions>
```

The trigger and previous outputs are prepended automatically. Your instructions go after the separator.

## Model Selection

Provider auto-detected from model name:

| Prefix | Provider |
|--------|----------|
| `claude-*` | Anthropic |
| `gpt-*`, `o1-*`, `o3-*`, `o4-*` | OpenAI |

For custom/fine-tuned models, set `provider` in frontmatter:

```yaml
provider: openai
model: ft:gpt-4o:my-org:custom-model
```

Or set `JERRY_DEFAULT_MODEL` for the fallback when agents don't specify a model.
