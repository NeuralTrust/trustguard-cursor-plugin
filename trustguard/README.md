# TrustGuard plugin for Cursor

Cursor plugin bundling the TrustGuard AI-firewall hooks: every prompt, shell
command, MCP tool call and file read the Cursor agent performs is evaluated by
[TrustGuard](https://neuraltrust.ai) before it executes, and blocked, gated or
allowed according to your TrustGuard policy.

Contents:

- `hooks/hooks.json` — registers the three events (`beforeSubmitPrompt`,
  `preToolUse`, `postToolUse`) against the bootstrap script. `preToolUse` is
  generic, so one hook covers every tool the agent runs — shell, file reads and
  writes, MCP calls, subagents — and `postToolUse` sees what each returns.
- `hooks/trustguard-hook.sh` — bootstrap (macOS/Linux, and Windows under Git
  Bash): runs `trustguard-cursor` from the PATH when present; otherwise
  installs the pinned release for the OS/arch into `~/.trustguard/bin` — in the
  background, so the editor never waits on a download — verifying its SHA-256
  against the table embedded in the script. Any bootstrap failure fails open
  with a stderr warning, so an unconfigured machine never loses the editor.
- `hooks/trustguard-hook.cmd` + `hooks/trustguard-hook.ps1` — Windows
  bootstrap with the same cascade (PATH → `%USERPROFILE%\.trustguard\bin` →
  verified download of the `.exe`). The hook command is a polyglot —
  `sh ./hooks/trustguard-hook.sh || ./hooks/trustguard-hook.cmd` — so one
  `hooks.json` works everywhere: unix runs the `.sh`; Windows uses Git Bash's
  `sh` when available and otherwise falls through to the `.cmd`/PowerShell
  path.
- `skills/setup-trustguard/` — guided setup: configure the endpoint + API key,
  verify; covers manual binary install where needed.

No MDM or manual distribution is required — the first hook event kicks off the
download and events are evaluated from the moment the binary lands (a second or
two later); until then they are allowed. (The PowerShell path still needs a
smoke test on a real Windows machine before publishing.)

## Event → evaluation mapping

| Cursor event | `protocol` | `direction` | Payload sent | Main detectors |
|---|---|---|---|---|
| `beforeSubmitPrompt` | `llm` | `input` | `{"messages":[{role:user, content:prompt}]}` | prompt_guard, data_loss_prevention, prompt_moderation, multiturn_guard |
| `preToolUse` (Shell) | `all` | `input` | `{"input": command}` | code_sanitation, data_loss_prevention |
| `preToolUse` (any other tool) | `mcp` | `input` | JSON-RPC `tools/call` (tool name + arguments) | indirect_prompt_injection, prompt_guard |
| `postToolUse` | `mcp` | `output` | JSON-RPC result carrying the tool output | indirect_prompt_injection, data_loss_prevention |

Verdict mapping: `block` → deny (`continue:false` for prompts) · `transform`
(PII/secrets found; hooks cannot rewrite content) → `ask` by default ·
`report` → allow with a notice · `allow`/gate `skip` → allow.

Neither event enforces `ask`: `beforeSubmitPrompt` only submits or silently
discards, and Cursor accepts but ignores `ask` on `preToolUse`. So an `ask`
verdict lets the action through and surfaces the warning; set
`transform_action: "deny"` to stop it instead.

`postToolUse` fires after the tool has already run and cannot revoke it, so a
finding there is injected as `additional_context` telling the agent to treat the
result as untrusted — never as a block it could not enforce. Disable it with
`{"events": {"postToolUse": false}}` if you only want pre-execution control.
`session_id` is Cursor's conversation id; `consumer_id` prefers the Cursor
account email from the hook payload (`cursor:<email>`), falling back to the OS
user; `attributes.collector.type = "ide"` lets policies target IDE traffic.

## Runtime configuration

Enterprise model: a TrustGuard admin creates **one Cursor collector** for the
org in NeuralTrust. Employees never need a NeuralTrust account — IT deploys the
org key by MDM and Cursor is protected for everyone.

Configuration is layered:

1. **Managed (MDM) file**: `/etc/trustguard/cursor.json` (Linux),
   `/Library/Application Support/TrustGuard/cursor.json` (macOS),
   `%ProgramData%\TrustGuard\cursor.json` (Windows); path override:
   `TRUSTGUARD_CURSOR_SYSTEM_CONFIG`.
2. **User file**: `~/.trustguard/cursor.json` (`TRUSTGUARD_CURSOR_CONFIG`
   overrides the path).
3. **Environment variables**.

When the managed file ships an `api_key`, the install is in **managed mode**:
`api_key`, `data_url` and `fail_mode` are locked — user file and env cannot
replace them (a developer cannot disable or redirect the org firewall). Soft
prefs (`timeout_ms`, `transform_action`, `events`, `consumer_id`) still layer.

Recommended MDM payload:

```json
{
  "data_url": "https://trustguard.example.com",
  "api_key": "tgk_ORG_CURSOR_COLLECTOR",
  "fail_mode": "closed"
}
```

| Env | File key | Default | Meaning |
|---|---|---|---|
| `TRUSTGUARD_DATA_URL` | `data_url` | `http://localhost:8081` | data-plane base URL (locked when managed) |
| `TRUSTGUARD_API_KEY` | `api_key` | — | org Cursor collector API key (`tgk_…`; locked when managed) |
| `TRUSTGUARD_FAIL_MODE` | `fail_mode` | `open` | `open` allows / `closed` denies on errors or timeouts (locked when managed) |
| `TRUSTGUARD_TRANSFORM_ACTION` | `transform_action` | `ask` | hook answer for a `transform` verdict |
| `TRUSTGUARD_TIMEOUT_MS` | `timeout_ms` | `5000` | per-request timeout |
| `TRUSTGUARD_CONSUMER_ID` | `consumer_id` | `cursor:<email\|os user>` | fallback anomaly/policy anchor; runtime prefers Cursor `user_email` |
| — | `max_content_bytes` | `262144` | file/tool content clip size |
| — | `report_notice` | `true` | notice on report-only findings |
| — | `events` | all enabled | disable events, e.g. `{"postToolUse": false}` |

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
