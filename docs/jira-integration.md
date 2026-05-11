# Jira Integration

Connect Jira to Jerry so assigning a ticket kicks off automatic implementation.

## The Flow

```
Jira ticket assigned to "Jerry"
  → Jira Automation fires HTTP request
    → GitHub Actions / GitLab CI starts
      → Jerry plans, implements, tests
        → Pull request opened for review
```

Total time: under 5 minutes from ticket assignment to PR.

## Quick Setup

```bash
jerry setup jira
```

This interactive wizard detects your repo, CI platform, and workflows, then prints copy-paste-ready configuration. Follow the steps it outputs.

## Manual Setup

### Step 1: CI Workflow

Make sure you have the feature workflow:

```bash
jerry init --template feature
```

This creates `.jerry/feature/` and the CI config for your platform.

### Step 2: Secrets

**GitHub Actions:**
- Add `ANTHROPIC_API_KEY` as a repository secret
- Enable "Allow GitHub Actions to create and approve pull requests" in Settings → Actions

**GitLab CI:**
- Add `ANTHROPIC_API_KEY` and `GITLAB_TOKEN` as CI/CD variables
- Create a pipeline trigger token

### Step 3: GitHub PAT (GitHub Actions only)

Create a Personal Access Token at https://github.com/settings/tokens with `repo` scope. The Jira automation rule uses this to trigger GitHub Actions.

### Step 4: Jira Automation Rule

Go to your Jira project → Project Settings → Automation → Create rule.

**Trigger:** Field value changed → Assignee

**Condition:** Assignee equals "Jerry" (or your chosen name)

**Action:** Send web request

#### For GitHub Actions:

```
URL: https://api.github.com/repos/<owner>/<repo>/dispatches
Method: POST
Headers:
  Authorization: Bearer <your-github-pat>
  Content-Type: application/json
Body:
{
  "event_type": "jerry-ticket",
  "client_payload": {
    "type": "ticket",
    "source": "jira",
    "intent": "{{issue.summary}}",
    "raw_payload": {
      "key": "{{issue.key}}",
      "summary": "{{issue.summary}}",
      "description": "{{issue.description}}"
    }
  }
}
```

#### For GitLab CI:

```
URL: https://gitlab.com/api/v4/projects/<project-id>/trigger/pipeline
Method: POST
Headers:
  Content-Type: application/x-www-form-urlencoded
Body:
  token=<pipeline-trigger-token>
  ref=main
  variables[JERRY_TICKET_SOURCE]=jira
  variables[JERRY_TICKET_INTENT]={{issue.summary}}
  variables[JERRY_TICKET_KEY]={{issue.key}}
  variables[JERRY_TICKET_DESCRIPTION]={{issue.description}}
```

### Step 5: Test

Create a Jira ticket with a clear summary and description. Assign it to "Jerry." Watch the CI Actions tab — you should see the workflow start within a minute.

## Tips

- **Write clear ticket descriptions.** The agent uses the summary as intent and the description for context. "Add PATCH /users/{id} endpoint" is better than "update users."
- **Start small.** Test with a simple ticket before graduating to complex features.
- **Review the PR.** Jerry opens a PR, not a merge. You review and merge.
- **Check CI logs.** Run with `--verbose` (default in generated templates) to see every tool call and LLM response.
