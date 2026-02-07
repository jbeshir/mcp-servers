# mcp-servers

Go MCP servers for various services. Each server communicates over stdio.

## Install

Pre-built binaries for macOS, Windows, and Linux are available from [Releases](https://github.com/jbeshir/mcp-servers/releases).

## Build from source

```
make build
```

Binaries are written to `bin/`.

## Usage

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "workflowy": {
      "command": "/path/to/workflowy-mcp",
      "env": { "WORKFLOWY_API_TOKEN": "<your-token>" }
    },
    "manifold": {
      "command": "/path/to/manifold-mcp",
      "env": { "MANIFOLD_API_KEY": "<your-key>" }
    }
  }
}
```

### Claude Code

```
claude mcp add workflowy -- env WORKFLOWY_API_TOKEN=<your-token> /path/to/workflowy-mcp
claude mcp add manifold -- env MANIFOLD_API_KEY=<your-key> /path/to/manifold-mcp
```

## Servers

### Workflowy

Search, read, create, and organize Workflowy nodes including move, complete, and delete operations.

```
WORKFLOWY_API_TOKEN=<your-token> bin/workflowy-mcp
```

Get your API token from [workflowy.com/api-key](https://workflowy.com/api-key).

### Manifold Markets

Search and read prediction markets, place bets and limit orders, create and resolve markets, and comment.

```
MANIFOLD_API_KEY=<your-key> bin/manifold-mcp
```

Get your API key from your Manifold account settings.
