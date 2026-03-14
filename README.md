# mcp-servers

A collection of [Model Context Protocol](https://modelcontextprotocol.io/) servers written in Go. Each server communicates over stdio and can be used with any MCP-compatible client such as Claude Desktop or Claude Code.

## Servers

### [Workflowy](workflowy/)

Full read/write access to [Workflowy](https://workflowy.com) — search, create, organize, and complete nodes. Smart caching respects API rate limits, and search results include breadcrumb paths showing where nodes sit in the hierarchy.

**10 tools:** `search_nodes`, `get_node`, `list_children`, `create_node`, `update_node`, `delete_node`, `move_node`, `complete_node`, `uncomplete_node`, `list_targets`

**Requires:** Workflowy API token

### [Manifold Markets](manifold/)

Search and trade on [Manifold Markets](https://manifold.markets) prediction markets — place bets and limit orders, create and resolve markets, manage liquidity, and comment.

**16 tools:** `search_markets`, `get_market`, `get_user`, `get_me`, `list_bets`, `get_comments`, `get_positions`, `place_bet`, `sell_shares`, `cancel_bet`, `create_market`, `resolve_market`, `close_market`, `add_comment`, `add_liquidity`, `send_mana`

**Requires:** Manifold API key

### [UK Supermarkets](supermarkets-uk/)

Search and compare grocery prices across 9 UK supermarkets (Tesco, Sainsbury's, Ocado, Morrisons, Asda, Waitrose, HiYoU, Tuk Tuk Mart, Morueats). Returns prices, promotions, descriptions, ingredients, and nutritional information. Optional login enables order history and basket management for Tesco.

**9 tools:** `list_supermarkets`, `search_products`, `compare_prices`, `get_product_details`, `browse_categories`, `get_order_history`, `get_basket`, `add_to_basket`, `remove_from_basket`

**Requires:** Chrome/Chromium/Edge for some stores (Tesco, Asda, Waitrose). No API keys needed.

### [Amazon Products](amazon-products/)

Search for products and get detailed information from Amazon across 20+ regional sites. Handles WAF challenges and CAPTCHA detection automatically.

**3 tools:** `list_regions`, `search_products`, `get_product_details`

**Requires:** Chrome/Chromium/Edge

## Installation

### Pre-built binaries

Download from [Releases](https://github.com/jbeshir/mcp-servers/releases) — available for macOS (Intel & Apple Silicon), Linux, and Windows.

### Install from source

Requires Go 1.24+.

```
go install github.com/jbeshir/mcp-servers/workflowy/cmd/workflowy-mcp@latest
go install github.com/jbeshir/mcp-servers/manifold/cmd/manifold-mcp@latest
go install github.com/jbeshir/mcp-servers/supermarkets-uk/cmd/supermarkets-uk-mcp@latest
go install github.com/jbeshir/mcp-servers/amazon-products/cmd/amazon-products-mcp@latest
```

### Build from source

Clone the repo and run `make build` — binaries are written to `bin/`.

## Registry

These servers are published to the [MCP Registry](https://registry.modelcontextprotocol.io):

- [workflowy-mcp](https://registry.modelcontextprotocol.io/server/io.github.jbeshir/workflowy-mcp)
- [manifold-mcp](https://registry.modelcontextprotocol.io/server/io.github.jbeshir/manifold-mcp)
- [supermarkets-uk-mcp](https://registry.modelcontextprotocol.io/server/io.github.jbeshir/supermarkets-uk-mcp)
- [amazon-products-mcp](https://registry.modelcontextprotocol.io/server/io.github.jbeshir/amazon-products-mcp)
