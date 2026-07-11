# assets-mcp

An [MCP](https://modelcontextprotocol.io/) server that provides offline access to vendored design assets — icons, illustrations, and fonts — with license and attribution metadata. All asset data is embedded in the binary at build time; the server makes no network calls at runtime. The server communicates over stdio.

## Getting Started

### Requirements

None. The server is fully offline and requires no API keys or tokens.

### Configuration

| Variable | Required | Description |
|---|---|---|
| `ASSETS_OUTPUT_DIR` | No | Directory rendered assets are written to (default: `<OS temp dir>/assets-mcp`) |

### Install from source

Requires Go 1.24+.

```
go install github.com/jbeshir/mcp-servers/assets/cmd/assets-mcp@latest
```

### Docker

```
docker build -t assets-mcp ./assets
docker run assets-mcp
```

### Claude Desktop

Add to your Claude Desktop configuration (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "assets": {
      "command": "/path/to/assets-mcp"
    }
  }
}
```

### Claude Code

```
claude mcp add assets -- /path/to/assets-mcp
```

## Tools

Assets are identified by a **composite id** of the form `<provider>:<local>` — e.g. `embedded-icons:lucide/camera`, `embedded-illustrations:open-doodles/coffee-doodle`, `embedded-fonts:inter`. The `search_*` tools return each hit's composite id; pass it to the matching `get_*` tool, or construct one directly. The `local` part is opaque to everyone but the emitting provider.

| Tool | Arguments | Description |
|---|---|---|
| `list_asset_sources` | `kind?`, `providers?`, `exclude_providers?`, `sources?`, `exclude_sources?` | List registered providers and the upstream sources each serves, with license and item count, as a readable listing plus a structured JSON block |
| `search_icons` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?` | Search vendored icon sets by name; returns composite ids |
| `get_icon` | `id`, `color?`, `size?` | Render an icon to a standalone SVG file (by composite id) and write it to disk |
| `search_illustrations` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?` | Search vendored illustration collections by name; returns composite ids |
| `get_illustration` | `id` | Fetch an SVG illustration (by composite id) and write it to disk, unmodified |
| `search_fonts` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?` | Search vendored font families by name, slug, or category; returns composite ids |
| `get_font` | `id`, `weight?`, `style?`, `format?` | Fetch a font's woff2 file (and optionally an `@font-face` CSS snippet) by composite id and write it to disk |

The `sources`/`providers`/`exclude_*` arguments are string arrays. `providers`/`exclude_providers` currently only match the offline `embedded-*` providers.

## Offline Posture

Every icon, illustration, and font is vendored into the binary via Go's `embed` package at build time (`internal/providers/embedded{icons,illustrations,fonts}/data/`). Each provider owns its own license metadata and derives item counts from the embedded data it actually carries, so nothing can drift out of sync. The server never makes an outbound network request — `search_*` and `get_*` tools resolve entirely against embedded data.

## Return Contract

Tools that produce files (`get_icon`, `get_illustration`, `get_font`) write the asset(s) to disk under `ASSETS_OUTPUT_DIR` (or the default temp directory) and return:

1. A human-readable summary **text content block** of what was written.
2. A native **`structuredContent`** object shaped `{"files":[{"path","kind","source","license","attribution"}],"count":N}` (with `count == len(files)`).

This mirrors the structured-output shape used elsewhere in this monorepo, e.g. `image-gen-mcp`'s structured `image_url` result.

## Licenses

Every asset carries a license and (where applicable) attribution, retrievable via `list_asset_sources` or the `license`/`attribution` fields returned by `get_icon`/`get_illustration`/`get_font`.

**Icons:**

| Set | License |
|---|---|
| Lucide | ISC |
| Material Symbols | Apache-2.0 |
| Simple Icons | CC0-1.0 (vector data only — see trademark note below) |
| Bootstrap Icons, Feather, Heroicons, Phosphor, Tabler | MIT |

> **Simple Icons trademark note:** the CC0-1.0 license covers the vector artwork, not the brand marks it depicts. Brand logos are third-party trademarks of their respective owners — using them must not imply sponsorship or endorsement.

**Illustrations:** all three collections (Open Doodles, Humaaans, Open Peeps) are CC0-1.0.

**Fonts:** all fourteen families are OFL-1.1. Each family's `LICENSE` file travels alongside its woff2 files in `internal/fonts/data/<family>/` and is not re-served by the MCP server (`get_font`'s `attribution` field is empty for this reason — the license text itself is bundled in the repo).

## Follow-ups

- **2D game art (Kenney).** Kenney's CC0 game-art packs (sprites, tilesets, UI packs) are a natural fourth asset domain but are not included in this initial version; adding them is a follow-up.
- **demesne filegen wiring.** Sandboxed demesne environments route file-generating MCP tools through `/workspace/generated/`. This server currently writes directly to `ASSETS_OUTPUT_DIR`; wiring it into that convention is a follow-up.
