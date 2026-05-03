# Setup

This guide covers the environment needed to use `kgh` successfully.

## Requirements

- Go 1.25.x if you want to build from source
- Kaggle CLI for live Kaggle execution
- Kaggle credentials configured for the account that will run kernels and submissions
- access to the competitions and datasets referenced by your target config

Dry-run mode does not require Kaggle credentials. Live runs do.

## Install Paths

### Build from source

```bash
go build ./cmd/kgh
./kgh version
```

### Run without installing

```bash
go run ./cmd/kgh help
```

### Prebuilt binaries

Use a release artifact from GitHub Releases if you want a prebuilt binary.

### Nix

```bash
nix develop
nix build
nix flake check
```

The Nix dev shell includes Go, Python, and the Kaggle CLI. Kaggle-authenticated smoke or live flows still depend on your own credentials.

## First-Time Setup

1. Create `.kgh/config.yaml` from `.kgh/config_example.yaml`.
2. Edit at least one target so it points at a real notebook and Kaggle kernel id.
3. Run a dry-run locally before attempting live execution.

```bash
cp .kgh/config_example.yaml .kgh/config.yaml
go run ./cmd/kgh run --target issue7-e2e
```

Continue with [Config](./config.md) for target structure and [Local Run](./local-run.md) for execution behavior.
