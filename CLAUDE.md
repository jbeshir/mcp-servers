# CLAUDE.md - mcp-servers (Go workspace)

## Project Structure

This is a Go workspace containing multiple MCP (Model Context Protocol) servers. Each server lives in its own module subdirectory and is listed in `go.work`.

## Validation

To validate changes in this repository, run the following automated checks:

- Linting: `make lint`
- Tests: `make test-short`
- Build all servers: `make build`

All automated steps must pass before changes can be merged.

For changes to datasource scraping/parsing logic, also run live integration tests:

- Integration tests: `make test` (omits `-short` — hits live supermarket sites, may be flaky)

Additionally, perform global manual quality checks, plus these project-specific checks:

- **Environment variables**: Check that env vars referenced in code are documented in the server's README.
- **MCP tool definitions in sync**: When changing tools, check all three locations are consistent:
  - `internal/server/tools.go` — authoritative tool registration (descriptions, parameters, options)
  - `manifest.json` — tool list for MCPB distribution (every server must have one)
  - `README.md` — user-facing tool documentation table
- **server.json descriptions**: Must be ≤100 characters (MCP registry rejects longer ones).

All steps must pass before changes can be merged.

## Desloppify

Subjective LLM-based code review complementing golangci-lint. Local-only (not in CI).

- `make desloppify` — scan for quality issues
- `desloppify next` — show priority fix queue
- `.desloppify/` is committed for state persistence across sessions
- Exclude worktree directories when created: `desloppify exclude <worktree-dir>/`

## Coding Standards

### Datasource Constructors
- Per-store wrappers (e.g. `NewHiyou`) are thin: fill in the store-specific `Config` and delegate to the generic constructor

## Supermarkets-UK

- Use `go run ./supermarkets-uk/cmd/capture-html` to fetch live supermarket HTML for debugging selectors and test fixture updates (supports `-store`, `-query`, `-url`, `-wait` flags).
