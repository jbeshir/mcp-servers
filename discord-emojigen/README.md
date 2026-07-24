# Discord Emojigen MCP server

Generate a static custom emoji, optionally use existing emoji from the same
Discord server as visual references, upload it, and receive a Discord mention
that is ready to use immediately.

The server is stateless. It removes an emoji only when Discord reports that the
authenticated bot created it. Deployments that share a bot token consequently
share ownership.

Configure the Discord bot with `CREATE_GUILD_EXPRESSIONS` only. Do not grant
`MANAGE_GUILD_EXPRESSIONS` or Administrator: those permissions would give the
bot authority over emoji created by other users even though this server refuses
to exercise it.

## Tools

| Tool | Description |
|---|---|
| `list_server_emojis` | List guild emoji for discovery and reference IDs |
| `create_emoji` | Generate and upload an emoji, optionally using existing emoji as references |
| `list_created_emojis` | List emoji that Discord reports were created by this bot |
| `remove_emoji` | Remove an emoji only after a Discord creator check |

## Configuration

| Environment variable | Required | Description |
|---|---:|---|
| `DISCORD_EMOJIGEN_BOT_TOKEN` | yes | Discord bot token |
| `DISCORD_EMOJIGEN_GUILD_ID` | yes | The only guild the process may access |
| `OPENAI_API_KEY` | yes | OpenAI API key for image generation |
| `DISCORD_EMOJIGEN_IMAGE_MODEL` | no | Image model; defaults to `gpt-image-2` |
| `DISCORD_EMOJIGEN_MAX_REFERENCES` | no | Maximum references per request; defaults to 4, hard limit 8 |
| `DISCORD_EMOJIGEN_HTTP_TIMEOUT` | no | Upstream timeout as a Go duration; defaults to `2m` |

The current default OpenAI model does not produce transparent backgrounds. The
server preserves the generated background and downsamples the output to the
128×128 PNG required by Discord.

## Installation

```sh
go install github.com/jbeshir/mcp-servers/discord-emojigen/cmd/discord-emojigen-mcp@latest
```

The server uses stdio transport:

```sh
discord-emojigen-mcp
```
