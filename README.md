# NeuralTrust plugins for Cursor

Marketplace of [NeuralTrust](https://neuraltrust.ai) plugins for
[Cursor](https://cursor.com).

| Plugin | What it does |
|---|---|
| [`trustguard/`](./trustguard/) | **NeuralTrust** — TrustGuard AI firewall (hooks) **and** TrustGate MCP Gateway (remote MCP tools). One install for security + aggregated tools. |

## Install

- **Enterprise / Teams**: IT deploys the plugin (Team Marketplace or marketplace)
  plus:
  - managed `cursor.json` with the org Cursor collector key (`tgk_…`) for the firewall
  - plugin variables for the MCP URL (and API key if not OAuth)
- **Local / BYO**: clone this repo and add it in Cursor via
  **Customize → Plugins → Add → From Local Repo**, or develop against
  `~/.cursor/plugins/local/trustguard` (`make install-local`).

Full configuration lives in [`trustguard/README.md`](./trustguard/README.md).

## Repository layout

| Path | Role |
|---|---|
| [`trustguard/`](./trustguard/) | Cursor plugin (hooks, MCP, bootstraps, skill, logo) |
| [`cli/`](./cli/) | `trustguard-cursor` binary (Go, stdlib-only) — talks to TrustGuard `/v1/evaluate` |
| [`.github/workflows/`](./.github/workflows/) | CI + the release state machine that pins and publishes the platform binaries |
| [`scripts/`](./scripts/) | Release plumbing (`release.py`, `build-dist.sh`) and manifest validation |

```bash
make build                 # build ./bin/trustguard-cursor
make test                  # go test -race ./cli/
make install-local         # copy plugin into ~/.cursor/plugins/local (Cursor rejects out-of-tree symlinks)
make validate-marketplace  # AJV-validate manifests against Cursor schemas
```

## Releasing the binary

`main` requires a pull request, so the **Release** workflow never writes to it.
On every push it compares the version pinned in the repo with the `v*` tags and
picks one of two actions:

| State of `main` | Action |
|---|---|
| Pinned version is already tagged | **prepare** — bump the patch, build, pin the new checksums and push the `release/vX.Y.Z` branch |
| Pinned version has no tag | **publish** — rebuild, verify the pins still match, tag and publish the GitHub Release |

Shipping is therefore: merge your work, open the release pull request from the
link the run summary leaves (Actions cannot open it — the org forbids that —
but an already-open one keeps refreshing itself), then approve and merge it.
Bump the minor or major by hand in `plugin.json` and the workflow pins that
version instead of a patch.

The publish job rebuilds rather than trusting what the pull request measured,
and fails if any hash moved — the released assets always match the checksums
the bootstraps verify. That reproducibility relies on the exact `GO_VERSION`
pinned in the workflow; raise it together with the `go` directive in `go.mod`.

Manual run: **Actions → Release → Run workflow**.

```bash
make dist VERSION=0.2.0   # same cross-compile the workflow runs, into ./dist/
```

## Publishing to the Cursor Marketplace

The repo is a multi-plugin marketplace (`neuraltrust`) with one plugin
(`trustguard` / display name **NeuralTrust**). Submission checklist:

1. `make validate-marketplace` and `make test` pass.
2. A recent GitHub Release exists (created automatically from `main`).
3. Repo is **public**: https://github.com/NeuralTrust/trustguard-cursor-plugin
4. Submit the repository URL at
   [cursor.com/marketplace/publish](https://cursor.com/marketplace/publish).

Cursor reviews every marketplace plugin manually.
