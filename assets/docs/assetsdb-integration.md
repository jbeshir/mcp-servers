# Spec: wire the `assets` MCP server to consume `assetsdb`

Hand-off for a coding agent. Implements the game-art integration: the `assets` server serves items
from a local **assetsdb** database (curated CC0 packs: Kenney, KayKit) alongside its existing
providers.

## Context

- `assetsdb` is a standalone public repo/tool: **github.com/jbeshir/assetsdb**. It builds
  an on-disk DB (`datapackage.json` + `sources/<id>.zip`) and exposes a pure, network-free Go package
  **`github.com/jbeshir/assetsdb/format`**. See that repo's `SPEC.md` for the format + the consumer
  contract this spec implements.
- We are NOT re-indexing packs here. We import `format`, `Read` a DB the user built with
  `assetsdb build`, and serve it.
- Design already settled (do not relitigate): **items surface in the existing per-kind searches,
  tagged with their pack**; a "pack" is a cross-kind grouping, not a new kind; add a `get_pack`
  (whole-ZIP) path; tool descriptions nudge "search everything first, then pack-download on overlap".

## `format` API you'll use (exact)

```go
db, err := format.Read(dir)                 // loads <dir>/datapackage.json
hits := db.Search(query string) []format.Item   // case-insensitive; every query token must substring-match title+tokens
lic  := db.LicenseFor(it) format.License        // .Name is the SPDX id (e.g. "CC0-1.0")
src, ok := db.SourceByID(it.Source) (format.Source, bool)  // pack metadata (Title, Tags, Origin)
rc, err := db.Open(it) (io.ReadCloser, error)   // extracts it.Path from sources/<it.Source>.zip; wraps format.ErrNotFound
```

`format.Item`: `ID` = `"assetsdb:<sourceid>/<path>"` (with `#<subtexture>` suffix for atlas sub-sprites),
`Name`, `Kind` (`sprite2d|model3d|audio|font`), `Source` (pack id), `Path`, `Tokens`, `MediaType`,
`Region *Region` (pixel rect, set only for atlas sub-sprites), `Provenance *Provenance` (always nil for
curated — phase-2 reserved). `format.Source`: `Name`, `Title`, `Tags`, `Origin`, `Licenses`.

## Kind mapping (assetsdb → assetcore)

| assetsdb `Kind` | assetcore kind |
|---|---|
| `model3d` | `KindModel` (existing) |
| `audio` | `KindAudio` (existing) |
| `font` | `KindFont` (existing) |
| `sprite2d` | `KindSprite` |

**Decided — 2D home.** Kenney 2D tiles/sprites use the dedicated `sprite` kind with
`search_sprites` and `get_sprite`. Atlas fetches return the sheet plus region metadata. Raster sprites
are distinct from the SVG scene artwork represented by `illustration`.

## Implementation (in `assets/`)

1. **Dependency.** `go get github.com/jbeshir/assetsdb@latest` in `assets/go.mod`. The repo is public,
   so no `GOPRIVATE`/auth is needed.

2. **Config (`internal/config`).** Add `ASSETS_DB` (a filesystem path) read only in `LoadConfig` →
   `Config.AssetsDB string`. In `Setup`, **if it's set**, `format.Read(cfg.AssetsDB)` once and construct
   the game-art providers from the returned `*format.DB`. **Not gated by `ASSETS_DISABLE_REMOTE`** —
   assetsdb is a *local* source (no network), so it may register even in offline mode. Unset → not
   registered (never fatal); a `format.Read` error → log and skip (don't crash the server).

3. **Provider package `internal/providers/gameart`** (self-contained, per repo convention). One shared
   loaded `*format.DB`; expose one provider value per kind it serves — `gameart.NewModels(db)`,
   `NewAudio(db)`, `NewFonts(db)`, `NewSprites(db)` (or `NewIllustrations`), each implementing the
   matching per-kind interface. All share provider `Name() == "assetsdb"` (fine — the registry is
   per-kind, so same name across kinds doesn't collide). At construction, index items by their
   provider-local id (`ID` minus the `assetsdb:` prefix) for O(1) `Fetch`.
   - `Search(ctx, SearchOpts)`: `db.Search(opts.Query)`, filter to this provider's kind, honour
     `opts.Sources`/`opts.Providers` filters, map each `format.Item` → `assetcore.Asset`:
     `ID = assetcore.AssetID("assetsdb", "<sourceid>/<path>")`, `Source = it.Source`, `Title/Tags` from
     the item + `Tokens`, `License` from `db.LicenseFor` (SPDX → `assetcore.License`),
     `Meta{ "pack": it.Source, "pack_title": src.Title, "kind": string(it.Kind) }` (+ region for sprites).
     Return via the aggregate `SearchResult` (no cursor needed — the whole index is in memory; return all
     matches up to `ClampLimit`, `NextCursor=""`).
   - `Fetch(ctx, localID, opts)`: look up the `format.Item` by localID → `db.Open(it)` → read bytes →
     `assetcore.Blob{ Content, ContentType = it.MediaType, Filename = base(it.Path) }`; map
     `format.ErrNotFound` → `assetcore.ErrNotFound`.

4. **New `get_pack` tool + pack discovery** (`internal/server`). `get_pack(pack_id)` → resolve the
   pack, copy `<ASSETS_DB>/sources/<pack_id>.zip` to the output dir, return it as a `manifestFile`
   (ContentType `application/zip`). Surface packs in discovery: extend `list_asset_sources` to include
   assetsdb packs (id, title, tags, item count, license), OR add `search_packs(query)`. Keep
   tools.go / manifest.json / README in sync (repo rule).

5. **Tool-description guidance.** In the `search_*` descriptions (at least the kinds assetsdb feeds) and
   `get_pack`, add: *"Game-art results include their pack in `pack`. Search for everything you need
   first; when several results share a pack, prefer one `get_pack` over many individual fetches."*

## Testing

- Unit-test the gameart providers against a **fixture DB**: check a tiny `datapackage.json` +
  `sources/*.zip` into `testdata/` (or generate one in-test with `format.Write` + a hand-built zip),
  then assert Search (kind filter + pack Meta + SPDX license), Fetch (real bytes + ErrNotFound), and
  get_pack. Mock the per-kind interface if you add the `sprite` kind (regenerate mockery).
- Everything must pass the repo gate: `make lint` (golangci-lint **v2.8.0**), `test-short`, `build`,
  `check-mocks`, `go test -race ./internal/...`.
- **Live verification (host):** build a real DB (`assetsdb build --manifest packs.yaml --out /tmp/db`
  in the assetsdb repo — already verified to produce ~1900 items), point `ASSETS_DB` at it, and smoke
  `search_models`/`search_sprites`/`get_pack`.

## Out of scope (phase 2, tracked in assetsdb BACKLOG)

Generated-source ingest (`assetsdb add` write path + a `save_asset` MCP tool for the
search-before-generate flow); itch-only packs; vision-caption enrichment. The `format` schema already
reserves `Provenance` for these — no format change needed to get there.
