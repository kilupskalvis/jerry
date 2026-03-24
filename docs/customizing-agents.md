# Customizing Motif Agents

Motif ships with three core agents (context, plan, generate) that work out of the box. This guide explains how to adapt them for your team's codebase and conventions.

## Getting Started

Run `motif init` to scaffold the core agents into `.motif/agents/`. Then customize them in place — they're just markdown files.

## Adding Project Conventions

The most impactful customization is adding your team's coding standards to the generate agent. Append a section to `.motif/agents/generate.md`:

```markdown
## Project Conventions

- All handlers must use the `respond` helper from pkg/http for consistent error formatting
- Database access goes through the repository layer — never call the DB directly from handlers
- All new endpoints must have an entry in docs/api.md
- Test files use testify/assert, not the standard testing package
- Environment variables are accessed through the config package, never os.Getenv directly
```

The context agent benefits from similar guidance about where to look:

```markdown
## Project-Specific Guidance

- The API layer lives in internal/api/ — each resource has its own file
- Shared types are in internal/types/ — always check here for existing types before creating new ones
- Migration files are in db/migrations/ — ordered by timestamp
```

## Constraining Tool Access

Use tool constraints to prevent agents from writing to protected directories or running dangerous commands:

```yaml
tools:
  - read_file
  - write_file:
      restrict_to:
        - src/
        - tests/
  - run_command:
      allow:
        - go test
        - go build
        - go vet
      deny:
        - rm
        - curl
        - wget
```

## Adjusting Models and Limits

For large codebases, the context agent may need more iterations:

```yaml
max_iterations: 50  # default is 30
timeout: 600s       # default is 300s
```

For complex generation tasks, use a stronger model:

```yaml
model: claude-opus-4-6  # more capable for complex multi-file changes
```

For simple tasks, use a faster model to save cost:

```yaml
model: claude-haiku-4-5
```

You can also set defaults in `.motif/config.yaml` so you don't repeat model settings in every agent:

```yaml
defaults:
  model: claude-sonnet-4-6
  timeout: 600s
```

## Creating Custom Agents

You can create new agent definitions for specialized tasks. For example, a security review agent:

```markdown
---
name: security-reviewer
model: claude-sonnet-4-6
max_iterations: 20
tools:
  - read_file
  - search_codebase
  - glob
context_access:
  - trigger
  - codebase
  - plan
output_key: security_review
output_schema:
  issues:
    type: array
    items:
      severity: string
      file: string
      description: string
      recommendation: string
  approved: boolean
---

# Security Reviewer

Review the planned changes for security issues...
```

Then add it to your pipeline:

```yaml
steps:
  - name: context
    agent: ./agents/context.md
  - name: plan
    agent: ./agents/plan.md
  - name: security-review
    agent: ./agents/security-reviewer.md
  - name: generate
    agent: ./agents/generate.md
```

## Custom Pipelines

The `feature.yaml` pipeline is a starting point. Create additional pipelines for different workflows:

- `hotfix.yaml` — skip context analysis, use a minimal plan, generate quickly
- `refactor.yaml` — deeper context analysis, no new features, focus on code quality
- `test.yaml` — analyze untested code, generate test files only

Each pipeline is a YAML file in `.motif/pipelines/`.

## Tips

- **Start simple.** Use the core agents as-is first. Customize only after you see what the agents get wrong on your codebase.
- **Be specific.** Vague instructions like "follow best practices" don't help. Say exactly what your conventions are.
- **Iterate.** Run the pipeline, review the output, update the agent instructions, repeat.
- **Version control.** Agent definitions are code — review changes in PRs like any other code change.
