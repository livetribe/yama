GOLANGCI_LINT_VERSION := v2.12.2
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

.PHONY: all build test coverage lint fmt check-fmt generate tidy clean check ci

all: build check

build:
	go build ./...

test:
	go test -v -race -timeout 5m ./...

coverage:
	go test -v -covermode=count -coverpkg=./... -coverprofile=coverage.out -timeout 5m ./...
	grep -v '/internal/generator/sandbox/' coverage.out > coverage.filtered && mv coverage.filtered coverage.out
	go tool cover -func=coverage.out

$(GOLANGCI_LINT):
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

check-fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt --diff

generate:
	go generate ./...

tidy:
	go mod tidy

clean: $(GOLANGCI_LINT)
	rm -f coverage.out
	$(GOLANGCI_LINT) cache clean
	go clean -testcache

check: test lint check-fmt

ci: check coverage
