# jerry init

Scaffold a new `.jerry/` directory.

## Usage

```
jerry init [flags]
```

## Description

Creates a `.jerry/` directory with a workflow spec, prompt files, org policy, and gitignore entries. The default template is `review` — a PR review pipeline ready to customize.

After init, run `jerry validate` to verify the spec, then `jerry generate` to compile CI config.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--path` | string | current directory | Directory to initialize in |
| `--template` | string | `"review"` | Workflow template. Available: `review`, `feature` |

## Templates

### review (default)

A PR review pipeline: agent reviews the diff, posts findings as a PR comment, sets a check status.

```
.jerry/
  review/
    workflow.yaml
    reviewer.md
  settings.yaml
```

### feature

A ticket-to-PR pipeline: planner agent → implementer agent → test gate → open PR.

```
.jerry/
  feature/
    workflow.yaml
    planner.md
    generator.md
```

Add it to an existing project:

```bash
jerry init --template feature
```

## What init creates

- **Workflow directory** with `workflow.yaml` and prompt `.md` files
- **`settings.yaml`** (first init only) with default deny rules for dangerous shell patterns and credential files
- **`.jerry/.gitignore`** ignoring `settings.local.yaml`
- **`.gitignore` entry** for `.jerry-run/` (the ephemeral context directory)

## Example

```bash
$ jerry init
Jerry initialized:
  .jerry/review/workflow.yaml
  .jerry/review/reviewer.md

Next steps:
  jerry validate              Check the spec is valid
  jerry run review "your task"   Run locally in preview mode
  jerry generate              Compile to CI config (.github/workflows/)
  git add .jerry .github && git push
```
