<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25.0-00ADD8?logo=go&logoColor=white" alt="Go 1.25.0" />
  <a href="https://github.com/morshoto/kgh/releases"><img src="https://img.shields.io/github/downloads/morshoto/kgh/total?label=downloads&logo=github" alt="GitHub release downloads total" /></a>
  <img src="https://github.com/morshoto/kgh/actions/workflows/go-ci.yml/badge.svg?branch=main" alt="Go CI" />
  <a href="https://deepwiki.com/morshoto/kgh"><img src="https://img.shields.io/badge/DeepWiki-morshoto%2Fkgh-blue.svg?logo=data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACwAAAAyCAYAAAAnWDnqAAAAAXNSR0IArs4c6QAAA05JREFUaEPtmUtyEzEQhtWTQyQLHNak2AB7ZnyXZMEjXMGeK/AIi+QuHrMnbChYY7MIh8g01fJoopFb0uhhEqqcbWTp06/uv1saEDv4O3n3dV60RfP947Mm9/SQc0ICFQgzfc4CYZoTPAswgSJCCUJUnAAoRHOAUOcATwbmVLWdGoH//PB8mnKqScAhsD0kYP3j/Yt5LPQe2KvcXmGvRHcDnpxfL2zOYJ1mFwrryWTz0advv1Ut4CJgf5uhDuDj5eUcAUoahrdY/56ebRWeraTjMt/00Sh3UDtjgHtQNHwcRGOC98BJEAEymycmYcWwOprTgcB6VZ5JK5TAJ+fXGLBm3FDAmn6oPPjR4rKCAoJCal2eAiQp2x0vxTPB3ALO2CRkwmDy5WohzBDwSEFKRwPbknEggCPB/imwrycgxX2NzoMCHhPkDwqYMr9tRcP5qNrMZHkVnOjRMWwLCcr8ohBVb1OMjxLwGCvjTikrsBOiA6fNyCrm8V1rP93iVPpwaE+gO0SsWmPiXB+jikdf6SizrT5qKasx5j8ABbHpFTx+vFXp9EnYQmLx02h1QTTrl6eDqxLnGjporxl3NL3agEvXdT0WmEost648sQOYAeJS9Q7bfUVoMGnjo4AZdUMQku50McDcMWcBPvr0SzbTAFDfvJqwLzgxwATnCgnp4wDl6Aa+Ax283gghmj+vj7feE2KBBRMW3FzOpLOADl0Isb5587h/U4gGvkt5v60Z1VLG8BhYjbzRwyQZemwAd6cCR5/XFWLYZRIMpX39AR0tjaGGiGzLVyhse5C9RKC6ai42ppWPKiBagOvaYk8lO7DajerabOZP46Lby5wKjw1HCRx7p9sVMOWGzb/vA1hwiWc6jm3MvQDTogQkiqIhJV0nBQBTU+3okKCFDy9WwferkHjtxib7t3xIUQtHxnIwtx4mpg26/HfwVNVDb4oI9RHmx5WGelRVlrtiw43zboCLaxv46AZeB3IlTkwouebTr1y2NjSpHz68WNFjHvupy3q8TFn3Hos2IAk4Ju5dCo8B3wP7VPr/FGaKiG+T+v+TQqIrOqMTL1VdWV1DdmcbO8KXBz6esmYWYKPwDL5b5FA1a0hwapHiom0r/cKaoqr+27/XcrS5UwSMbQAAAABJRU5ErkJggg==" alt="DeepWiki"></a>
  <img src="https://img.shields.io/github/license/morshoto/kgh" alt="License" />
</p>

# kgh

`kgh` is a GitHub-native tool for Kaggle workflows.

### Current status

This repository currently contains the Milestone 1 project skeleton:

- Go CLI entrypoint at `cmd/kgh`
- Internal package layout for future implementation
- Basic Go CI in GitHub Actions

### Quick start

```bash
go test ./...
go build ./cmd/kgh
./kgh version
```

The primary CI path is the GitHub commit trigger. In GitHub Actions, `kgh` reads the checked-out head commit message and looks for exactly one command in this form:

```text
submit: <target> [gpu=<bool>] [internet=<bool>]
```

The CLI entrypoint for that path is:

```bash
./kgh github run
./kgh github run --dry-run=false
```

For local debugging, keep using the explicit target path. Create `.kgh/config.yaml` from `.kgh/config_example.yaml` and then:

```bash
./kgh run --target issue7-e2e
./kgh run --target issue7-e2e --dry-run=false
```

`kgh run` defaults to `--dry-run=true`. Live runs poll Kaggle every `5s` for up to `30m`.

```bash
./kgh run --target issue7-e2e --dry-run=false --poll-interval=2s --timeout=5m
```

The repository includes a PR workflow at [`.github/workflows/commit-trigger.yml`](.github/workflows/commit-trigger.yml) that checks the head commit for `submit:` and invokes `kgh github run` only when a trigger is present.

### Issue 7 live verification

Use the committed smoke fixture for repeatable Kaggle verification:

- target config example: `.kgh/config.issue7_example.yaml`
- notebook fixture: `notebooks/issue7-e2e.ipynb`
- verified kernel ref: `bloodymonday/issue7-e2e`

Keep `.kgh/config.yaml` local-only. Copy one of the example configs and adjust it for your Kaggle account before running live commands.

Credential setup:

```bash
export KAGGLE_USERNAME=<username>
export KAGGLE_KEY=<api-key>
```

Verification commands:

```bash
go run ./cmd/kgh/main.go kgh run --target issue7-e2e --dry-run
go run ./cmd/kgh/main.go kgh run --target issue7-e2e --dry-run=false --poll-interval=2s --timeout=5m
go run ./cmd/kgh/main.go kgh github run
kaggle kernels status bloodymonday/issue7-e2e
```

Manual fallback:

```bash
kaggle kernels push -p <bundle-dir>
kaggle kernels status <owner>/<kernel-slug>
```

### Kaggle smoke test

Use the live smoke path only when you need to validate the adapter against real Kaggle credentials and the Kaggle CLI.
It is intentionally opt-in and is not part of default CI.

```bash
export KGH_KAGGLE_SMOKE=1
export KGH_KAGGLE_SMOKE_COMPETITION=<competition-slug>
export KAGGLE_API_TOKEN=<token>
# or:
# export KAGGLE_USERNAME=<username>
# export KAGGLE_KEY=<key>

make smoke-kaggle
```

The smoke test runs a read-only submissions listing through the Kaggle adapter. Use a competition you have access to and have already joined. If you prefer not to use `make`, the equivalent command is:

```bash
KGH_KAGGLE_SMOKE=1 KGH_KAGGLE_SMOKE_COMPETITION=<competition-slug> go test -tags smoke ./internal/kaggle -run '^TestSmokeKaggleAdapterLive$' -count=1
```
