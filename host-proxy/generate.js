#!/usr/bin/env node
// Generates ~/.config/host-mcp-proxy/config.json by joining ~/.claude.json's
// mcpServers (commands + credentials) with the per-server read-only allowlist
// below. Re-run after rotating credentials in Claude Code or editing the
// allowlist.

const fs = require('fs');
const path = require('path');
const os = require('os');

const PORT = 9090;
const CLAUDE_CONFIG = path.join(os.homedir(), '.claude.json');
const OUT = path.join(os.homedir(), '.config', 'host-mcp-proxy', 'config.json');

// Per-server read-only tool allowlist. Add an entry to expose a server;
// list only tools that read or do pure compute (no writes, no GUI side effects,
// no sync/upload).
const ALLOWLIST = {
  alignment: [
    'get_article',
    'get_recommendations',
    'get_similar_articles',
    'list_disliked',
    'list_liked',
    'list_unreviewed',
    'search_articles',
    'semantic_search',
  ],
  amazon: ['get_product_details', 'list_regions', 'search_products'],
  anki: [
    'collection_stats',
    'findNotes',
    'getTags',
    'get_cards',
    'get_due_cards',
    'guiCurrentCard',
    'guiSelectedNotes',
    'modelFieldNames',
    'modelNames',
    'modelStyling',
    'notesInfo',
    'review_stats',
  ],
  bunpro: [
    'get_decks',
    'get_grammar_point',
    'get_grammar_srs_details',
    'get_jlpt_progress',
    'get_review_activity',
    'get_review_forecast',
    'get_srs_overview',
    'get_stats',
    'get_study_queue',
    'get_user',
    'get_vocab',
    'get_vocab_srs_details',
  ],
  manifold: [
    'get_baseline',
    'get_comments',
    'get_market',
    'get_me',
    'get_portfolio_pnl',
    'get_positions',
    'get_user',
    'list_bets',
    'search_markets',
  ],
  mermaid: ['generate'],
  'supermarkets-uk': [
    'browse_categories',
    'compare_prices',
    'get_basket',
    'get_order_history',
    'get_product_details',
    'list_supermarkets',
    'search_products',
  ],
  wanikani: [
    'get_assignments',
    'get_level_progressions',
    'get_review_statistics',
    'get_subjects',
    'get_summary',
    'get_user',
  ],
  workflowy: ['get_node', 'list_children', 'list_targets', 'search_nodes'],
};

const claude = JSON.parse(fs.readFileSync(CLAUDE_CONFIG, 'utf8'));
const upstreams = claude.mcpServers || {};

const proxyServers = {};
const skipped = [];
for (const [name, allowed] of Object.entries(ALLOWLIST)) {
  const u = upstreams[name];
  if (!u) {
    skipped.push(name);
    continue;
  }
  proxyServers[name] = {
    command: u.command,
    args: u.args || [],
    env: u.env || {},
    options: { toolFilter: { mode: 'allow', list: allowed } },
  };
}

const config = {
  mcpProxy: {
    addr: `127.0.0.1:${PORT}`,
    name: 'nanoclaw-host-mcp-proxy',
    version: '1.0.0',
    type: 'streamable-http',
  },
  mcpServers: proxyServers,
};

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, JSON.stringify(config, null, 2) + '\n');

console.log(`wrote ${OUT}`);
console.log(`  upstreams: ${Object.keys(proxyServers).join(', ')}`);
if (skipped.length) {
  console.log(`  skipped (not in ~/.claude.json): ${skipped.join(', ')}`);
}
