# MCP Servers workspace Makefile
# Validates and builds all MCP server modules in this workspace.

MODULES := workflowy manifold supermarkets-uk amazon

.PHONY: setup-tools
setup-tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# ── Validation ────────────────────────────────────────────────

.PHONY: validate
validate: lint test-short build

.PHONY: lint
lint:
	@for mod in $(MODULES); do \
		echo "==> Linting $$mod"; \
		golangci-lint run --config .golangci.yml ./$$mod/...; \
	done

.PHONY: test-short
test-short:
	@for mod in $(MODULES); do \
		echo "==> Testing $$mod"; \
		(cd $$mod && go test -v -short ./...); \
	done

.PHONY: test
test:
	@for mod in $(MODULES); do \
		echo "==> Testing $$mod"; \
		(cd $$mod && go test -v ./...); \
	done

.PHONY: fmt
fmt:
	@for mod in $(MODULES); do \
		echo "==> Formatting $$mod"; \
		(cd $$mod && go fmt ./...); \
	done
	goimports -w .

# ── Build ─────────────────────────────────────────────────────

.PHONY: build
build: build-workflowy build-manifold build-supermarkets-uk build-amazon

.PHONY: build-workflowy
build-workflowy:
	go build -o bin/workflowy-mcp ./workflowy/cmd/workflowy-mcp

.PHONY: build-manifold
build-manifold:
	go build -o bin/manifold-mcp ./manifold/cmd/manifold-mcp

.PHONY: build-supermarkets-uk
build-supermarkets-uk:
	go build -o bin/supermarkets-uk-mcp ./supermarkets-uk/cmd/supermarkets-uk-mcp

.PHONY: build-amazon
build-amazon:
	go build -o bin/amazon-mcp ./amazon/cmd/amazon-mcp

.PHONY: build-all-platforms
build-all-platforms: build-workflowy-all-platforms build-manifold-all-platforms build-supermarkets-uk-all-platforms build-amazon-all-platforms

.PHONY: build-workflowy-all-platforms
build-workflowy-all-platforms:
	GOOS=darwin GOARCH=amd64 go build -o bin/workflowy-mcp-darwin-amd64 ./workflowy/cmd/workflowy-mcp
	GOOS=darwin GOARCH=arm64 go build -o bin/workflowy-mcp-darwin-arm64 ./workflowy/cmd/workflowy-mcp
	GOOS=linux GOARCH=amd64 go build -o bin/workflowy-mcp-linux-amd64 ./workflowy/cmd/workflowy-mcp
	GOOS=windows GOARCH=amd64 go build -o bin/workflowy-mcp-windows-amd64.exe ./workflowy/cmd/workflowy-mcp

.PHONY: build-manifold-all-platforms
build-manifold-all-platforms:
	GOOS=darwin GOARCH=amd64 go build -o bin/manifold-mcp-darwin-amd64 ./manifold/cmd/manifold-mcp
	GOOS=darwin GOARCH=arm64 go build -o bin/manifold-mcp-darwin-arm64 ./manifold/cmd/manifold-mcp
	GOOS=linux GOARCH=amd64 go build -o bin/manifold-mcp-linux-amd64 ./manifold/cmd/manifold-mcp
	GOOS=windows GOARCH=amd64 go build -o bin/manifold-mcp-windows-amd64.exe ./manifold/cmd/manifold-mcp

.PHONY: build-supermarkets-uk-all-platforms
build-supermarkets-uk-all-platforms:
	GOOS=darwin GOARCH=amd64 go build -o bin/supermarkets-uk-mcp-darwin-amd64 ./supermarkets-uk/cmd/supermarkets-uk-mcp
	GOOS=darwin GOARCH=arm64 go build -o bin/supermarkets-uk-mcp-darwin-arm64 ./supermarkets-uk/cmd/supermarkets-uk-mcp
	GOOS=linux GOARCH=amd64 go build -o bin/supermarkets-uk-mcp-linux-amd64 ./supermarkets-uk/cmd/supermarkets-uk-mcp
	GOOS=windows GOARCH=amd64 go build -o bin/supermarkets-uk-mcp-windows-amd64.exe ./supermarkets-uk/cmd/supermarkets-uk-mcp

.PHONY: build-amazon-all-platforms
build-amazon-all-platforms:
	GOOS=darwin GOARCH=amd64 go build -o bin/amazon-mcp-darwin-amd64 ./amazon/cmd/amazon-mcp
	GOOS=darwin GOARCH=arm64 go build -o bin/amazon-mcp-darwin-arm64 ./amazon/cmd/amazon-mcp
	GOOS=linux GOARCH=amd64 go build -o bin/amazon-mcp-linux-amd64 ./amazon/cmd/amazon-mcp
	GOOS=windows GOARCH=amd64 go build -o bin/amazon-mcp-windows-amd64.exe ./amazon/cmd/amazon-mcp

# ── Clean ─────────────────────────────────────────────────────

.PHONY: clean
clean:
	rm -rf bin/
