# Contributing

Contributions are welcome! The preferred process is to open a pull request.

## Getting started

1. Fork the repo and clone your fork
2. Install Go 1.24+ and run `make setup-tools` to install linting/formatting tools
3. Make your changes
4. Run `make validate` to check lint, tests, and build
5. Run `make fmt` to auto-format
6. Open a PR against `main`

## What CI checks

Every PR is automatically checked for:

- **Lint** — `golangci-lint` across all modules
- **Format** — `goimports` formatting
- **Tests** — `go test -short` across all modules
- **Build** — all servers compile for linux/amd64

All checks must pass before a PR can be merged.

## Project structure

This is a Go workspace with independent modules:

| Directory | Server |
|---|---|
| `workflowy/` | Workflowy MCP server |
| `manifold/` | Manifold Markets MCP server |
| `supermarkets-uk/` | UK Supermarkets MCP server |
| `amazon-products/` | Amazon Products MCP server |

Each module has its own `go.mod` and builds independently. The entry point for each server is `<module>/cmd/<module>-mcp/main.go`.

## Running tests

- `make test-short` — unit tests only (fast, no network)
- `make test` — all tests including integration tests (requires network and may require credentials)

## Adding a new supermarket

If you're adding a new supermarket data source to `supermarkets-uk`:

1. Create a new package under `supermarkets-uk/internal/datasource/`
2. Implement the `datasource.Source` interface
3. Register it in the client
4. Add test fixtures with captured HTML/JSON under `testdata/`
5. Update the README tables (supported supermarkets, product information)
