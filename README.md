# NeuralTrust plugins for Cursor

Marketplace of [NeuralTrust](https://neuraltrust.ai) plugins for
[Cursor](https://cursor.com).

| Plugin | What it does |
|---|---|
| [`trustguard/`](./trustguard/) | AI firewall for the Cursor agent. One org collector, MDM-deployed — every developer is protected without a NeuralTrust account. |

## Install

- **Enterprise**: IT deploys the plugin (or marketplace) plus a managed
  `cursor.json` with the org Cursor collector key. Developers open Cursor and
  are protected — no NeuralTrust login, no local setup.
- **Local / BYO**: clone this repo and add it in Cursor via
  **Customize → Plugins → Add → From Local Repo**, or develop against
  `~/.cursor/plugins/local/trustguard` (`make install-local`).

Full configuration, hooks and release notes live in
[`trustguard/README.md`](./trustguard/README.md).

## Repository layout

| Path | Role |
|---|---|
| [`trustguard/`](./trustguard/) | Cursor plugin (hooks, bootstraps, skill, logo) |
| [`cli/`](./cli/) | `trustguard-cursor` binary (Go, stdlib-only) — talks to TrustGuard `/v1/evaluate` |
| [`.github/workflows/`](./.github/workflows/) | CI + tag-driven release of the pinned platform binaries |

```bash
make build                 # build ./bin/trustguard-cursor
make test                  # go test -race ./cli/
make install-local         # copy plugin into ~/.cursor/plugins/local (Cursor rejects out-of-tree symlinks)
make validate-marketplace  # AJV-validate manifests against Cursor schemas
```

## Publishing to the Cursor Marketplace

The repo is a multi-plugin marketplace (`neuraltrust`) with one plugin
(`trustguard`). Submission checklist:

1. `make validate-marketplace` and `make test` pass.
2. GitHub Release `vX.Y.Z` exists with the six platform binaries and checksums
   pinned in `trustguard/hooks/trustguard-hook.{sh,ps1}`.
3. Repo is **public**: https://github.com/NeuralTrust/trustguard-cursor-plugin
4. Submit the repository URL at
   [cursor.com/marketplace/publish](https://cursor.com/marketplace/publish).

Cursor reviews every marketplace plugin manually.