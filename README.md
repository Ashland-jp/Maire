# MAIRE – Multi-Anchor Immutable Reasoning Engine
Pronunciation: “Mary” • Stable: v1.0 

## Core Philosophy 
- Every LLM response is appended to prompt header — 100 % immutable audit trail
- Full context is preserved by default (no compression until explicitly requested)
- No model ever knows who spoke before it by name (optional anonymization)
- In advanced modes, every model gets the first AND last word exactly once
- Truth wins — not the loudest, fastest, or most expensive model

## Version Roadmap

| Version | Name             | Key Feature                                    | Status         |
|--------|------------------|------------------------------------------------|----------------|
| v1.0   | Standard Chain   | Forward → Reverse pass + final summary         | DONE (Go)      |
| v2.0   | Double Helix     | Simultaneous forward + reverse chains          | Ready          |
| v3.0   | Star Topology    | N parallel chains, each model anchors once     | Ready          |
| v4.0   | Compression Mode | Last-2-layers + semantic summarization         | Planned        |
| v5.0   | Hash-Chained     | SHA3-512 per layer, verifiable offline logs    | Planned        |
| v6.0   | MCP Agent        | Cursor / VSCode tool integration               | Planned        |
| v7.0   | Local-First      | Fully offline with 405B-class models           | Planned        |
| v8.0   | Paid Tier        | Optional middle-man billing layer              | Future         |
| v9.0   | Juggernaut       | Star + compression + local cache + MCP         | Endgame        |

## Tech Stack (as of 2025)
- Backend: Pure Go (net/http, zero external deps for core engine)
- Frontend: Custom SvelteKit / Vite + Tailwind (migrating from Open WebUI fork)
- Persistence: Append-only signed chains on disk (future: IPFS / Arweave)

## License
MIT — original Open WebUI code copyright preserved where used.  
Everything added after 2025 is © Ashland-jp.

Built from Figma Make prototype. Backend in Go
  
  ## Running the code

  Run `npm i` to install the dependencies.

  Run `npm run dev` to start the development server.
  
