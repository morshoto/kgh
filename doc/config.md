# Config

`kgh` reads repository targets from `.kgh/config.yaml`.

## Example

```yaml
targets:
  issue7-e2e:
    notebook: notebooks/issue7-e2e.ipynb
    kernel_id: yourname/issue7-e2e
    competition: playground-series-s6e2
    submit: true
    resources:
      gpu: false
      internet: false
      private: false
    sources:
      competition_sources:
        - playground-series-s6e2
      dataset_sources:
        - yourname/feature-pack-v3
    outputs:
      submission: submission.csv
      metrics: metrics.json
```

## Fields

- `targets`: map of target names to runnable Kaggle workflows
- `notebook`: notebook file that `kgh` stages into the Kaggle bundle
- `kernel_id`: Kaggle kernel identifier in `<owner>/<slug>` form
- `competition`: Kaggle competition slug used for kernel metadata and optional submission
- `submit`: whether a successful live run should submit the configured output file
- `resources.gpu`: default GPU setting for the target
- `resources.internet`: default internet access setting for the target
- `resources.private`: default Kaggle kernel privacy setting
- `sources.competition_sources`: Kaggle competition sources attached to the kernel metadata
- `sources.dataset_sources`: Kaggle dataset sources attached to the kernel metadata
- `outputs.submission`: expected submission file name in downloaded outputs
- `outputs.metrics`: expected metrics file name in downloaded outputs

## Behavior Notes

- `.kgh/config.yaml` is the default config path used by both local and GitHub-triggered execution.
- `kgh run --target <name>` fails if the target does not exist or required fields are missing.
- Dry-run output shows the fully resolved execution spec, including defaulted arrays and override state.
- Runtime overrides can change `gpu` and `internet` at execution time without editing the config file.

## Next Step

Use [Local Run](./local-run.md) to execute a target locally, or [GitHub Trigger](./github-trigger.md) to resolve targets from commit metadata.
