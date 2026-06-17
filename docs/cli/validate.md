# jerry validate

Check that the `.jerry/` spec is valid.

## Usage

```
jerry validate [flags]
```

## Description

Validates every workflow, `settings.yaml`, and `jerry.lock` under `.jerry/`. Checks include:

- Schema: required fields, correct types, kebab-case names, kind exclusivity (exactly one of prompt/run/ci)
- Templates: every `${{ }}` reference resolves to a known step and declared output key, no forward references
- Policy: workflow budgets fit under settings ceiling, runtimes are in the allowlist
- Lockfile: pinned runtimes match declared runtimes (warning if unpinned)

Validation runs the same checks that `jerry generate` applies before compiling, so `validate` is the fast authoring-time feedback loop.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--verbose` | bool | false | Show detailed output |
| `--quiet` | bool | false | Show only errors |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | All workflows valid |
| 1 | Validation errors found |

## Example

```bash
$ jerry validate
  ✓ feature — valid
  ✓ review — valid
```
