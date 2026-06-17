# pi adapter

[pi](https://github.com/mariozechner/pi-coding-agent) (`@mariozechner/pi-coding-agent`) is an open-source coding agent with a unified LLM API across ~15 providers. Jerry uses pi as its default runtime — shelling out to it for agent steps. Zero provider code to maintain.

## Installation

```bash
npm install -g @mariozechner/pi-coding-agent
```

Pin the version:

```bash
jerry lock
```

## How Jerry invokes pi

For each agent step, `jerry exec` spawns:

```
pi --print --mode json --model <model> [--tools <csv> | --no-tools] "<prompt>"
```

- `--print` — non-interactive: process the prompt and exit
- `--mode json` — emit a JSONL event stream (one JSON object per line)
- `--model` — always passed; pi's default provider is Google, so explicit model selection ensures the right provider
- Prompt is a positional argument
- **Stdin is closed** — pi hangs on open stdin in print mode. Jerry leaves `cmd.Stdin = nil` (connects `/dev/null`)

## API keys

pi reads API keys from environment variables (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.). Jerry passes these through the env allowlist declared in `workflow.yaml` `env:` — no `--api-key` flag needed.

## Version pinning

`jerry.lock` pins the pi version. The adapter runs `pi --version` at startup and refuses to proceed if the installed version doesn't match (exit 2). This preflight prevents subtle version-drift failures.

The compiler reads `jerry.lock` and emits `npm install -g @mariozechner/pi-coding-agent@<version>` in the CI preamble.

## Permission mapping

Jerry's `permissions.allow` patterns map to pi's tool flags:

| Allow pattern noun | pi tool |
|---|---|
| `read` | `read` |
| `edit` | `edit` |
| `write(...)` | `write` |
| `bash(...)` | `bash` |
| `grep` | `grep` |
| `find` | `find` |
| `ls` | `ls` |

Empty allow list → `--no-tools`. Multiple nouns → `--tools read,bash,...` (comma-separated, deduped).

**Known limitation:** `deny` patterns have no pi flag equivalent. pi allowlists at tool granularity, not pattern granularity — it can enable or disable `bash`, but cannot enforce `deny: ["bash(rm:*)"]` while allowing other bash commands. Deny rules are enforced at Jerry's authoring-time validation and org policy level, but are advisory at the pi runtime level. This is documented in the spec as a lossy translation.

## Output parsing

pi emits JSONL events. Jerry reads the **last `message_end` event with `message.role == "assistant"`** and extracts:

- **Text** — concatenation of all `content[]` blocks with `type == "text"`
- **Stop reason** — `stopReason: "stop"` is success; `"error"` or `"aborted"` is failure (exit 3)
- **Error** — `errorMessage` field (present when `stopReason == "error"`)

## Usage/cost mapping

| pi field | Jerry `runtime.Usage` field |
|---|---|
| `usage.input + usage.cacheRead + usage.cacheWrite` | `InputTokens` |
| `usage.output` | `OutputTokens` |
| `usage.cost.total` | `CostUSD` |

Jerry keeps no price tables. Cost is whatever pi reports.

## Structured output

pi does not natively support structured output schemas. When a step declares `outputs:`, Jerry appends a structured-output directive to the prompt asking the model to emit JSON, then parses the response text for the outermost `{...}` block. This is a prompt convention, not a guarantee — but it works reliably with current models.

## Fixtures and testing

The pi adapter's unit tests use committed JSONL fixtures (`internal/runtime/testdata/pi-*.jsonl`) rather than live API calls. The integration test (`//go:build integration`) requires a real pi installation and valid API key.
