# CI Setup

Jerry runs as a single step in your CI pipeline. `jerry init` auto-detects your platform and generates the config. This guide covers manual setup and advanced configuration.

## GitHub Actions

### Review (PR trigger)

```yaml
# .github/workflows/jerry-review.yml
name: Jerry Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Jerry
        run: curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh
      - name: Run review
        run: jerry run review --trigger-file "$GITHUB_EVENT_PATH" --verbose
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Feature (dispatch trigger)

```yaml
# .github/workflows/jerry-feature.yml
name: Jerry Feature

on:
  repository_dispatch:
    types: [jerry-ticket]

jobs:
  feature:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - name: Install Jerry
        run: curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh
      - name: Configure git
        run: |
          git config user.name "jerry[bot]"
          git config user.email "jerry[bot]@users.noreply.github.com"
      - name: Run feature workflow
        run: jerry run feature --trigger-file "$GITHUB_EVENT_PATH" --verbose
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Required setup:**
1. Add `ANTHROPIC_API_KEY` as a repository secret (Settings → Secrets → Actions)
2. Enable PR creation (Settings → Actions → General → "Allow GitHub Actions to create and approve pull requests")

### Testing dispatch locally

```bash
gh api repos/OWNER/REPO/dispatches \
  -f event_type=jerry-ticket \
  -f 'client_payload[type]=ticket' \
  -f 'client_payload[source]=jira' \
  -f 'client_payload[intent]=Add search endpoint'
```

## GitLab CI

Jerry uses the `--trigger key=value` flag on GitLab since GitLab doesn't provide a single event JSON file like GitHub.

### Review (MR trigger)

```yaml
jerry-review:
  stage: test
  script:
    - curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh
    - >
      jerry run review
      --trigger type=pull_request
      --trigger source=gitlab
      --trigger intent="$CI_MERGE_REQUEST_TITLE"
      --trigger number=$CI_MERGE_REQUEST_IID
      --trigger head_sha=$CI_COMMIT_SHA
      --trigger repo_owner=$CI_PROJECT_NAMESPACE
      --trigger repo_name=$CI_PROJECT_NAME
      --trigger author=$GITLAB_USER_LOGIN
      --verbose
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
  variables:
    ANTHROPIC_API_KEY: $ANTHROPIC_API_KEY
    GITLAB_TOKEN: $GITLAB_TOKEN
```

### Feature (pipeline trigger)

```yaml
jerry-feature:
  stage: build
  script:
    - curl -sSL https://raw.githubusercontent.com/kilupskalvis/jerry/main/install.sh | sh
    - git config user.name "jerry[bot]"
    - git config user.email "jerry[bot]@users.noreply.github.com"
    - >
      jerry run feature
      --trigger type=ticket
      --trigger source=$JERRY_TICKET_SOURCE
      --trigger intent="$JERRY_TICKET_INTENT"
      --trigger repo_owner=$CI_PROJECT_NAMESPACE
      --trigger repo_name=$CI_PROJECT_NAME
      --verbose
  rules:
    - if: '$CI_PIPELINE_SOURCE == "trigger"'
  variables:
    ANTHROPIC_API_KEY: $ANTHROPIC_API_KEY
    GITLAB_TOKEN: $GITLAB_TOKEN
```

**Required setup:**
1. Add CI/CD variables: `ANTHROPIC_API_KEY`, `GITLAB_TOKEN` (project access token with API scope)
2. Create a pipeline trigger token (Settings → CI/CD → Pipeline trigger tokens)

## Trigger Input Methods

| Method | When to use |
|--------|-------------|
| `--trigger-file <path>` | GitHub Actions (`$GITHUB_EVENT_PATH`) |
| `--trigger key=value` | GitLab CI, Jenkins, any platform |
| `--trigger-stdin` | Pipe JSON from another command |
| Positional arg | Local development (`jerry run review "check for issues"`) |

These are mutually exclusive — use one per invocation.

### Available trigger fields

| Key | Type | Description |
|-----|------|-------------|
| `type` | string | `pull_request`, `ticket`, `push`, `manual` |
| `source` | string | `github`, `gitlab`, `jira`, `cli` |
| `intent` | string | PR title, ticket summary, commit message |
| `number` | int | PR/MR/issue number |
| `head_sha` | string | Commit SHA |
| `repo_owner` | string | Repository owner/namespace |
| `repo_name` | string | Repository name |
| `author` | string | User who triggered the event |
| `url` | string | Link to the PR/issue |
