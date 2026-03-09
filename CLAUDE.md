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

Additionally, perform the following manual checks:

### Manual Quality Checks

After automated validation passes, review code for these issues:

1. **Unnecessarily Optional Fields**: Check for dependencies being defensively allowed to be optional, and simplify by assuming we always provide them.

2. **Duplicate Code**: Check for repeated definitions (constants, types, helper functions) that should be consolidated.

3. **Incomplete or Disconnected Logic**: Look for:
   - Legacy mechanisms or functions left in the code instead of being removed
   - Closures closing over data that may be outdated
   - Fields in types/interfaces that are never read or written
   - Functions that are defined but never called
   - Features partially implemented but not wired up
   - TODO comments indicating unfinished work
   - Environment variables referenced in code but missing from README documentation

4. **Validation Script Coverage**: Ensure any new validations are:
   - Added to CI

5. **Redundant fields or parameters**: Look for:
   - Fields that contain information present in or inferrable from other fields
   - Parameters that contain information present in or inferrable from other fields

6. **Poor use of types**: Look for:
   - String comparisons on errors instead of using typed errors and errors.Is or errors.As

7. **MCP tool definitions out of sync**: When adding or changing tool parameters, descriptions, or behaviour, check all three locations are consistent:
   - `internal/server/tools.go` — authoritative tool registration (descriptions, parameters, options)
   - `manifest.json` — tool list for MCPB distribution (every server must have one)
   - `README.md` — user-facing tool documentation table

All steps must pass before changes can be merged.

## Coding Standards

### Naming
- Names should make sense from the current code alone — don't encode history (e.g. no `SearchEnhanced` because the old `Search` was removed)

### General
- Prefer immutable data flows — return results rather than mutating state on receivers (e.g. return a value from a method rather than accumulating fields on a struct)

### Datasource Constructors
- Single `New` constructor taking `(Config, *http.Client)` — no `NewWithURL`/`NewWithClient` variants
- `Config` struct holds optional overrides (e.g. `BaseURL`); zero value uses built-in defaults
- Per-store wrappers (e.g. `NewHiyou`) are thin: fill in the store-specific `Config` and delegate to the generic constructor

### URL Handling
- Store `baseURL` in config, compute full URLs inline in methods — don't precompute URL functions or store derived URL fields

## Supermarkets-UK

- Use `go run ./supermarkets-uk/cmd/capture-html` to fetch live supermarket HTML for debugging selectors and test fixture updates (supports `-store`, `-query`, `-url`, `-wait` flags).
