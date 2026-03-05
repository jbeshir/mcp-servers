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
    },
    "supermarkets-uk": {
      "command": "/path/to/supermarkets-uk-mcp",
      "env": {
        "TESCO_LOGIN": "true",
        "SAINSBURYS_LOGIN": "true",
        "OCADO_LOGIN": "true",
        "MORRISONS_LOGIN": "true",
        "SUPERMARKET_COOKIE_DIR": "/home/you/.supermarket-cookies"
      }
    }
  }
}
```

### Claude Code

```
claude mcp add workflowy -- env WORKFLOWY_API_TOKEN=<your-token> /path/to/workflowy-mcp
claude mcp add manifold -- env MANIFOLD_API_KEY=<your-key> /path/to/manifold-mcp
claude mcp add supermarkets-uk -- env TESCO_LOGIN=true SUPERMARKET_COOKIE_DIR=/path/to/cookies /path/to/supermarkets-uk-mcp
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

### UK Supermarkets

Search and compare grocery prices across Tesco, Sainsbury's, Ocado, and Morrisons. Uses a headless browser to scrape product data.

Tools: `search_products`, `compare_prices`, `get_product_details`, `browse_categories`, `list_supermarkets`.

```
bin/supermarkets-uk-mcp
```

A headless Chromium browser is used at runtime.

**Optional login for location-specific results:** Set `<SUPERMARKET>_LOGIN=true` (e.g. `TESCO_LOGIN=true`) to enable authenticated sessions. On first use of each enabled supermarket, a browser window opens for you to log in manually. Session cookies are cached to disk for subsequent runs.

| Variable | Description |
|---|---|
| `TESCO_LOGIN` | Enable Tesco login (`true`/`1`/`yes`) |
| `SAINSBURYS_LOGIN` | Enable Sainsbury's login |
| `OCADO_LOGIN` | Enable Ocado login |
| `MORRISONS_LOGIN` | Enable Morrisons login |
| `SUPERMARKET_COOKIE_DIR` | Override cookie storage path (default: OS config dir). Required if your MCP client sandboxes the filesystem. |
