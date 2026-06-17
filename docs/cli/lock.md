# jerry lock

Update `jerry.lock` with installed runtime versions.

## Usage

```
jerry lock [flags]
```

## Description

Queries installed runtimes for their version (e.g., `pi --version`) and writes the result to `.jerry/jerry.lock`. The lockfile is committed to the repo and consumed by the compiler to emit pinned install commands in generated CI config.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--verbose` | bool | false | Show detailed output |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Lockfile written |
| 1 | Error (runtime not installed, version query failed) |

## Example

```bash
$ jerry lock
jerry: locked pi 0.73.1

$ cat .jerry/jerry.lock
version: 1
runtimes:
    pi:
        package: "@mariozechner/pi-coding-agent"
        version: 0.73.1
```

## When to run

- After installing or upgrading a runtime (`npm i -g @mariozechner/pi-coding-agent@latest`)
- After `jerry init` if you want pinned versions in CI immediately
- Before committing if you changed your local runtime version

The lockfile is analogous to Terraform's `.terraform.lock.hcl`: it ensures CI installs exactly what you tested against.
