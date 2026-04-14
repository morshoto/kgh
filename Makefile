.PHONY: test build fmt smoke-kaggle

test:
	go test ./...

build:
	go build ./cmd/kh

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')

smoke-kaggle:
	@if [ -z "$$KGH_KAGGLE_SMOKE_COMPETITION" ]; then echo "KGH_KAGGLE_SMOKE_COMPETITION is required"; exit 1; fi
	KGH_KAGGLE_SMOKE=1 go test -tags smoke ./internal/kaggle -run '^TestSmokeKaggleAdapterLive$$' -count=1
