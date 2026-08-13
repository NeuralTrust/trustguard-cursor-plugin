# TrustGuard plugin for Cursor

Cursor plugin bundling the TrustGuard AI-firewall hooks: every prompt, shell
command, MCP tool call and file read the Cursor agent performs is evaluated by
[TrustGuard](https://neuraltrust.ai) before it executes, and blocked, gated or
allowed according to your TrustGuard policy.

Contents:

- `hooks/hooks.json` — registers the four control events
  (`beforeSubmitPrompt`, `beforeShellExecution`, `beforeMCPExecution`,
  `beforeReadFile`) against the bootstrap script.
- `hooks/trustguard-hook.sh` — bootstrap (macOS/Linux, and Windows under Git
  Bash): runs `trustguard-cursor` from the PATH when present; otherwise
  downloads the pinned release for the OS/arch into `~/.trustguard/bin`,
  verifies its SHA-256 against the table embedded in the script, and executes
  it. Any bootstrap failure fails open with a stderr warning, so an
  unconfigured machine never loses the editor.
- `hooks/trustguard-hook.cmd` + `hooks/trustguard-hook.ps1` — Windows
  bootstrap with the same cascade (PATH → `%USERPROFILE%\.trustguard\bin` →
  verified download of the `.exe`). The hook command is a polyglot —
  `sh ./hooks/trustguard-hook.sh || ./hooks/trustguard-hook.cmd` — so one
  `hooks.json` works everywhere: unix runs the `.sh`; Windows uses Git Bash's
  `sh` when available and otherwise falls through to the `.cmd`/PowerShell
  path.
- `skills/setup-trustguard/` — guided setup: configure the endpoint + API key,
  verify; covers manual binary install where needed.

No MDM or manual distribution is required — the first hook event downloads
the binary automatically. (The PowerShell path still needs a smoke test on a
real Windows machine before publishing.)

## Publishing model

Cursor requires plugin sources to be publicly accessible, so this directory
is the **source of truth** mirrored to the public
`NeuralTrust/trustguard-cursor-plugin` repository by the `cursor-plugin-sync`
workflow on every change to `main` (same split as `semgrep/cursor-plugin` and
`snyk/cursor-plugin-snyk`). Binaries are released there too — a public repo is
also required for unauthenticated downloads — by the
`cursor-integration-release` workflow on `cursor-vX.Y.Z` tags, which prints
the `VERSION` + checksum blocks to paste into both bootstrap scripts. Both
workflows need the `GH_TOKEN` secret (PAT with write access to the public
repo). The marketplace listing points at the public repo. Runtime configuration lives in `~/.trustguard/cursor.json` — see
[../README.md](../README.md) for the full reference (fail modes, per-event
toggles, enterprise distribution notes).
