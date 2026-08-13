# NeuralTrust plugins for Cursor

Cursor plugin marketplace for [NeuralTrust](https://neuraltrust.ai). Install in
Cursor via **Customize → Plugins → Add → From Local Repo** (a clone of this
repository) or with the repository URL where supported.

| Plugin | What it does |
|---|---|
| [`trustguard/`](./trustguard/) | AI firewall for the Cursor agent: every prompt, shell command, MCP tool call and file read is evaluated by TrustGuard before it executes — prompt injection, dangerous commands and sensitive-data leaks are blocked per policy. |

See [`trustguard/README.md`](./trustguard/README.md) for configuration
(TrustGuard endpoint + collector API key) and how the binary bootstrap works.

## Repository layout

- [`trustguard/`](./trustguard/) — the Cursor plugin (hooks, bootstraps, skill).
- [`cli/`](./cli/) — source of the `trustguard-cursor` hook binary (Go,
  stdlib-only; it talks exclusively to the TrustGuard data plane
  `/v1/evaluate`). `make build` / `make test`.
- [`.github/workflows/`](./.github/workflows/) — CI and the tag-driven
  release that publishes the platform binaries the plugin bootstraps
  auto-download (SHA-256 pinned in the reviewed plugin).
