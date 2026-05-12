# Agents

Agents are Markdown files with YAML frontmatter. The frontmatter configures runtime behavior — model, tools, permissions, iteration limits. The Markdown body is the agent's system prompt, sent to the LLM on every turn.

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

---

## Frontmatter Reference

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique identifier for this agent. Used in logs, validation errors, and as the default step name. |

### Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `$JERRY_DEFAULT_MODEL` | LLM model identifier. See [Model Selection](#model-selection). |
| `provider` | string | auto-detected | LLM provider: `anthropic` or `openai`. Required only for custom/fine-tuned models where the prefix doesn't match. |
| `temperature` | float | `0.0` | Sampling temperature. Range: 0.0 (deterministic) to 2.0 (creative). Lower values produce more consistent output. |
| `max_iterations` | int | `50` | Maximum number of tool-use cycles. Each cycle is: LLM responds → tools execute → results fed back. When reached, the agent stops with an error. |
| `tools` | list of strings | `[]` | Additional tools this agent can use, beyond the always-on set. See [Tools](tools.md). |
| `permissions` | object | — | Per-agent deny/allow rules. Merged with project-level `settings.yaml`. See [Permissions](permissions.md). |
| `secrets` | list of strings | `[]` | Environment variable names that must be set. Validated at agent load time — if any are missing, the agent fails to load. |

---

## Model Selection

### Auto-Detection

The LLM provider is detected from the model name prefix:

| Prefix | Provider | Examples |
|--------|----------|----------|
| `claude-*` | Anthropic | `claude-sonnet-4-6`, `claude-opus-4-6`, `claude-haiku-4-5` |
| `gpt-*` | OpenAI | `gpt-4o`, `gpt-4o-mini` |
| `o1-*`, `o3-*`, `o4-*` | OpenAI | `o1`, `o3`, `o4-mini` |

### Explicit Provider

For custom or fine-tuned models where the name doesn't match a known prefix, set `provider` explicitly:

```yaml
provider: openai
model: ft:gpt-4o:my-org:custom-model:abc123
```

### Default Model

If `model` is omitted, Jerry uses the `JERRY_DEFAULT_MODEL` environment variable. If neither is set, validation fails with:

```
agent "reviewer": no model specified and no default configured (set JERRY_DEFAULT_MODEL)
```

### Cost Considerations

Different models have different cost/capability tradeoffs:

| Model | Best for | Relative cost |
|-------|----------|---------------|
| `claude-haiku-4-5` | Simple tasks, classification, grep-and-report | $ |
| `claude-sonnet-4-6` | Code review, generation, most agent tasks | $$ |
| `claude-opus-4-6` | Complex reasoning, architecture decisions | $$$$ |

Jerry applies Anthropic prompt caching automatically — system prompts and tool definitions are cached across turns, reducing input token costs by up to 90% on turns 2+.

---

## Tools

Every agent automatically has three tools: `bash`, `read_file`, and `write_file`. Do not list these in `tools:` — they're always available.

To use additional tools, list them by name:

```yaml
tools:
  - post_pr_comment          # built-in CI tool
  - deploy                   # custom tool from .jerry/tools/deploy.yaml
  - security_reviewer        # subagent from security_reviewer.md in same directory
```

See [Tools](tools.md) for the complete reference on built-in, custom, and subagent tools.

---

## Permissions

Restrict what an agent's tools can do. Deny rules block matching tool calls. Allow rules restrict to only matching calls.

```yaml
permissions:
  deny:
    - write_file: ["**"]                      # block all writes
    - bash: ["git push *", "git commit *"]    # block git mutations
  allow:
    - bash: ["go test *", "go build *"]       # allow only go commands
```

Permissions are merged with project-level rules from `settings.yaml`. See [Permissions](permissions.md) for the full reference including glob patterns and the resolution chain.

---

## System Prompt

The Markdown body (everything after the closing `---`) becomes the agent's system prompt. This is sent to the LLM with every request, so it must be concise and directive.

### What the LLM Sees

