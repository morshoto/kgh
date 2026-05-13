# Competition Submission Loop

This recipe shows a small but realistic target for a Kaggle competition workflow where you want one reproducible local entrypoint for dry-run inspection, live execution, and optional submission.

## When to Use It

Use this pattern when:

- your repository contains a single notebook-based experiment you want to run repeatedly
- you need fixed competition and dataset attachments
- you want the same target name to work both locally and from GitHub-triggered automation

## Example Target

```yaml
targets:
  titanic-baseline:
    notebook: notebooks/titanic_baseline.ipynb
    kernel_id: yourname/titanic-baseline
    competition: titanic
    submit: true
    resources:
      gpu: false
      internet: false
      private: false
    sources:
      competition_sources:
        - titanic
      dataset_sources:
        - yourname/titanic-feature-pack
    outputs:
      submission: submission.csv
      metrics: metrics.json
```

## Workflow

1. Add the target to `.kgh/config.yaml`.
2. Verify the notebook path and Kaggle identifiers match what exists in your repository and account.
3. Resolve the target locally in dry-run mode.
4. Switch to live mode once the resolved JSON looks correct.

## Commands

Resolve the target without invoking Kaggle:

```bash
go run ./cmd/kgh run --target titanic-baseline
```

Run the full Kaggle workflow and submit if the configured output is produced:

```bash
go run ./cmd/kgh run --target titanic-baseline --dry-run=false
```

Temporarily override runtime settings without editing the target:

```bash
go run ./cmd/kgh run --target titanic-baseline --internet=true
go run ./cmd/kgh run --target titanic-baseline --gpu=false
```

## Notes

- Keep `kernel_id` stable so repeated runs map to the same Kaggle kernel lineage.
- Prefer `submit: true` only when the notebook reliably writes the configured `outputs.submission` file.
- Store reusable feature inputs as Kaggle datasets when the notebook depends on artifacts outside the repository.
- Start from dry-run whenever you change config shape, notebook path, or source attachments.
