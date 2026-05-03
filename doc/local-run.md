# Local Run

Use `kgh run` when you want to resolve and execute a configured target from your local checkout.

## Basic Usage

```bash
kgh run --target exp142
```

The `--target` flag is required.

By default, `kgh run` uses `--dry-run=true`, so the command prints resolved JSON and does not invoke Kaggle.

## Dry-Run and Live Mode

Resolve a target without executing it:

```bash
go run ./cmd/kgh run --target issue7-e2e
```

Run the live Kaggle workflow:

```bash
go run ./cmd/kgh run --target issue7-e2e --dry-run=false
```

Live mode performs the Kaggle-facing workflow: bundle, push, poll, download outputs, and optionally submit.

## Runtime Overrides

You can override selected runtime settings without editing `.kgh/config.yaml`.

```bash
kgh run --target exp142 --gpu=false
kgh run --target exp142 --internet=true
```

Supported overrides:

- `--gpu=true|false`
- `--internet=true|false`

## Polling Controls

Use polling flags when you want non-default status timing for live runs.

```bash
kgh run --target exp142 --poll-interval=2s --timeout=5m
```

- `--poll-interval`: live Kaggle status polling interval
- `--timeout`: overall live polling timeout

## Output Expectations

Dry-run returns the resolved execution contract as JSON.

Live mode returns a richer JSON report that can include:

- resolved execution metadata
- staged bundle information
- Kaggle push details
- poll status and terminal state
- downloaded output paths
- submission result when `submit: true`

When `submit: true` succeeds, the live JSON includes structured submission metadata under
`submission` and `score`, including `submission_id`, `status`, and `submitted_at`.
