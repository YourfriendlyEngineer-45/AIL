.PHONY: test build examples

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -o ail ./cmd/ail

examples: build
	./scripts/run_examples.sh
