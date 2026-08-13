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

## Event → evaluation mapping

| Cursor event | `protocol` | `direction` | Payload sent | Main detectors |
|---|---|---|---|---|
| `beforeSubmitPrompt` | `llm` | `input` | `{"messages":[{role:user, content:prompt}]}` | prompt_guard, data_loss_prevention, prompt_moderation, multiturn_guard |
| `beforeShellExecution` | `all` | `input` | `{"input": command}` | code_sanitation, data_loss_prevention |
| `beforeMCPExecution` | `mcp` | `input` | JSON-RPC `tools/call` (tool name + arguments) | indirect_prompt_injection, prompt_guard |
| `beforeReadFile` | `mcp` | `output` | JSON-RPC result carrying the file content | indirect_prompt_injection, data_loss_prevention |

Verdict mapping: `block` → deny (`continue:false` for prompts) · `transform`
(PII/secrets found; hooks cannot rewrite content) → `ask` by default ·
`report` → allow with a notice · `allow`/gate `skip` → allow. `session_id` is
Cursor's conversation id; `consumer_id` defaults to the OS user;
`attributes.collector.type = "ide"` lets policies target IDE traffic.

## Runtime configuration

The guard, detectors and policies are managed by a TrustGuard admin in the
NeuralTrust app; the hook only needs the data-plane URL and a collector API
key. Configuration is layered — each level overrides the fields it sets:

1. **Managed (MDM) file**: `/etc/trustguard/cursor.json` (Linux),
   `/Library/Application Support/TrustGuard/cursor.json` (macOS),
   `%ProgramData%\TrustGuard\cursor.json` (Windows); path override:
   `TRUSTGUARD_CURSOR_SYSTEM_CONFIG`.
2. **User file**: `~/.trustguard/cursor.json` (`TRUSTGUARD_CURSOR_CONFIG`
   overrides the path).
3. **Environment variables** (win over both).

| Env | File key | Default | Meaning |
|---|---|---|---|
| `TRUSTGUARD_DATA_URL` | `data_url` | `http://localhost:8081` | data-plane base URL |
| `TRUSTGUARD_API_KEY` | `api_key` | — | collector API key (`tgk_…`) |
| `TRUSTGUARD_FAIL_MODE` | `fail_mode` | `open` | `open` allows / `closed` denies on errors or timeouts |
| `TRUSTGUARD_TRANSFORM_ACTION` | `transform_action` | `ask` | hook answer for a `transform` verdict |
| `TRUSTGUARD_TIMEOUT_MS` | `timeout_ms` | `5000` | per-request timeout |
| `TRUSTGUARD_CONSUMER_ID` | `consumer_id` | `cursor:<os user>` | anomaly/policy anchor |
| — | `max_content_bytes` | `262144` | file/tool content clip size |
| — | `report_notice` | `true` | notice on report-only findings |
| — | `events` | all enabled | disable events, e.g. `{"beforeReadFile": false}` |

Without an API key the hook logs to stderr and **allows everything** — an
unconfigured install never bricks the editor.

## Release model

This repository is self-contained: the binary source lives in
[`../cli/`](../cli/), and pushing a `vX.Y.Z` tag runs the `release` workflow —
cross-compiles the six platform binaries, publishes them as a GitHub Release
here (the URLs the bootstraps download from), and prints the `VERSION` +
checksum blocks to commit into both bootstrap scripts (plus the `plugin.json`
version bump). The pinned checksums in the reviewed plugin are what make the
auto-downloaded binary trustworthy — never ship a release without updating
them.
