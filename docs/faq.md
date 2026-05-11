# FAQ

## How much does it cost per run?

Depends on the workflow, codebase size, and model. Rough estimates:

| Workflow | Model | Approximate Cost |
|----------|-------|-----------------|
| Code review (small PR) | claude-sonnet-4-6 | $0.05 – $0.15 |
| Code review (large PR) | claude-sonnet-4-6 | $0.15 – $0.40 |
| Feature generation (simple) | claude-sonnet-4-6 | $0.50 – $1.50 |
| Feature generation (complex) | claude-sonnet-4-6 | $1.50 – $5.00 |

Cost is driven by input tokens (codebase context) and output tokens (generated code). Multi-step workflows (planner + generator) cost more because each step makes LLM calls.

Use `--verbose` to see token counts per turn.

## Which LLM models work?

Any model from Anthropic or OpenAI:

- **Anthropic:** `claude-sonnet-4-6`, `claude-opus-4-6`, `claude-haiku-4-5`
- **OpenAI:** `gpt-4o`, `o1`, `o3`, `o4-mini`

Set in agent frontmatter (`model: claude-sonnet-4-6`) or via `JERRY_DEFAULT_MODEL` env var.

## Can agents modify files?

Yes. Every agent has `bash`, `read_file`, and `write_file` always available. The CI runner is the sandbox — agents can do anything within the runner's environment.

## Is it safe to run in CI?

Jerry runs inside your CI runner (a fresh container per job). Agents can only access what the CI environment provides:
- Files in the checked-out repo
- Secrets you explicitly inject via env vars
- Network access the runner has

The `bash` tool runs commands in a clean environment with only `PATH`, `HOME`, and `JERRY_SECRET_*` variables — process env (including `ANTHROPIC_API_KEY`) is not leaked to agent commands.

## Can I use Jerry without CI?

Yes. Jerry works locally:

```bash
jerry run review "Check for common issues"
jerry run feature "Add pagination to users endpoint"
```

The only difference: CI tools (`post_pr_comment`, `create_pull_request`) need `GITHUB_TOKEN` or `GITLAB_TOKEN` to work. Without them, agents skip CI tools and produce text output instead.

## What happens if a step fails?

The workflow aborts. If a script step returns non-zero, subsequent steps don't run. If an agent hits `max_iterations`, it returns what it has. Failed runs are logged locally (`jerry logs`).

Steps can configure retries:

```yaml
steps:
  - agent: generator
    retries: 2
    timeout: 600s
```

## Can I use custom models or fine-tunes?

Yes. Set `provider` explicitly in the agent frontmatter:

```yaml
---
name: my-agent
provider: openai
model: ft:gpt-4o:my-org:custom-model
---
```

## Does Jerry support parallel steps?

Not yet. Steps execute sequentially. Each step sees all previous step outputs.

## How do I debug agent behavior?

1. **`--verbose` flag** — shows every tool call with arguments, results, and LLM turn summaries
2. **`jerry run --dry-run`** — validates workflow without executing
3. **`jerry logs`** — review past local runs
4. **Run locally first** — test with `jerry run feature "description"` before setting up CI triggers
