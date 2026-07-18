# assetsdb integration

The assets server can consume a local database built by the public
[`github.com/jbeshir/assetsdb`](https://github.com/jbeshir/assetsdb) tool. Set `ASSETS_DB` to the
database root. Loading is local and remains available when remote providers are disabled; an unset or
invalid database is nonfatal.

`internal/providers/assetsdb` is the sole adapter. Configuration loads the root consumer API once with
`assetsdb.Read` and creates one shared catalog. Its model, audio, font, and sprite views preserve the
provider name `assetsdb`, source filters, license and pack metadata, and sprite regions.

The root `assetsdb.DB` owns database layout and archive security:

- `Sources` enumerates registered packs.
- `ItemsForSource` supplies pack and per-kind counts.
- `OpenSource` returns an original pack archive.
- `Open` extracts an indexed item.

The MCP adapter does not construct database paths or repeat archive-containment checks. Pack discovery
is included in `list_asset_sources`; `get_pack` writes the raw archive returned by `OpenSource`.
