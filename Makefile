GOLANGCI_LINT_VERSION := v2.12.2
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

.PHONY: all build test coverage lint fmt check-fmt generate tidy clean check ci

all: build check

build:
	go build ./...

test:
	go test -v -race -timeout 30s ./...

coverage:
	go test -v -covermode=count -coverpkg=./... -coverprofile=coverage.out -timeout 30s ./...
	grep -v '/internal/generator/sandbox/' coverage.out > coverage.filtered && mv coverage.filtered coverage.out
	go tool cover -func=coverage.out

$(GOLANGCI_LINT):
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

fmt:
	gofmt -l -w .

check-fmt:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needs to be run on the following files:"; gofmt -l .; exit 1)

generate:
	go generate ./...

tidy:
	go mod tidy

clean:
	rm -f coverage.out

check: test lint check-fmt

ci: check coverage
