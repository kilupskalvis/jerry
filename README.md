<table align="center"><tr><td>
<pre>
     ██╗███████╗██████╗ ██████╗ ██╗   ██╗
     ██║██╔════╝██╔══██╗██╔══██╗╚██╗ ██╔╝
     ██║█████╗  ██████╔╝██████╔╝ ╚████╔╝
██   ██║██╔══╝  ██╔══██╗██╔══██╗  ╚██╔╝
╚█████╔╝███████╗██║  ██║██║  ██║   ██║
 ╚════╝ ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝
</pre>
</td></tr></table>

<p align="center">
<b>Terraform for AI agents in CI.</b><br>
Declare agent pipelines once. Jerry compiles them to native CI config — GitHub Actions or GitLab CI. Any agent runtime. Governed. Portable. Zero new infrastructure.
</p>

<p align="center">
<a href="https://github.com/kilupskalvis/jerry/releases"><img src="https://img.shields.io/github/v/release/kilupskalvis/jerry" alt="Release"></a>
<a href="LICENSE"><img src="https://img.shields.io/github/license/kilupskalvis/jerry" alt="License"></a>
</p>

---

**You push a PR. An agent reviews it, posts findings, and sets a check — as native CI steps.**

```yaml
# .jerry/review/workflow.yaml
version: 1
on:
  pull_request: { types: [opened, synchronize] }
steps:
  - name: review
    prompt: reviewer.md
    runtime: pi                                  # or claude-code — one-line swap
    permissions: { allow: ["read", "bash(go test:*)"] }
    budget: { max_cost: 1.50 }
    outputs: { verdict: string, findings: string }
  - name: report
    ci: post_pr_comment
    body: "${{ steps.review.outputs.findings }}"
  - name: gate
    ci: add_check_status
    status: "${{ steps.review.outputs.verdict }}"
```

```
$ jerry generate

jerry: wrote .github/workflows/jerry-review.yml
```

```
$ jerry run review "check my PR"

▸ review (pi)
✓ review
▸ report — ci preview: post_pr_comment
  body:
    ## Jerry Review
    No critical findings. LGTM.
✓ report (preview)
▸ gate — ci preview: add_check_status
  status:
    success
✓ gate (preview)
workflow "review" completed
```

---

## Get Started

```bash
curl -sSL https://jerry.dev/install.sh | sh
cd your-repo
jerry init                                       # scaffold .jerry/ + settings
jerry generate                                   # compile to CI config
git add .jerry .github && git commit -m "add jerry review pipeline" && git push
```

Add `ANTHROPIC_API_KEY` to your CI secrets. Open a PR. Jerry reviews it.

Want ticket-to-PR automation?

```bash
jerry init --template feature                    # adds .jerry/feature/
jerry generate                                   # updates CI config
```

Assign a Jira ticket → dispatch trigger fires → planner agent → implementer agent → test gate → PR opens.

---

## Why Jerry

Everyone is wiring AI agents into CI by hand: YAML that shells out to an agent CLI with a heredoc prompt, bash gluing stdout into `gh pr comment`, copy-pasted variants with quoting bugs, no budgets, no policy, one platform only, untestable locally.

The runtimes are great. CI is great. **The declaration layer between them is missing.**

| Concern | Owner |
|---|---|
| Triggers, sequencing, retries, secrets, logs | **CI platform** (compiled from your spec) |
| LLM loop, providers, tool execution | **Agent runtime** (pi, Claude Code, or your own) |
| Portable spec, compilation, typed handoffs, budgets, policy | **Jerry** |

Jerry owns the translation — and nothing else. No LLM code, no orchestrator, no daemon.

---

## Multi-Runtime

Swap runtimes per step with one line:

```yaml
steps:
  - name: plan
    prompt: planner.md
    runtime: pi                     # cheap planning model
    model: claude-haiku-4-5
  - name: implement
    prompt: generator.md
    runtime: claude-code            # strong implementation model
    model: claude-sonnet-4-6
```

Built-in runtimes: **pi** (default), **Claude Code**. Define your own in YAML — no Go required:

```yaml
# .jerry/adapters/aider.yaml
name: aider
command: aider --yes --no-git --message
prompt: arg
parse:
  text: "output"
```

---

## Multi-Platform

Same spec, any CI:

```bash
jerry generate                          # → .github/workflows/jerry-*.yml
jerry generate --backend gitlab         # → .gitlab-ci-jerry.yml
jerry generate --backend all            # → both
```

The drift gate (`jerry generate --check`) runs as the first CI step. Hand-edit the generated YAML? It fails on the next push.

---

## Features

- **Compile, don't configure** — `.jerry/` spec → native CI YAML. Reviewable in PRs, diffable, versioned.
- **Typed handoffs** — `${{ steps.plan.outputs.approach }}` passes structured data between steps. Static validation catches typos at authoring time.
- **Budget caps** — per-step and per-run cost/token limits. Breach = hard stop (exit 4). Every attempt recorded, even retries.
- **Permission policy** — allow/deny per step + org-wide deny in `settings.yaml`. Compiled in, not bolted on.
- **Drift detection** — `jerry generate --check` is the first compiled CI step. Generated files stay in sync with the spec.
- **Local testing** — `jerry run review "check this"` executes the full pipeline locally. CI steps show previews instead of calling APIs.
- **Version pinning** — `jerry.lock` pins runtime versions. CI installs exactly what you tested against.
- **Custom adapters** — define community runtimes in YAML. Command, prompt delivery, output parsing, capabilities.

---

## Install

```bash
curl -sSL https://jerry.dev/install.sh | sh                                # recommended
go install github.com/kilupskalvis/jerry/cmd/jerry@latest                  # or Go
```

Requires: **Node.js 18+** (for pi runtime: `npm i -g @mariozechner/pi-coding-agent`)

## Commands

| Command | Description |
|---------|-------------|
| `jerry init` | Scaffold `.jerry/` with a workflow spec |
| `jerry validate` | Check the spec — schema, templates, policy |
| `jerry generate` | Compile to CI config (GitHub, GitLab, or both) |
| `jerry run <wf> [intent]` | Execute a workflow locally |
| `jerry exec <wf>/<step>` | Run one step (CI entry point) |
| `jerry lock` | Pin runtime versions to `jerry.lock` |

## Documentation

- **[Quickstart](docs/quickstart.md)** — 5 minutes from zero to CI
- **[Spec format](docs/spec/workflow.md)** — `workflow.yaml` schema, every field
- **[Template grammar](docs/spec/templates.md)** — `${{ }}` reference expressions
- **[Settings + lockfile](docs/spec/settings.md)** — org policy and runtime pinning
- **[CLI reference](docs/cli/validate.md)** — every command, every flag
- **[pi adapter](docs/adapters/pi.md)** — how Jerry invokes pi
- **[Claude Code adapter](docs/adapters/claude-code.md)** — Claude Code as a runtime
- **[Custom adapters](docs/adapters/custom.md)** — define runtimes in YAML
- **[Local development](docs/guides/local-development.md)** — `jerry run`, debugging, preview mode
- **[GitLab guide](docs/guides/gitlab.md)** — same spec, GitLab CI output
- **[FAQ](docs/faq.md)** — comparisons, limitations, security, cost model

## License

MIT
