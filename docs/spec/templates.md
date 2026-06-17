# Template grammar

Jerry uses `${{ }}` expressions in prompt files, ci step fields (`body`, `status`, `title`), and shell step `run` commands. Templates are resolved at execution time by `jerry exec` and statically validated at authoring time by `jerry validate`.

## Reference forms

| Reference | Example | Resolves to |
|---|---|---|
| `trigger.intent` | `${{ trigger.intent }}` | Normalized trigger intent string |
| `trigger.type` | `${{ trigger.type }}` | `pull_request`, `push`, `dispatch`, `schedule`, `manual` |
| `trigger.source` | `${{ trigger.source }}` | `github`, `gitlab`, `cli` |
| `trigger.raw.<path>` | `${{ trigger.raw.pull_request.number }}` | Dot-path into the raw JSON payload |
| `steps.<name>.output` | `${{ steps.review.output }}` | Step's final text output |
| `steps.<name>.outputs.<key>` | `${{ steps.review.outputs.verdict }}` | Typed structured output value |
| `steps.<name>.diff` | `${{ steps.implement.diff }}` | Unified diff patch of workspace changes |
| `steps.<name>.diff_stat` | `${{ steps.implement.diff_stat }}` | Short diff stat line |
| `run.id` | `${{ run.id }}` | Run identifier (context dir basename) |
| `run.cost` | `${{ run.cost }}` | Cumulative run cost in USD |
| `run.tokens` | `${{ run.tokens }}` | Cumulative token count |

## Static validation

`jerry validate` checks every `${{ }}` expression at authoring time:

- **Unknown step** — referencing a step name that doesn't exist in the workflow is an error, with a did-you-mean suggestion if the edit distance is close.
- **Forward reference** — referencing a step that runs *after* the current step is an error. Steps can only see prior outputs.
- **Undeclared output key** — `steps.review.outputs.verdict` requires `review` to declare `verdict` in its `outputs` map.

## Context assembly

The `context:` field on agent steps controls what is prepended to the prompt:

- **Omitted** (default) — trigger block (fenced as `<untrusted-trigger>`) + every prior step's text output, in order.
- **Explicit list** — only the named refs. Examples:
  - `context: ["trigger"]` — trigger block only, no prior step outputs.
  - `context: ["trigger", "steps.plan"]` — trigger + one specific step.
  - `context: ["steps.plan", "diff:implement"]` — a step's output + another step's diff.

## trigger.raw path grammar

The `trigger.raw.<path>` form uses a deliberately simple dot-path grammar: dot-separated segments with optional `[index]` for arrays. This is NOT JSONPath — no filters, no wildcards, no recursive descent.

```
trigger.raw.pull_request.title          → string
trigger.raw.pull_request.labels[0].name → string
```

The same grammar is used internally by custom adapter output parsing and the trigger raw lookup — one implementation, three call sites.

## Prompt injection mitigation

Trigger content (PR titles, ticket bodies, commit messages) is attacker-influenced text. When `jerry exec` assembles prompts, trigger data is always fenced in an `<untrusted-trigger>` block. This is a structural mitigation — it marks the boundary, not a guarantee. Pair it with read-only permissions on steps that process untrusted input.
