# Agent Guidelines & Engineering Knowledge

This document captures architectural conventions, platform quirks, and engineering learnings for AI agents and developers working on this codebase.

## 1. Scraping & Social Media Data Ingestion
- **Headless Chrome vs Direct HTTP APIs:**
  - Major platforms (Instagram, etc.) actively block and aggressively rate-limit direct REST/GraphQL calls (`web_profile_info`, etc.) with HTTP 401/429 even when supplying active session cookies.
  - Rely on headless browser automation (e.g. `chromedp` with a persistent profile directory) for reliable DOM extraction without tripping WAF / anti-bot challenges.
  - Handle lazy-loaded content gracefully using window scroll events or cap fetches to the visible viewport (e.g. 10–15 recent posts per sync).

## 2. URL Handling & Collaboration Posts
- **Preserve Raw DOM URLs:**
  - Avoid truncating, canonicalizing, or regex-replacing URLs extracted from the browser DOM (e.g. collaboration reels with long tracking hashes like `/reel/DZhqagnOnFNGYq7YKn414FStUGtD2pnz8obJmg0/`). Modifying these links risks pointing users to unrelated videos.
- **Collaborative Posts Tagging:**
  - For shared or collaborative reels originated from a partner profile, set the Atom entry title to `"Collab Video"`. Standard reels should be titled `"Video"` and photos titled `"Photo"`.

## 3. Handle File Conventions & Privacy
- **Directory Structure:**
  - All platform target files belong inside the dedicated `handles/` directory (e.g., `handles/instagram.txt`, `handles/facebook.txt`).
- **Private / Local Accounts (`.local.txt`):**
  - Personal or uncommitted handles must be placed in a companion file named `handles/<platform>.local.txt`.
  - All `handles/*.local.txt` files are strictly excluded via `.gitignore`.
  - The sync engine discovers both public and local files, merging handles and deduplicating them case-insensitively before fetching.
- **Strict Handle-Only Validation:**
  - Target files strictly accept handle names (`username` or `@username`). Full URLs (`https://...`) are rejected at parse time with an actionable error showing how to extract the handle.

## 4. Feed Generation & Formatting
- **Clickable Media:**
  - RSS/Atom content HTML must embed thumbnail images inside clickable hyperlinks (`<a href="..."><img src="..." /></a>`) pointing to the original post. Avoid cluttering content with redundant trailing text links.
- **Single Feed Architecture (Capped at 25 Items):**
  - Every profile produces one feed: `<handle>.xml` capped at the latest 25 items for fast, clean RSS reader consumption without bloat.
  - Feeds act as their own state store: the sync engine parses existing feeds on disk to calculate deltas, merge new items, and keep the latest 25 items idempotently without requiring an external database.
