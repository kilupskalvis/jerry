# Prompt files

Agent steps reference prompt files by path. The file's entire body becomes the runtime's instruction after template resolution.

## Format

Plain Markdown. No special syntax beyond `${{ }}` templates.

```markdown
# Code Reviewer

You are reviewing a pull request.

## Context

${{ trigger.intent }}

## Instructions

Review the diff for correctness, security, and completeness.
Report findings as structured output.
```

## File reference vs inline

In `workflow.yaml`, the `prompt:` field accepts two forms:

- **File reference** — a string ending in `.md` is a path relative to the workflow directory:
  ```yaml
  prompt: reviewer.md    # reads .jerry/review/reviewer.md
  ```

- **Inline string** — anything else is used directly as the prompt:
  ```yaml
  prompt: "Review this PR for correctness issues."
  ```

## Optional frontmatter

Prompt files may include YAML frontmatter with default values for `model`, `permissions`, and `budget`. The workflow.yaml always overrides frontmatter.

```markdown
---
model: claude-sonnet-4-6
---
# Code Reviewer

Review the pull request...
```

Frontmatter is optional. Most prompts don't need it — set these fields in workflow.yaml where they're visible alongside the rest of the pipeline.

## Template resolution

Templates in prompt files are resolved at execution time by `jerry exec`, not at compile time. The prompt file on disk contains literal `${{ }}` expressions; `jerry validate` checks them statically.

See [template grammar](templates.md) for all available references.

## Context assembly

The prompt the runtime receives is not just the file body. `jerry exec` prepends context based on the step's `context:` field:

1. **Trigger block** — fenced as `<untrusted-trigger>` with the normalized trigger data
2. **Prior step outputs** — text from earlier steps, in execution order

Then the prompt file body follows, with all `${{ }}` templates resolved.

The `context:` field controls what's prepended. See [template grammar — context assembly](templates.md#context-assembly).
