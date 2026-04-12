.PHONY: test build fmt

test:
	go test ./...

build:
	go build ./cmd/kh

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')
