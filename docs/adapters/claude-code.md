# Claude Code adapter

[Claude Code](https://claude.ai/code) is Anthropic's CLI for Claude. Jerry includes a built-in adapter that invokes `claude -p` in non-interactive mode.

## Usage

Set `runtime: claude-code` on any agent step:

```yaml
steps:
  - name: review
    prompt: reviewer.md
    runtime: claude-code
    model: claude-sonnet-4-6
```

Swap from pi with one line — the spec, prompts, and outputs are the same.

## Installation

Claude Code is installed via npm:

```bash
npm install -g @anthropic-ai/claude-code
```

Pin the version:

```bash
jerry lock
```

## How Jerry invokes Claude Code

```
claude -p "<prompt>" --output-format json [--model <m>] [--allowedTools <csv>] [--disallowedTools <csv>]
```

- `-p` — non-interactive print mode
- `--output-format json` — structured JSON response
- `--model` — model selection (always passed when set in the spec)
- Stdin is closed (non-interactive mode)

## Permission mapping

Jerry's permission patterns map to Claude Code tool flags:

| Allow pattern noun | Claude Code tool | Flag |
|---|---|---|
| `read` | `Read` | `--allowedTools` |
| `write` | `Write` | `--allowedTools` |
| `edit` | `Edit` | `--allowedTools` |
| `bash` | `Bash` | `--allowedTools` |
| `grep` | `Grep` | `--allowedTools` |
| `find` | `Glob` | `--allowedTools` |
| `ls` | `LS` | `--allowedTools` |

**Claude Code supports deny natively** via `--disallowedTools`, unlike pi where deny patterns are advisory. Jerry's `deny:` rules translate to `--disallowedTools` flags.

## Output parsing

Claude Code with `--output-format json` returns:

```json
{
  "type": "result",
  "subtype": "success",
  "result": "the text response",
  "cost_usd": 0.05,
  "is_error": false,
  "usage": { "input_tokens": 100, "output_tokens": 50 }
}
```

Jerry extracts `result` as the step's text output and maps usage directly:

| Claude Code field | Jerry Usage field |
|---|---|
| `usage.input_tokens` | `InputTokens` |
| `usage.output_tokens` | `OutputTokens` |
| `cost_usd` | `CostUSD` |

## Error handling

When `is_error` is `true`, the adapter returns an error with the subtype (e.g. `error_max_turns`) and the result text. The usage is still available for ledger recording.

## Version pinning

Same as pi: `jerry.lock` pins the version, the adapter runs `claude --version` at startup and refuses on mismatch (exit 2). The compiler reads the pin from `jerry.lock` for CI install commands.

## Structured output

Like pi, Claude Code's structured output is handled generically by Jerry's prompt directive + JSON parse — not via a native schema flag. When a step declares `outputs:`, Jerry appends an instruction asking the model to emit JSON.
