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
        "ASDA_LOGIN": "true",
        "WAITROSE_LOGIN": "true",
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

Search and compare grocery prices across UK supermarkets.

#### Supported supermarkets

| Supermarket | ID | Description | Login |
|---|---|---|---|
| Tesco | `tesco` | The UK's largest supermarket chain | Yes |
| Sainsbury's | `sainsburys` | One of the UK's largest supermarket chains | Yes |
| Ocado | `ocado` | Online-only UK supermarket and grocery delivery service | Yes |
| Morrisons | `morrisons` | Major UK supermarket chain | Yes |
| Asda | `asda` | One of the UK's largest supermarket chains | Yes |
| Waitrose | `waitrose` | Premium UK supermarket chain | Yes |
| HiYoU | `hiyou` | Asian supermarket based in Newcastle | No |
| Tuk Tuk Mart | `tuktukmart` | Manchester-based Asian supermarket (Hang Won Hong's online store) | No |
| Morueats | `morueats` | Asian grocery covering Japanese, Chinese, Korean, and Thai products | No |

#### Tools

| Tool | Description |
|---|---|
| `list_supermarkets` | List all supported supermarkets with their IDs, names, and descriptions |
| `search_products` | Search for grocery products across one or more supermarkets |
| `compare_prices` | Compare prices for a product across all supermarkets |
| `get_product_details` | Get detailed information about a specific product |
| `browse_categories` | Browse product categories for a specific supermarket |

```
bin/supermarkets-uk-mcp
```

#### Chrome requirement

Tesco, Asda, and Waitrose require JavaScript rendering, so a headless Chromium browser is launched at startup. Chrome, Chromium, or Microsoft Edge must be installed on the system. The Shopify-based stores (HiYoU, Tuk Tuk Mart, Morueats) and Sainsbury's use JSON APIs, and Ocado and Morrisons use server-rendered HTML, so they work without a browser — but the browser-based datasources will fail if no browser is available.

#### Login

Some supermarkets return location-specific results (e.g. local stock and delivery availability) when logged in. Login is optional — all supermarkets work without it, but results may be less accurate.

To enable login for a supermarket, set `<SUPERMARKET>_LOGIN=true` (e.g. `TESCO_LOGIN=true`). Login always requires Chrome, Chromium, or Edge, even for supermarkets that don't otherwise need a browser. On first use of that supermarket, a visible browser window opens for you to complete login manually. Once login succeeds, session cookies are cached to disk and reused on subsequent runs. If cached cookies expire, the server clears them and triggers a fresh login on the next use.

| Variable | Description |
|---|---|
| `TESCO_LOGIN` | Enable Tesco login (`true`/`1`/`yes`) |
| `SAINSBURYS_LOGIN` | Enable Sainsbury's login |
| `OCADO_LOGIN` | Enable Ocado login |
| `MORRISONS_LOGIN` | Enable Morrisons login |
| `ASDA_LOGIN` | Enable Asda login |
| `WAITROSE_LOGIN` | Enable Waitrose login |
| `SUPERMARKET_COOKIE_DIR` | Override cookie storage path (default: OS config dir). Required if your MCP client sandboxes the filesystem. |
