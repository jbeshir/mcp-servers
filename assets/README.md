# assets-mcp

An [MCP](https://modelcontextprotocol.io/) server that provides access to design assets — icons, illustrations, fonts, photos, textures, 3D models, and audio — with license and attribution metadata. A vendored offline base (icons, illustrations, fonts) is embedded in the binary at build time and always available; by default the server additionally queries four keyless remote APIs for a wider catalogue (see [Remote providers](#remote-providers)) — set `ASSETS_DISABLE_REMOTE=1` to run fully offline. Seven further opt-in keyed providers (see [Keyed providers](#keyed-providers)) register only when their API key or flag is configured. The server communicates over stdio.

## Getting Started

### Requirements

None. No API keys or tokens are required for any provider, embedded or remote. Configuring an API key/flag for one of the [keyed providers](#keyed-providers) is optional and only unlocks that provider.

### Configuration

| Variable | Required | Description |
|---|---|---|
| `ASSETS_OUTPUT_DIR` | No | Directory rendered assets are written to (default: `<OS temp dir>/assets-mcp`) |
| `ASSETS_DISABLE_REMOTE` | No | Set to `1` (or any non-empty value) to run fully offline with only the embedded providers |
| `ASSETS_CACHE_DIR` | No | On-disk cache directory for remote fetches (default: `<OS cache dir>/assets-mcp`, falling back to the OS temp dir) |
| `ASSETS_UNSPLASH_ACCESS_KEY` | No | Unsplash Access Key — when set, enables the opt-in Unsplash photo provider |
| `ASSETS_PIXABAY_KEY` | No | Pixabay API key — when set, enables the opt-in Pixabay photo provider |
| `ASSETS_PEXELS_KEY` | No | Pexels API key — when set, enables the opt-in Pexels photo provider |
| `ASSETS_POLYPIZZA_KEY` | No | Poly Pizza API key — when set, enables the opt-in Poly Pizza 3D model provider |
| `ASSETS_POLYHAVEN_ENABLE` | No | Set to `1` to enable the opt-in Poly Haven model provider (CC0 assets, but its API terms are non-commercial) |
| `ASSETS_JAMENDO_CLIENT_ID` | No | Jamendo client id — when set, enables the opt-in Jamendo audio provider |
| `ASSETS_FREESOUND_API_KEY` | No | Freesound API key — when set, enables the opt-in Freesound audio provider |

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

Assets are identified by a **composite id** of the form `<provider>:<local>` — e.g. `embedded-icons:lucide/camera`, `embedded-illustrations:open-doodles/coffee-doodle`, `embedded-fonts:inter`, or, for the remote providers, `iconify:lucide/home`, `googlefonts:roboto`, `openverse:<uuid>`, `ambientcg:Bricks097`, and, for the keyed providers, `unsplash:<id>`, `pixabay:<id>`, `pexels:<id>`, `polypizza:<id>`, `polyhaven:<slug>`, `jamendo:<trackid>`, `freesound:<soundid>`. The `search_*` tools return each hit's composite id; pass it to the matching `get_*` tool, or construct one directly. The `local` part is opaque to everyone but the emitting provider.

| Tool | Arguments | Description |
|---|---|---|
| `list_asset_sources` | `kind?`, `providers?`, `exclude_providers?`, `sources?`, `exclude_sources?` | List registered providers and the upstream sources each serves, with license and item count, as a readable listing plus a structured JSON block |
| `search_icons` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?`, `cursor?` | Search icon sets (embedded + Iconify) by name; returns composite ids |
| `get_icon` | `id`, `color?`, `size?` | Render an icon to a standalone SVG file (by composite id) and write it to disk |
| `search_illustrations` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?`, `cursor?` | Search vendored illustration collections by name; returns composite ids |
| `get_illustration` | `id` | Fetch an SVG illustration (by composite id) and write it to disk, unmodified |
| `search_fonts` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?`, `cursor?` | Search font families (embedded + Google Fonts) by name, slug, or category; returns composite ids |
| `get_font` | `id`, `weight?`, `style?`, `format?` | Fetch a font's woff2 file (and optionally an `@font-face` CSS snippet) by composite id and write it to disk |
| `search_photos` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?`, `cursor?` | Search keyless Openverse CC-licensed photos by name; returns composite ids |
| `get_photo` | `id` | Fetch a photo (by composite id) and write it to disk |
| `search_textures` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?`, `cursor?` | Search keyless CC0 ambientCG PBR material sets by name; returns composite ids |
| `get_texture` | `id`, `resolution?`, `format?` | Fetch a PBR material archive (a zip of texture maps) by composite id and write it to disk |
| `search_models` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?`, `cursor?` | Search 3D model providers (Poly Pizza, Poly Haven) by name; returns composite ids |
| `get_model` | `id`, `format?`, `resolution?` | Fetch a 3D model (by composite id) and write it to disk — a .glb, or a zip of a glTF and its assets |
| `search_audio` | `query`, `sources?`, `exclude_sources?`, `providers?`, `exclude_providers?`, `limit?`, `cursor?` | Search audio providers (Jamendo, Freesound) by name; returns composite ids |
| `get_audio` | `id`, `format?` | Fetch an audio clip (mp3 or ogg) by composite id and write it to disk |

The `sources`/`providers`/`exclude_*` arguments are string arrays. Every `search_*` tool accepts an optional `cursor` (an opaque pagination token from a previous call's `next_cursor`) and, when more results remain, returns a `next_cursor` to pass back for the following page.

## Remote providers

By default, four keyless and anonymous remote APIs are queried alongside the embedded offline base, additively — the embedded providers remain the always-present offline base, and the remote providers extend the catalogue without requiring any API keys:

- **Iconify** — icons, spanning essentially every open icon set Iconify hosts
- **Google Fonts** — fonts, the full Google Fonts catalogue
- **Openverse** — photos, openly (CC) licensed images
- **ambientCG** — textures, CC0 PBR material archives

Set `ASSETS_DISABLE_REMOTE=1` to disable all four and run fully offline against only the embedded providers. Because results from embedded and remote providers for the same kind are merged and paginated together, cross-page deduplication of near-identical hits is best-effort, not guaranteed.

## Keyed providers

Seven further remote providers are opt-in: each is OFF by default and registers only when its API key or flag is configured (see [Configuration](#configuration)), leaving the free keyless providers above as the default. They add photos, 3D models, and audio:

- **Unsplash** (`ASSETS_UNSPLASH_ACCESS_KEY`) — photos under the Unsplash License; credit is required. `get_photo` fires the ToS-mandated download-tracking request before returning bytes.
- **Pixabay** (`ASSETS_PIXABAY_KEY`) — photos under the Pixabay Content License; credit is appreciated, not required. Images are downloaded and cached rather than hotlinked, per Pixabay's ToS.
- **Pexels** (`ASSETS_PEXELS_KEY`) — photos under the Pexels License; credit is required.
- **Poly Pizza** (`ASSETS_POLYPIZZA_KEY`) — 3D models; license is per-model, either CC0 or CC-BY (credit required for CC-BY).
- **Poly Haven** (`ASSETS_POLYHAVEN_ENABLE`) — 3D models, CC0. Enabled by a plain flag rather than a key because Poly Haven's API terms are non-commercial use only, even though the assets themselves are CC0 and unrestricted.
- **Jamendo** (`ASSETS_JAMENDO_CLIENT_ID`) — audio (`KindAudio`); Creative-Commons-licensed music tracks. License is per-track, CC 3.0 BY family (`by`, `by-sa`, `by-nc`, `by-nc-sa`, `by-nd`, `by-nc-nd`) — credit is always required, as Jamendo's catalogue carries no CC0 track. `get_audio`'s `format` argument selects mp3 or ogg.
- **Freesound** (`ASSETS_FREESOUND_API_KEY`) — audio (`KindAudio`); Creative-Commons-licensed sound effects and clips. License is per-sound: CC-BY or CC-BY-NC (3.0 or 4.0), or CC0 (no attribution required). `get_audio`'s `format` argument selects mp3 or ogg. The fetched audio is Freesound's high-quality **preview** (`preview-hq-mp3`/`preview-hq-ogg`), not the original master file — the original requires an OAuth2 access token and is out of scope for this API-key-authenticated provider.

## Offline Posture

The embedded offline base — every vendored icon, illustration, and font — is bundled into the binary via Go's `embed` package at build time (`internal/providers/embedded{icons,illustrations,fonts}/data/`) and is always registered, regardless of configuration. Each embedded provider owns its own license metadata and derives item counts from the embedded data it actually carries, so nothing can drift out of sync. By default the server additionally makes outbound requests to the four remote providers described above; set `ASSETS_DISABLE_REMOTE=1` to disable them and restrict the server to the embedded base, at which point it makes no network requests.

## Return Contract

Tools that produce files (`get_icon`, `get_illustration`, `get_font`, `get_photo`, `get_texture`, `get_model`, `get_audio`) write the asset(s) to disk under `ASSETS_OUTPUT_DIR` (or the default temp directory) and return:

1. A human-readable summary **text content block** of what was written.
2. A native **`structuredContent`** object shaped `{"files":[<file>,...]}`, where each `<file>` is:

   ```json
   {
     "path": "/tmp/assets-mcp/icon-lucide-camera.svg",
     "id": "embedded-icons:lucide/camera",
     "kind": "icon",
     "source": "lucide",
     "title": "camera",
     "content_type": "image/svg+xml",
     "license": {
       "spdx": "ISC",
       "attribution": "",
       "requiresAttribution": false
     }
   }
   ```

`list_asset_sources` likewise returns its listing as both a summary text block and a native `structuredContent` object shaped `{"providers":[...]}` — see [Tools](#tools).

`get_icon`, `get_illustration`, `get_font`, `get_photo`, `get_texture`, `get_model`, `get_audio`, and `list_asset_sources` all declare an MCP `outputSchema` for their structured content, so clients can validate or consume it directly without parsing the text block. The `search_*` tools remain text-only.

This mirrors the structured-output shape used elsewhere in this monorepo, e.g. `image-gen-mcp`'s structured `image_url` result.

## Licenses

Every asset carries a license and (where applicable) attribution, retrievable via `list_asset_sources` or the embedded `license` object (`spdx`, `name`, `url`, `attribution`, `requiresAttribution`) on each file returned by `get_icon`/`get_illustration`/`get_font`/`get_photo`/`get_texture`/`get_model`/`get_audio`.

**Icons:**

| Set | License |
|---|---|
| Lucide | ISC |
| Material Symbols | Apache-2.0 |
| Simple Icons | CC0-1.0 (vector data only — see trademark note below) |
| Bootstrap Icons, Feather, Heroicons, Phosphor, Tabler | MIT |

> **Simple Icons trademark note:** the CC0-1.0 license covers the vector artwork, not the brand marks it depicts. Brand logos are third-party trademarks of their respective owners — using them must not imply sponsorship or endorsement.

**Illustrations:** all three collections (Open Doodles, Humaaans, Open Peeps) are CC0-1.0.

**Fonts:** all fourteen families are OFL-1.1. Each family's `LICENSE` file travels alongside its woff2 files in `internal/fonts/data/<family>/` and is not re-served by the MCP server (`get_font`'s `license.attribution` is empty for this reason — the license text itself is bundled in the repo).

## Follow-ups

- **2D game art (Kenney).** Kenney's CC0 game-art packs (sprites, tilesets, UI packs) are a natural fourth asset domain but are not included in this initial version; adding them is a follow-up.
- **demesne filegen wiring.** Sandboxed demesne environments route file-generating MCP tools through `/workspace/generated/`. This server currently writes directly to `ASSETS_OUTPUT_DIR`; wiring it into that convention is a follow-up.
