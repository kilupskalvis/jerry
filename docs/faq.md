# FAQ

## How Jerry compares

### vs vendor CI actions (claude-code-action, codex-action)

| | Vendor action | Jerry |
|---|---|---|
| Setup | Copy YAML snippet | `jerry init && jerry generate` |
| Steps | One agent, one action | Multi-step pipelines with typed handoffs |
| Runtimes | Locked to one vendor | Any runtime (pi default, more coming) |
| CI platforms | GitHub only | GitHub now, GitLab planned |
| Budgets | None | Per-step and per-run caps |
| Permissions | Action-level | Per-step allow/deny with org policy |
| Local testing | No | `jerry run` with preview mode |
| Spec reviewable | No (YAML in workflow) | Yes (`.jerry/` in PRs) |

Vendor actions are the right choice for a single agent step with no governance needs. Jerry is for multi-step pipelines, cross-vendor composition, and teams that want policy.

### vs hand-rolled YAML + bash

Every team wiring agents into CI is writing this by hand: a GitHub Actions YAML that shells out to an agent CLI with a heredoc prompt, a bash pipeline gluing stdout into `gh pr comment`, copy-pasted variants with quoting bugs, no budgets, no permissions.

Jerry replaces the hand-rolled YAML with a reviewable spec. The generated YAML is the same shape — but compiled from a declaration, governed by policy, and drift-checked.

### vs n8n / workflow SaaS

n8n composes SaaS API calls from a hosted server. Jerry compiles agent pipelines that mutate git working trees inside CI runners you already own. Different substrate (git vs SaaS APIs), different artifact (CI config vs hosted workflows), different buyer (engineering teams vs operations).

## Known limitations

### pi deny patterns are advisory

pi allows or disables tools at the tool level (`--tools read,bash` or `--no-tools`) but cannot enforce pattern-level deny rules like `deny: ["bash(rm:*)"]`. Jerry's deny rules are enforced at authoring time (validation and policy) but are advisory at the pi runtime level. A determined model can work around them.

### Structured output via prompt convention

When a step declares `outputs:`, Jerry appends a directive to the prompt asking the model to emit JSON with the declared keys. It then parses the response text for the outermost `{...}` block. This works reliably with current models but is a convention, not a schema-enforced contract. A future pi version may support native structured output.

### CI platform support

GitHub Actions and GitLab CI are supported. Use `jerry generate --backend github` (default) or `jerry generate --backend gitlab`. The spec format is platform-neutral — only the backend output changes. See the [GitLab guide](guides/gitlab.md).

### post_review_comment not yet live

The `post_review_comment` ci action works in preview mode but requires `path` and `line` fields not yet in the spec. Use `post_pr_comment` for comment-level findings.

### No streaming

pi supports streaming output via `--mode rpc`. Jerry's `StreamingAdapter` interface exists but is not wired. Agent steps block until completion. For long-running steps, check CI logs for the step's native output.

### Budget enforcement is post-hoc

For one-shot runtime spawns, budget caps are checked after the step completes — the money is already spent. The ledger records every attempt (retried steps accumulate). This is inherent to the one-shot model and documented as such.

## Security model

### Secret isolation

Runtimes and shell steps receive an explicit env allowlist — only the names declared in `env:` with values from the CI runner's environment. Never `os.Environ()`. Secret values exist only in the CI runner's env; the spec and generated YAML contain only names.

### Prompt injection

Trigger content (PR titles, ticket bodies, commit messages) is attacker-influenced text. Jerry fences it in `<untrusted-trigger>` blocks when assembling prompts. The default review template uses read-only permissions — an injected instruction cannot make a read-only agent write or exfiltrate.

### Fork PRs

The compiler emits `on: pull_request` — never `pull_request_target`. Fork-originated runs get no secrets; agent steps fail fast (exit 2, missing API key) instead of executing attacker-influenced prompts with credentials.

### Drift gate

`jerry generate --check` runs as the first step in every generated workflow. Hand-edits to generated files fail the run before any agent executes.

## Cost model

Jerry keeps no price tables. Cost is whatever the runtime reports. The budget ledger records per-step and per-run totals. When a cap is breached, the step exits with code 4 (budget exceeded) and the pipeline stops.

Budget caps are useful for:
- Preventing runaway costs from agent loops
- Setting per-pipeline ceilings via `settings.yaml`
- Audit: `ledger.json` records every attempt, including failed/retried ones
