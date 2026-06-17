# Quickstart

Set up a PR review pipeline in 5 minutes. By the end, every pull request to your repo triggers an AI code review that posts findings and sets a check status.

## Prerequisites

- A GitHub repository you can push to
- Go 1.21+ (to build Jerry from source) or the install script
- An Anthropic API key (`ANTHROPIC_API_KEY`)
- Node.js 18+ (for the pi runtime: `npm i -g @mariozechner/pi-coding-agent`)

## 1. Install Jerry

```bash
curl -sSL https://jerry.dev/install.sh | sh
```

Or from source:

```bash
go install github.com/kilupskalvis/jerry/cmd/jerry@latest
```

## 2. Initialize the project

```bash
cd your-repo
jerry init
```

This creates:

```
.jerry/
  review/
    workflow.yaml    # pipeline spec
    reviewer.md      # reviewer prompt
  settings.yaml      # org policy (deny rules, budget ceiling)
  .gitignore         # ignores local-only files
```

## 3. Check the spec

```bash
jerry validate
```

```
  ✓ review — valid
```

## 4. Test locally

```bash
jerry run review "check my code"
```

This runs every step in sequence on your machine. Agent steps invoke the pi runtime. CI steps (`post_pr_comment`, `add_check_status`) run in preview mode — they print what would be posted instead of calling the API.

## 5. Generate CI config

```bash
jerry generate
```

This compiles `.jerry/review/workflow.yaml` into `.github/workflows/jerry-review.yml` — a native GitHub Actions workflow where every step is a `jerry exec` call.

Inspect the output:

```bash
cat .github/workflows/jerry-review.yml
```

## 6. Pin the runtime

```bash
jerry lock
```

This writes `jerry.lock` with the installed pi version. The generated CI config installs this exact version.

## 7. Push

```bash
git add .jerry .github jerry.lock
git commit -m "add jerry review pipeline"
git push
```

## 8. Open a PR

Create a pull request. The `jerry-review` workflow triggers automatically. Each step appears as a native CI step in the Actions UI:

1. **Checkout** — `actions/checkout@v4` with full history
2. **Install Jerry** — pinned version
3. **Install pi** — pinned from `jerry.lock`
4. **Drift check** — `jerry generate --check` ensures nobody hand-edited the workflow
5. **review** — `jerry exec review/review` invokes the reviewer agent
6. **report** — `jerry exec review/report` posts findings as a PR comment
7. **gate** — `jerry exec review/gate` sets the check status

## What's next

- Edit `.jerry/review/reviewer.md` to tune the review prompt
- Add a [budget](spec/workflow.md#budget) to cap spend per step
- Restrict [permissions](spec/workflow.md#permissions) to read-only
- Add a second workflow: `jerry init --template feature` for ticket-to-PR automation
- Run `jerry generate` after any spec change, then push — the drift check enforces this
