# GitLab CI

Jerry compiles the same `.jerry/` spec to GitLab CI as it does to GitHub Actions. This guide covers the differences.

## Generate GitLab config

```bash
jerry generate --backend gitlab
```

This produces `.gitlab-ci-jerry.yml` at the repo root. All workflows are in one file with separate jobs.

## Structure

GitLab CI config uses jobs and stages instead of GitHub's workflow/steps model:

- **Stages** enforce ordering: `preamble` → per-step stages
- **Jobs** are named `jerry-<workflow>-<step>` with `needs:` dependencies
- **Rules** map spec triggers to GitLab conditions:
  - `on.pull_request` → `$CI_PIPELINE_SOURCE == "merge_request_event"`
  - `on.push` → `$CI_COMMIT_BRANCH == "<branch>"`
  - `on.dispatch` → `$CI_PIPELINE_SOURCE == "trigger"`
  - `on.schedule` → `$CI_PIPELINE_SOURCE == "schedule"`

## Secrets

GitLab uses CI/CD variables, not a `secrets` namespace. The generated config references them as `$VARIABLE_NAME` (vs GitHub's `${{ secrets.VARIABLE_NAME }}`).

Set your variables in GitLab: Settings → CI/CD → Variables. Add `ANTHROPIC_API_KEY` and any other secrets your workflows need. Mark them as masked and protected.

## Checkout

GitLab CI checks out your repo automatically. The generated config sets `GIT_DEPTH: 0` for full history (diff-aware steps need base refs).

## Trigger resolution

GitLab has no equivalent of GitHub's `$GITHUB_EVENT_PATH` (a JSON file with the full event payload). Instead, `jerry exec` reads trigger data from `--trigger key=value` flags or from environment variables that GitLab sets automatically (`$CI_MERGE_REQUEST_IID`, `$CI_MERGE_REQUEST_TITLE`, etc.).

When running in GitLab CI (`$GITLAB_CI=true`), ci: steps automatically run in live mode.

## Preamble

Every generated GitLab pipeline starts with a `jerry-install` job that:

1. Installs Jerry (pinned version)
2. Installs runtimes from `jerry.lock`
3. Runs `jerry generate --check` (drift gate)

## Including the generated file

Add to your `.gitlab-ci.yml`:

```yaml
include:
  - local: .gitlab-ci-jerry.yml
```

Or use `.gitlab-ci-jerry.yml` as your CI config directly in GitLab settings.

## Both platforms

To support both GitHub and GitLab from the same spec:

```bash
jerry generate --backend all
```

This writes both `.github/workflows/jerry-*.yml` and `.gitlab-ci-jerry.yml`. The drift check in each platform's pipeline only verifies its own files.
