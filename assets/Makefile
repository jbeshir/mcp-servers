GOLANGCI_LINT_VERSION := v2.8.0

.PHONY: lint test-short build check-mocks validate
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run
test-short:
	go test -short ./...
build:
	go build ./cmd/assets-mcp
check-mocks:
	go tool mockery
	git diff --exit-code -- internal/assetcore/mocks
validate: lint test-short build check-mocks
	go test -race ./internal/...
