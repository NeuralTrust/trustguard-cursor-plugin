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
| Pinned version is already tagged | **prepare** — bump the patch, build, pin the new checksums and open or refresh the `chore(release): vX.Y.Z` pull request |
| Pinned version has no tag | **publish** — rebuild, verify the pins still match, tag and publish the GitHub Release |

Shipping is therefore: merge your work (the release PR refreshes itself on top
of it), then approve and merge that release PR. Bump the minor or major by
hand in `plugin.json` and the workflow pins that version instead of a patch.

The publish job rebuilds rather than trusting what the pull request measured,
and fails if any hash moved — the released assets always match the checksums
the bootstraps verify. That reproducibility relies on the exact `GO_VERSION`
pinned in the workflow; raise it together with the `go` directive in `go.mod`.

Manual run: **Actions → Release → Run workflow**.

```bash
make dist VERSION=0.1.2   # same cross-compile the workflow runs, into ./dist/
```

## Publishing to the Cursor Marketplace

The repo is a multi-plugin marketplace (`neuraltrust`) with one plugin
(`trustguard` / display name **TrustGuard**). Submission checklist:

1. `make validate-marketplace` and `make test` pass.
2. A recent GitHub Release exists (created automatically from `main`).
3. Repo is **public**: https://github.com/NeuralTrust/trustguard-cursor-plugin
4. Submit the repository URL at
   [cursor.com/marketplace/publish](https://cursor.com/marketplace/publish).

Cursor reviews every marketplace plugin manually.