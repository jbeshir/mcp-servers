# Discord Emojigen MCP server

Discover emoji in one Discord server, fetch a selected static emoji as image
data, and upload host-generated image data as a custom emoji. This server does
not generate images: the MCP host or agent supplies that capability.

The intended workflow is:

1. Call `list_server_emojis` to discover stable emoji IDs and metadata.
2. Optionally call `fetch_emoji_reference` with an ID to obtain base64 image
   data for a host-provided image generator.
3. Generate or edit the image outside this server.
4. Call `upload_emoji` with raw base64 or a PNG/JPEG/GIF data URL. The server
   crops and downsamples it to a 128×128 PNG and returns an immediately usable
   Discord mention.

The server is stateless. Listing always comes from Discord, and removal succeeds
only when Discord reports that the authenticated bot created the emoji.
Deployments sharing a bot token consequently share ownership.

Configure the bot with `CREATE_GUILD_EXPRESSIONS` only. Do not grant
`MANAGE_GUILD_EXPRESSIONS` or Administrator: those permissions would give the
bot authority over emoji created by others even though this server refuses to
exercise it.

## Tools

| Tool | Description |
|---|---|
| `list_server_emojis` | List guild emoji with stable IDs, mentions, preview URLs, and animation state |
| `fetch_emoji_reference` | Fetch one validated static guild emoji as base64 image data |
| `upload_emoji` | Prepare and upload host-generated image data, with optional role restrictions |
| `list_created_emojis` | List emoji Discord reports were created by this bot |
| `remove_emoji` | Remove an emoji only after a fresh Discord creator check |

Animated emoji are listed but deliberately rejected by
`fetch_emoji_reference`. Upload input is bounded at 16 MiB decoded and prepared
output must fit Discord's 256 KiB limit.

## Configuration

| Environment variable | Required | Description |
|---|---:|---|
| `DISCORD_EMOJIGEN_BOT_TOKEN` | yes | Discord bot token |
| `DISCORD_EMOJIGEN_GUILD_ID` | yes | The only guild the process may access |
| `DISCORD_EMOJIGEN_HTTP_TIMEOUT` | no | Discord HTTP timeout as a Go duration; defaults to `2m` |

## Installation

```sh
go install github.com/jbeshir/mcp-servers/discord-emojigen/cmd/discord-emojigen-mcp@latest
```

The server uses stdio transport:

```sh
discord-emojigen-mcp
```
