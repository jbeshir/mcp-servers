# mcp-servers

A collection of [Model Context Protocol](https://modelcontextprotocol.io/) servers written in Go. Each server communicates over stdio and can be used with any MCP-compatible client such as Claude Desktop or Claude Code.

Pre-built binaries for macOS (Intel & Apple Silicon), Linux, and Windows are available from [Releases](https://github.com/jbeshir/mcp-servers/releases).

To build all servers from source: `make build` (binaries are written to `bin/`). Requires Go 1.24+.

---

# Workflowy

Search, read, create, and organize [Workflowy](https://workflowy.com) nodes.

**Binary:** `workflowy-mcp`

## Requirements

- A Workflowy API token from [workflowy.com/api-key](https://workflowy.com/api-key)

## Configuration

| Variable | Required | Description |
|---|---|---|
| `WORKFLOWY_API_TOKEN` | Yes | Your Workflowy API token |
| `WORKFLOWY_API_URL` | No | Custom API URL (default: `https://workflowy.com`) |

### Claude Desktop

```json
{
  "mcpServers": {
    "workflowy": {
      "command": "/path/to/workflowy-mcp",
      "env": { "WORKFLOWY_API_TOKEN": "<your-token>" }
    }
  }
}
```

### Claude Code

```
claude mcp add workflowy -- env WORKFLOWY_API_TOKEN=<your-token> /path/to/workflowy-mcp
```

## Tools

| Tool | Description |
|---|---|
| `search_nodes` | Search nodes by keyword across name and note fields |
| `get_node` | Get full details of a node by ID |
| `list_children` | List child nodes of a parent, sorted by priority |
| `create_node` | Create a new node |
| `update_node` | Update an existing node's properties |
| `delete_node` | Delete a node |
| `move_node` | Move a node to a different parent |
| `complete_node` | Mark a node as completed |
| `uncomplete_node` | Mark a node as not completed |
| `list_targets` | List system locations (home/inbox) and user shortcuts |

---

# Manifold Markets

Search and read prediction markets, place bets and limit orders, create and resolve markets, and comment on [Manifold Markets](https://manifold.markets).

**Binary:** `manifold-mcp`

## Requirements

- A Manifold API key from your [Manifold account settings](https://manifold.markets/profile)

## Configuration

| Variable | Required | Description |
|---|---|---|
| `MANIFOLD_API_KEY` | Yes | Your Manifold Markets API key |
| `MANIFOLD_API_URL` | No | Custom API URL (default: `https://api.manifold.markets`) |

### Claude Desktop

```json
{
  "mcpServers": {
    "manifold": {
      "command": "/path/to/manifold-mcp",
      "env": { "MANIFOLD_API_KEY": "<your-key>" }
    }
  }
}
```

### Claude Code

```
claude mcp add manifold -- env MANIFOLD_API_KEY=<your-key> /path/to/manifold-mcp
```

## Tools

| Tool | Description |
|---|---|
| `search_markets` | Search markets by keyword and filters |
| `get_market` | Get full market details including answers and description |
| `get_user` | Get a user's profile by username |
| `get_me` | Get the authenticated user's profile |
| `list_bets` | List bets with optional filters |
| `get_comments` | Get comments on markets |
| `get_positions` | Get user positions for a specific market |
| `place_bet` | Place a bet or limit order (supports `dryRun`) |
| `sell_shares` | Sell shares in a market |
| `cancel_bet` | Cancel a pending limit order |
| `create_market` | Create a new market (binary, multiple choice, or numeric) |
| `resolve_market` | Resolve a market you created |
| `close_market` | Close a market or change its closing time |
| `add_comment` | Comment on a market |
| `add_liquidity` | Add mana liquidity to a market |
| `send_mana` | Send mana to other users |

---

# UK Supermarkets

Search and compare grocery prices across UK supermarkets. Returns product details including prices, promotions, descriptions, ingredients, and nutritional information where available.

**Binary:** `supermarkets-uk-mcp`

> **Note:** This server scrapes supermarket websites and APIs that are not designed for programmatic access. Selectors and page structures can change without notice, which may cause individual supermarkets to break until the server is updated. Expect occasional flakiness — some supermarkets may intermittently block requests or return incomplete data, particularly when not logged in.

## Requirements

- **Chrome, Chromium, or Microsoft Edge** — required for Tesco, Asda, and Waitrose, which need JavaScript rendering via a headless browser. Also required for login to any supermarket. The remaining supermarkets (Sainsbury's, Ocado, Morrisons, and the Shopify-based stores) use JSON APIs or server-rendered HTML and work without a browser.

## Configuration

No environment variables are required to get started — all supermarkets work without login.

### Claude Desktop

```json
{
  "mcpServers": {
    "supermarkets-uk": {
      "command": "/path/to/supermarkets-uk-mcp"
    }
  }
}
```

### Claude Code

```
claude mcp add supermarkets-uk /path/to/supermarkets-uk-mcp
```

## Tools

| Tool | Description |
|---|---|
| `list_supermarkets` | List all supported supermarkets with IDs and status |
| `search_products` | Search for products across one or more supermarkets |
| `compare_prices` | Compare prices for a product across all supermarkets |
| `get_product_details` | Get detailed product info (price, description, ingredients, nutrition) |
| `browse_categories` | Browse product categories for a supermarket |

## Supported supermarkets

| Supermarket | Data source | Browser required |
|---|---|---|
| Tesco | HTML (headless browser) | Yes |
| Sainsbury's | JSON API | No |
| Ocado | Server-rendered HTML | No |
| Morrisons | Server-rendered HTML | No |
| Asda | Algolia API (search) + HTML (product details) | Yes |
| Waitrose | HTML (headless browser) | Yes |
| HiYoU | Shopify JSON API | No |
| Tuk Tuk Mart | Shopify JSON API | No |
| Morueats | Shopify JSON API | No |

## Product information by supermarket

Not all supermarkets provide the same level of detail:

| Field | Tesco | Sainsbury's | Ocado | Morrisons | Asda | Waitrose | Shopify stores |
|---|---|---|---|---|---|---|---|
| Name / Price / URL | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Price per unit | Yes | Yes | Yes | Yes | Yes | Yes | — |
| Promotions | Yes | Yes | Yes | Yes | Yes | Yes | — |
| Description | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Ingredients | Yes | Yes | Yes | Yes | Yes | Yes | — |
| Nutrition | Yes | Yes | Yes | Yes | Yes | Yes | — |
| Dietary info | — | — | — | — | Yes | — | — |
| Weight | — | — | — | Yes | Yes | Yes | Yes |

## Login

Login is optional. Some supermarkets return location-specific results (e.g. local stock and delivery availability) when logged in, but all work without it.

To enable login, set `<SUPERMARKET>_LOGIN=true` for each supermarket you want to log in to. On first use, a visible browser window opens for you to complete login manually. Session cookies are cached to disk and reused on subsequent runs. If cookies expire, the server clears them and triggers a fresh login.

**Be aware:** Login requires the server to be able to find and launch Chrome, Chromium, or Edge — including for supermarkets that don't otherwise need a browser. Supermarket sessions tend to expire frequently, so you should expect to be prompted to log in again regularly.

| Variable | Description |
|---|---|
| `TESCO_LOGIN` | Enable Tesco login |
| `SAINSBURYS_LOGIN` | Enable Sainsbury's login |
| `OCADO_LOGIN` | Enable Ocado login |
| `MORRISONS_LOGIN` | Enable Morrisons login |
| `ASDA_LOGIN` | Enable Asda login |
| `WAITROSE_LOGIN` | Enable Waitrose login |
| `SUPERMARKET_COOKIE_DIR` | Override cookie storage directory (default: OS config dir) |

Example with login enabled:

```json
{
  "mcpServers": {
    "supermarkets-uk": {
      "command": "/path/to/supermarkets-uk-mcp",
      "env": {
        "TESCO_LOGIN": "true",
        "WAITROSE_LOGIN": "true",
        "SUPERMARKET_COOKIE_DIR": "/home/you/.supermarket-cookies"
      }
    }
  }
}
```
