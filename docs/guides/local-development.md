# Local development

Run and debug Jerry pipelines on your machine before pushing to CI.

## Running a workflow

```bash
jerry run review "check the auth changes"
```

This executes every step in sequence, same semantics as CI:
- Agent steps invoke the pi runtime
- Shell steps run under `/bin/sh`
- CI steps run in **preview mode** — they print what would be posted instead of calling the API

## Preview mode for ci: steps

Locally, `post_pr_comment` and `add_check_status` steps print the fully resolved payload:

```
▸ report — ci preview: post_pr_comment
  body:
    ## Jerry Review
    No critical findings.
✓ report (preview)
```

This lets you verify template resolution and step handoffs without needing a real PR context. Use `--ci-live` to call the real API (requires `GITHUB_TOKEN` and a valid trigger).

## Inspecting step results

Use `--keep-ctx` to preserve the context directory:

```bash
jerry run review "test" --keep-ctx
```

Then inspect:

```bash
cat .jerry-run/trigger.json          # trigger data
cat .jerry-run/steps/review/output.txt     # agent text output
cat .jerry-run/steps/review/outputs.json   # structured outputs
cat .jerry-run/steps/review/diff.patch     # workspace changes
cat .jerry-run/steps/review/usage.json     # tokens + cost
cat .jerry-run/ledger.json                 # cumulative usage
```

Without `--keep-ctx`, the context dir is cleaned up after the run.

## Environment setup

Jerry needs:

1. **pi installed** — `npm i -g @mariozechner/pi-coding-agent`
2. **API key** — set `ANTHROPIC_API_KEY` in your environment or in a `.env` file at the repo root (Jerry loads `.env` automatically)
3. **Git repo** — Jerry uses git for diff capture; the working directory must be a git repo with at least one commit

## Common issues

### "unknown runtime" error

pi is not installed or not on PATH. Install it:

```bash
npm i -g @mariozechner/pi-coding-agent
```

### Version mismatch

The installed pi version doesn't match `jerry.lock`. Either update the lockfile:

```bash
jerry lock
```

Or install the pinned version:

```bash
npm i -g @mariozechner/pi-coding-agent@0.73.1
```

### API key errors

pi returns 401. Check that `ANTHROPIC_API_KEY` is set and valid. Jerry loads `.env` from the repo root if present — add it there for local development:

```
ANTHROPIC_API_KEY=sk-ant-...
```

Do not commit `.env`. The default `settings.yaml` denies `read(.env)` for this reason.

### "no trigger recorded" error

The first `jerry exec` call in a run must provide a trigger. When using `jerry run`, this is automatic. When calling `jerry exec` directly, provide `--intent`:

```bash
jerry exec review/review --intent "test review"
```

## Validating before pushing

A good pre-push checklist:

```bash
jerry validate                # spec is valid
jerry generate --check        # generated YAML matches spec
jerry run review "test run"   # pipeline works locally
```

If `jerry generate --check` fails, the spec changed but the generated CI wasn't updated. Run `jerry generate` and commit the result.
