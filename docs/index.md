# Jerry

**Terraform for AI agents in CI.**

Declare agent pipelines once — review agents, security scans, ticket-to-PR automation — in a small spec that lives in your repo. Jerry compiles it to native CI config for GitHub Actions or GitLab CI and gives your agents typed handoffs, budgets, permissions, and an audit trail. Any agent runtime. Any CI. Zero new infrastructure.

```
.jerry/review/workflow.yaml   ──jerry generate──▶   .github/workflows/jerry-review.yml
```

## How it works

```
AUTHORING PLANE (your machine)
  .jerry/ spec
    ├── jerry validate        schema + template checks
    ├── jerry generate        emit .github/workflows/jerry-*.yml
    ├── jerry generate --check   drift gate (exit 2 on mismatch)
    └── jerry run <wf>        local execution in preview mode

EXECUTION PLANE (CI runner, one process per step)
  CI platform owns sequencing, retries, secrets, logs
    └── jerry exec <wf>/<step>
          resolve trigger → build prompt → invoke runtime → capture output
  Agent runtime (pi) owns the LLM loop
```

Jerry does not run agents, orchestrate steps, or store logs. CI does the first, runtimes do the second, and the platform does the third. Jerry owns the declaration layer between them.

## Install

```bash
curl -sSL https://jerry.dev/install.sh | sh
```

Or build from source:

```bash
go install github.com/kilupskalvis/jerry/cmd/jerry@latest
```

## Get started

See the [5-minute quickstart](quickstart.md) to set up a PR review pipeline.

## Documentation

- **[Quickstart](quickstart.md)** — 5 minutes from zero to CI
- **[Spec format](spec/workflow.md)** — the `.jerry/` directory and `workflow.yaml` schema
- **[Template grammar](spec/templates.md)** — `${{ }}` reference expressions
- **[Settings + lockfile](spec/settings.md)** — org policy and runtime pinning
- **[CLI commands](cli/validate.md)** — validate, generate, run, exec, init, lock
- **[pi adapter](adapters/pi.md)** — how Jerry invokes pi
- **[Local development](guides/local-development.md)** — `jerry run`, debugging, preview mode
- **[FAQ](faq.md)** — comparisons, limitations, security, cost model
