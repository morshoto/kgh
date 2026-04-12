# kgh

`kgh` is a GitHub-native tool for Kaggle workflows.

## Current status

This repository currently contains the Milestone 1 project skeleton:

- Go CLI entrypoint at `cmd/kgh`
- Internal package layout for future implementation
- Basic Go CI in GitHub Actions

## Quick start

```bash
go test ./...
go build ./cmd/kgh
./kgh version
```
