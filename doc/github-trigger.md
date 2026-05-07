# GitHub Trigger

Use `kgh github run` when execution should be resolved from GitHub commit metadata instead of a direct `--target` flag.

## Trigger Syntax

The supported commit-message command is:

```text
submit: <target> [gpu=<bool>] [internet=<bool>]
```

Examples:

```text
submit: exp142
submit: exp142 gpu=false
submit: exp142 internet=true
submit: exp142 gpu=false internet=true
```

`kgh` supports one target per trigger. Malformed syntax or multiple commands are treated as errors.

## CLI Usage

```bash
kgh github run
```

This command resolves the target from GitHub context and then follows the same dry-run or live execution path as a local run.

## Environment

`kgh github run` is intended for GitHub Actions and relies on these environment variables:

- `GITHUB_EVENT_NAME`
- `GITHUB_EVENT_PATH`
- `GITHUB_SHA`
- `GITHUB_WORKSPACE`

The GitHub workflow wrapper is also expected to wire Kaggle credentials from GitHub Actions secrets for live execution:

- `KAGGLE_API_TOKEN`, or
- both `KAGGLE_USERNAME` and `KAGGLE_KEY`

For manual `workflow_dispatch` reruns, the GitHub workflow wrapper may also provide:

- `KGH_TRIGGER_SHA`
- `KGH_PULL_REQUEST_NUMBER`

If Kaggle credentials are present, the wrapper runs `kgh github run --dry-run=false`.
If Kaggle credentials are absent, the wrapper reports that live execution is unsafe and intentionally falls back to dry-run mode.

Use built-in help to inspect the current contract:

```bash
go run ./cmd/kgh github help
```

## When To Use It

- use `kgh run` for local debugging or explicit target execution
- use `kgh github run` when commit metadata is the source of truth for the target and runtime overrides
