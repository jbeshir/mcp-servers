# CLAUDE.md - mcp-servers (Go workspace)

## Project Structure

This is a Go workspace containing multiple MCP (Model Context Protocol) servers. Each server lives in its own module subdirectory and is listed in `go.work`.

## Validation

To validate changes in this repository, run the following automated checks:

- Linting: `make lint`
- Tests: `make test-short`
- Build all servers: `make build`

All automated steps must pass before changes can be merged.

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

4. **Validation Script Coverage**: Ensure any new validations are:
   - Added to CI

5. **Redundant fields or parameters**: Look for:
   - Fields that contain information present in or inferrable from other fields
   - Parameters that contain information present in or inferrable from other fields

6. **Poor use of types**: Look for:
   - String comparisons on errors instead of using typed errors and errors.Is or errors.As

All steps must pass before changes can be merged.
