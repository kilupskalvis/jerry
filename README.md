# Jerry

**Terraform for AI agents in CI.**

Declare agent pipelines once — review agents, security scans, ticket-to-PR
automation — in a small spec that lives in your repo. Jerry compiles it to
native CI config for GitHub Actions or GitLab CI and gives your agents typed
handoffs, budgets, permissions, and an audit trail. Any agent runtime. Any CI.
Zero new infrastructure.

```
.jerry/review/workflow.yaml   ──jerry generate──▶   .github/workflows/jerry-review.yml
```

## Why

Everyone is wiring agents into CI by hand: YAML that shells out to an agent
CLI, bash gluing stdout into PR comments, no budgets, no policy, GitHub-only,
untestable locally. The runtimes are great. CI is great. The declaration layer
between them is missing. Jerry owns that layer — and nothing else:

- **CI orchestrates.** Sequencing, retries, secrets, logs — your platform's job.
- **Runtimes agent.** pi (default), Claude Code, Codex — the loop is their job.
- **Jerry translates.** One portable, reviewable, governed spec → any of them.

## 5 minutes

```bash
curl -sSL https://jerry.dev/install.sh | sh
cd your-repo
jerry init            # scaffold .jerry/review/ + generate CI config
git add .jerry .github && git commit -m "add jerry review pipeline" && git push
```

Open a PR. The reviewer agent reads the diff, posts findings, sets a check —
as native CI steps you can watch live.

## The spec

```yaml
# .jerry/review/workflow.yaml
version: 1
on:
  pull_request: { types: [opened, synchronize] }
steps:
  - name: review
    prompt: reviewer.md
    permissions: { allow: ["read", "bash(go test:*)"] }
    budget: { max_cost: 1.50 }
    outputs: { verdict: string, findings: list }
  - name: report
    ci: post_pr_comment
    body: "${{ steps.review.outputs.findings }}"
  - name: gate
    ci: add_check_status
    status: "${{ steps.review.outputs.verdict }}"
```

Swap runtimes per step (`runtime: claude-code`). Pipe one step's diff into the
next step's prompt (`${{ steps.implement.diff }}`). Cap spend per step and per
run. Same spec compiles to GitHub and GitLab; `jerry run review` executes it
locally first.

## Status

v3 (compiler architecture) under active development — design docs land in `docs/`
as features ship. MIT.
