BINARY := arch

.PHONY: build test smoke-local run fmt lint

build:
	go build ./...

test:
	go test ./...

smoke-local:
	./scripts/smoke-local.sh

run:
	go run ./cmd/$(BINARY)

fmt:
	gofmt -w ./cmd ./internal ./pkg

lint:
	go vet ./...
