# Permissions

Permissions control what tools can do. Deny rules block dangerous actions. Allow rules restrict tools to safe operations. Defined in `settings.yaml` (project-wide) and agent frontmatter (per-agent).

## Quick Example

```yaml
# .jerry/settings.yaml
permissions:
  deny:
    - bash: ["rm -rf *", "curl * | sh"]
    - read_file: ["*.env", "*.pem", "*.key"]
    - write_file: ["*.env", "*.pem", ".jerry/settings.yaml"]
```

## How It Works

Before every tool execution, Jerry checks the tool call against deny and allow rules:

1. **Deny checked first.** If the input matches any deny pattern → blocked.
2. **Allow checked second.** If an allow list exists for the tool and the input doesn't match → blocked.
3. **No rules** for a tool → allowed.

Blocked tool calls return an error to the LLM. The agent sees the denial reason and can adapt.

```
Permission denied: "rm -rf /" blocked by guardrail.
Denied pattern: "rm -rf *" (source: settings.yaml)
```

## Settings File

### `.jerry/settings.yaml`

Project-wide permissions. Committed to the repo. Applies to all agents.

```yaml
permissions:
  deny:
    - bash: ["rm -rf *", "rm -r /*", "chmod 777 *", "curl * | sh", "wget * | sh"]
    - read_file: ["*.env", "*.pem", "*.key", "*.secret", "*credentials*"]
    - write_file: ["*.env", "*.pem", "*.key", "*.secret", ".jerry/settings.yaml"]
  allow:
    - bash: ["go test *", "go build *", "npm *"]
    - write_file: ["src/**", "tests/**", "docs/**"]
```

`jerry init` generates a default `settings.yaml` with sane deny rules.

### `.jerry/settings.local.yaml`

User-level overrides. Gitignored. Can only **tighten** rules — never relax.

```yaml
permissions:
  deny:
    - bash: ["docker *"]
```

## Agent Permissions

Add a `permissions` block to any agent's frontmatter. Same format as settings files.

```yaml
---
name: reviewer
model: claude-sonnet-4-6
permissions:
  deny:
    - write_file: ["**"]
    - bash: ["git push *", "git commit *"]
---
```

## Resolution Chain

Three layers, most restrictive wins:

```
settings.yaml (project-wide)
  → settings.local.yaml (tighten only)
    → agent frontmatter (tighten only)
```

- **Deny lists** merge across all layers (union). If any layer denies a pattern, it's denied.
- **Allow lists** intersect across layers. Each layer can only narrow what the previous allows.

| Parent has allow | Child has allow | Result |
|-----------------|-----------------|--------|
| Yes | Yes | Intersection (only patterns in both) |
| Yes | No | Parent's allow inherited |
| No | Yes | Child's allow becomes effective |
| No | No | No restriction (all permitted minus denies) |

## Pattern Syntax

Simple glob patterns:

| Pattern | Matches |
|---------|---------|
| `rm -rf *` | `rm -rf /`, `rm -rf .` |
| `*.env` | `.env`, `prod.env` |
| `src/**` | `src/main.go`, `src/pkg/foo.go` |
| `go test *` | `go test ./...`, `go test -v` |
| `**` | Anything (blocks all for that tool) |

- `*` matches any characters within a single segment.
- `**` matches recursively across directory separators.
- For commands (containing spaces), `*` matches any suffix at that position.
- For file paths (no spaces), standard `filepath.Match` semantics apply.

## What Gets Matched

| Tool | Pattern matched against |
|------|----------------------|
| `bash` | The `command` string |
| `read_file` | The `path` string |
| `write_file` | The `path` string |
| CI tools | Tool name only (no argument matching) |
| Custom tools | Tool name only |

## Examples

### Read-only agent

```yaml
permissions:
  deny:
    - write_file: ["**"]
    - bash: ["git push *", "git commit *", "git add *"]
```

### Restrict writes to specific directories

```yaml
permissions:
  allow:
    - write_file: ["src/**", "tests/**"]
```

### Allow only specific commands

```yaml
permissions:
  allow:
    - bash: ["go test *", "go build *", "go vet *"]
  deny:
    - bash: ["go install *"]
```

### Block sensitive file access project-wide

```yaml
# .jerry/settings.yaml
permissions:
  deny:
    - read_file: ["*.env", "*.pem", "*.key", "*.secret", "*credentials*", "*.p12"]
    - write_file: ["*.env", "*.pem", "*.key", "*.secret"]
```
