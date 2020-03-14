.DEFAULT_GOAL := help

.PHONY: setup
setup:  ## Setup for required tools.
	go get github.com/golang/lint/golint
	go get golang.org/x/tools/cmd/goimports
	go get golang.org/x/tools/cmd/stringer
	go get -u github.com/golang/dep/cmd/dep
	dep ensure

.PHONY: fmt
fmt: ## Formatting source codes.
	@goimports -w .

.PHONY: lint
lint: ## Run golint and go vet.
	@golint .
	@go vet .

.PHONY: test
test:  ## Run the tests.
	@go test .

.PHONY: coverage
cover:  ## Run the tests.
	@go test -coverprofile=coverage.o
	@go tool cover -func=coverage.o

.PHONY: race-test
race-test:  ## Checking the race condition.
	@go test -race .

.PHONY: help
help: ## Show help text
	@echo "Commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