On each turn, the LLM receives:

1. **System prompt** — trigger data + previous step outputs + your instructions (see below)
2. **Conversation history** — all previous LLM responses and tool results
3. **Tool definitions** — JSON schemas for available tools (generated from tool registrations)

### Context Assembly

If the agent is part of a multi-step workflow, Jerry prepends trigger data and previous step outputs to the system prompt:

```
## Trigger
Type: pull_request
Source: github
Intent: Fix null pointer in auth handler
URL: https://github.com/org/repo/pull/42
Author: alice

## Previous Steps

### planner
Plan: modify auth.go line 47 to add nil check before accessing token.Claims...

---

Review the PR for bugs and security issues. Post findings as a comment.
```

Empty trigger fields are omitted. If there's no trigger and no previous steps, the LLM sees only your instructions.

### Writing Effective Instructions

**Do:**
- State the role clearly: "You are a security auditor reviewing a pull request."
- Describe the process step by step: "1. Read the diff. 2. Check each changed file. 3. Post findings."
- Set constraints: "Do not modify files. Only report issues."
- Be specific about output format: "Report each finding on one line with file:line and severity."

**Don't:**
- Describe how tools work — the LLM already sees tool schemas.
- Hardcode file paths — discover them at runtime with `bash` or `read_file`.
- Write multi-page instructions — shorter prompts produce more focused agents.
- Include context that changes per run — that's what triggers and previous steps provide.

---

## Agent Loop Behavior

When Jerry executes an agent:

1. Send system prompt + "Begin your task." to the LLM
2. LLM responds with text, tool calls, or both
3. If tool calls: execute all tools in parallel → feed results back → go to step 2
4. If text only: agent is done — text becomes the step output
5. If `max_iterations` reached: agent stops with error

### Parallel Tool Execution

When the LLM returns multiple tool calls in one response, Jerry executes them all concurrently using goroutines. Results are returned to the LLM in the same order they were requested. This means an agent reading 5 files gets them all at once, not sequentially.

### Context Compaction

If the conversation grows too long for the model's context window, Jerry automatically compacts it by summarizing older messages. This happens transparently — the agent doesn't need to handle it. If compaction fails after 3 attempts, the step fails with `CONTEXT_TOO_LONG`.

### Guardrail Enforcement

Before each tool executes, Jerry checks it against the merged permissions (settings + agent). Blocked calls return an error message to the LLM:

```
Permission denied: "rm -rf /" blocked by guardrail.
Denied pattern: "rm -rf *" (source: settings.yaml)
```

The LLM sees this as a tool error and adapts — typically trying an alternative approach or skipping the action.

---

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
1. Run `git diff HEAD~1` to see the changes.
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

You are a triage agent. Read the PR diff and delegate to specialists:
- If changes touch authentication or authorization, use `security_reviewer`.
- If changes touch database queries or hot paths, use `performance_reviewer`.
- Review everything else yourself.

Combine all findings into a single PR comment.
```

### Code generator with restricted write access

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

## Process
1. Read existing code that the plan references.
2. Implement each change in dependency order.
3. Run `go build ./...` to verify compilation.
4. Run `go test ./...` to verify tests pass.
5. Fix any failures (up to 3 cycles).
6. Open a pull request with the changes.

## Constraints
- Follow existing code conventions exactly.
- Do not refactor existing code.
- Do not add features beyond what the plan specifies.
```

### Agent with required secrets

```markdown
---
name: deployer
model: claude-sonnet-4-6
secrets:
  - JERRY_SECRET_DEPLOY_TOKEN
tools:
  - deploy
---

Deploy the service using the deploy tool.
```

If `JERRY_SECRET_DEPLOY_TOKEN` is not set in the environment, the agent fails to load with a clear error message.

### Cheap agent for simple tasks

```markdown
---
name: classifier
model: claude-haiku-4-5
temperature: 0
max_iterations: 5
---

Read the PR title and description. Classify it as one of: bugfix, feature, refactor, docs, chore.
Output exactly one word.
```
