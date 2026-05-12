# FAQ

## How much does it cost per run?

Depends on the workflow, codebase size, and model. Rough estimates:

| Workflow | Model | Approximate Cost |
|----------|-------|-----------------|
| Code review (small PR) | claude-sonnet-4-6 | $0.05 – $0.15 |
| Code review (large PR) | claude-sonnet-4-6 | $0.15 – $0.40 |
| Feature generation (simple) | claude-sonnet-4-6 | $0.50 – $1.50 |
| Feature generation (complex) | claude-sonnet-4-6 | $1.50 – $5.00 |

Cost is driven by input tokens (codebase context) and output tokens (generated code). Multi-step workflows cost more because each step makes LLM calls. Subagent invocations add to the total.

Use `--verbose` to see token counts and cache stats per turn. Jerry automatically uses Anthropic's prompt caching to reduce costs on repeated system prompts and tool definitions.

## Which LLM models work?

Any model from Anthropic or OpenAI:

- **Anthropic:** `claude-sonnet-4-6`, `claude-opus-4-6`, `claude-haiku-4-5`
- **OpenAI:** `gpt-4o`, `o1`, `o3`, `o4-mini`

Set in agent frontmatter (`model: claude-sonnet-4-6`) or via `JERRY_DEFAULT_MODEL` env var.

## Can agents modify files?

Yes. Every agent has `bash`, `read_file`, and `write_file` always available. Use [permissions](configuration/permissions.md) to restrict what agents can do — block writes, limit commands, protect sensitive files.

## Is it safe to run in CI?

Jerry runs inside your CI runner (a fresh container per job). Safety layers:

1. **Clean environment** — `bash` runs with only `PATH`, `HOME`, and `JERRY_SECRET_*`. API keys are not leaked to agent commands.
2. **Permissions** — `settings.yaml` defines project-wide deny/allow rules. Block dangerous commands, protect sensitive files.
3. **Agent-level restrictions** — each agent can have its own permissions that further narrow what it can do.
4. **Validation** — `jerry validate` catches misconfigurations before CI runs.

## Can I use Jerry without CI?

Yes. Jerry works locally:

```bash
jerry run review "Check for common issues"
jerry run feature "Add pagination to users endpoint"
```

CI tools (`post_pr_comment`, `create_pull_request`) need `GITHUB_TOKEN` or `GITLAB_TOKEN`. Without them, agents skip CI tools and produce text output instead.

## What happens if a step fails?

The workflow aborts. Subsequent steps don't run. Configure retries to handle transient failures:

```yaml
steps:
  - agent: generator
    retries: 2
    timeout: 600s
```

Use [hooks](configuration/hooks.md) to get notified on failure.

## Can agents delegate to other agents?

Yes, using [subagent tools](configuration/tools.md#subagent-tools). An agent can invoke another agent as a tool at runtime. The parent agent decides when and what to delegate.

```yaml
tools:
  - security_reviewer
```

## Does Jerry support parallel steps?

Steps execute sequentially. However, tool calls within a step execute in parallel — if an agent requests 5 file reads in one turn, they all run concurrently.

## How do I get notified when a workflow completes?

Use [hooks](configuration/hooks.md). Fire a webhook, send a Slack message, or run any shell command on workflow events:

```yaml
hooks:
  on_workflow_complete:
    - run: curl -X POST $JERRY_SECRET_SLACK_WEBHOOK -d '{"text":"Done"}'
```

## How do I debug agent behavior?

1. **`--verbose` flag** — shows every tool call with arguments, results (indented blocks), and LLM turn summaries with token counts and cache stats.
2. **`jerry run --dry-run`** — validates workflow without executing.
3. **`jerry validate`** — catches config errors, typos (with "did you mean?" suggestions), missing tools, and invalid permissions.
4. **`jerry logs`** — review past local runs.

## How do I validate my configuration?

```bash
jerry validate
```

Checks:
- Workflow YAML structure and unknown fields
- Agent frontmatter types and unknown fields (with "did you mean?" suggestions)
- Tool references resolve (built-in, custom, and subagent)
- Custom tool definitions are valid
- Settings file permissions structure
- Hook event names, missing `run` fields, tool filter scope
