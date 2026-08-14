# TrustGuard for Cursor

AI firewall for the Cursor agent, powered by
[NeuralTrust TrustGuard](https://neuraltrust.ai).

An admin creates **one Cursor collector** in NeuralTrust. IT deploys that
collector’s credentials by MDM. Every developer’s Cursor is then gated —
prompts, tool calls and tool results — without giving them a NeuralTrust
account.

| Cursor event | When it runs | What TrustGuard sees |
|---|---|---|
| `beforeSubmitPrompt` | User hits send | The prompt (`llm` / input) |
| `preToolUse` | Before any tool runs | Shell command (`all`) or tool call (`mcp` / input) |
| `postToolUse` | After a tool returns | Tool output (`mcp` / output) |

`preToolUse` is generic: Shell, Read, Write, MCP, Task/subagents — one hook
covers them all. `postToolUse` covers the same surface on the way back
(indirect prompt injection and DLP on file contents, command output, MCP
responses).

## Enterprise rollout

1. In NeuralTrust, create a **Cursor** collector on the org’s guard and mint a
   collector API key (`tgk_…`).
2. Deploy this plugin to employees (marketplace, MDM, or image).
3. Push the managed config with MDM (or your fleet tool):

```json
{
  "data_url": "https://trustguard.example.com",
  "api_key": "tgk_ORG_CURSOR_COLLECTOR",
  "fail_mode": "closed"
}
```

| OS | Managed config path |
|---|---|
| macOS | `/Library/Application Support/TrustGuard/cursor.json` |
| Linux | `/etc/trustguard/cursor.json` |
| Windows | `%ProgramData%\TrustGuard\cursor.json` |

That’s it. Developers never configure anything. Attribution uses the Cursor
account email from each hook payload (`consumer_id = cursor:<email>`), so you
can audit and route policy per person without per-user credentials.

### Managed mode

When the managed file ships an `api_key`, the install is **managed**:

- Locked: `api_key`, `data_url`, `fail_mode` — user file and env cannot replace
  them. A developer cannot disable or redirect the org firewall.
- Soft prefs still layer: `timeout_ms`, `transform_action`, `events`,
  `consumer_id`.

## What each event does

| Event | Protocol | Direction | Payload | Typical detectors |
|---|---|---|---|---|
| `beforeSubmitPrompt` | `llm` | `input` | user message | prompt_guard, DLP, prompt_moderation, multiturn_guard |
| `preToolUse` (Shell) | `all` | `input` | `{"input": "<command>"}` | code_sanitation, DLP |
| `preToolUse` (other) | `mcp` | `input` | JSON-RPC `tools/call` | indirect_prompt_injection, prompt_guard |
| `postToolUse` | `mcp` | `output` | JSON-RPC tool result | indirect_prompt_injection, DLP |

### Verdicts

| TrustGuard status | Prompt (`beforeSubmitPrompt`) | Tool call (`preToolUse`) | Tool result (`postToolUse`) |
|---|---|---|---|
| `block` | Dropped — `TrustGuard blocked this action` | Denied | Context injected: treat result as untrusted |
| `transform` | Submitted with warning (or dropped if `transform_action=deny`) | Allowed with warning (or denied if `transform_action=deny`) | Context injected when configured to deny/ask |
| `report` | Allowed, optional notice | Allowed, optional notice | No-op unless notice applies |
| `allow` / `skip` | Allowed | Allowed | No-op |

Notes:

- Cursor has no `ask` for prompts, and does not enforce `ask` on `preToolUse`.
  Default `transform_action` is `ask` → the action proceeds with a warning.
  Set `transform_action: "deny"` to hard-stop PII/secrets.
- `postToolUse` cannot revoke a tool that already ran. Findings become
  `additional_context` for the agent, not a fake deny.

## Local / BYO setup

Only needed when MDM is not in play (dev machines, pilots).

1. Install the plugin (marketplace, local repo, or `make install-local`).
2. Write `~/.trustguard/cursor.json` (`chmod 600`):

```json
{
  "data_url": "https://trustguard.example.com",
  "api_key": "tgk_REPLACE_ME",
  "fail_mode": "closed"
}
```

3. Smoke-test:

```bash
echo '{"hook_event_name":"preToolUse","tool_name":"Shell","tool_input":{"command":"echo hello"},"user_email":"you@company.com"}' \
  | trustguard-cursor hook
# → {"permission":"allow"}
```

The first hook event downloads the pinned `trustguard-cursor` binary into
`~/.trustguard/bin` in the background (SHA-256 verified). Until it lands,
events fail open so the editor never bricks. A binary already on `PATH` always
wins (manual / MDM installs).

The plugin skill `setup-trustguard` walks through the same flow inside Cursor.

## Configuration reference

Layer order: managed file → user file → environment.

| Env | File key | Default | Notes |
|---|---|---|---|
| `TRUSTGUARD_DATA_URL` | `data_url` | `http://localhost:8081` | Locked when managed |
| `TRUSTGUARD_API_KEY` | `api_key` | — | Org Cursor collector key (`tgk_…`). Locked when managed |
| `TRUSTGUARD_FAIL_MODE` | `fail_mode` | `open` | `closed` recommended in enterprise. Locked when managed |
| `TRUSTGUARD_TRANSFORM_ACTION` | `transform_action` | `ask` | `ask` / `deny` / `allow` for `transform` verdicts |
| `TRUSTGUARD_TIMEOUT_MS` | `timeout_ms` | `5000` | Per `/v1/evaluate` call |
| `TRUSTGUARD_CONSUMER_ID` | `consumer_id` | `cursor:<os user>` | Fallback only — runtime prefers Cursor `user_email` |
| — | `max_content_bytes` | `262144` | Clip size for tool output sent to the guard |
| — | `report_notice` | `true` | Surface report-only findings to the user |
| — | `events` | all on | e.g. `{"postToolUse": false}` |

Path overrides: `TRUSTGUARD_CURSOR_SYSTEM_CONFIG`, `TRUSTGUARD_CURSOR_CONFIG`.

No API key → stderr warning and **allow everything** (fail open).

## Plugin layout

```
trustguard/
├── .cursor-plugin/plugin.json   # manifest + logo
├── assets/logo.svg
├── hooks/
│   ├── hooks.json               # beforeSubmitPrompt, preToolUse, postToolUse
│   ├── trustguard-hook.sh       # macOS / Linux (+ Git Bash)
│   ├── trustguard-hook.cmd      # Windows entry (POSIX fail-open half of the polyglot)
│   └── trustguard-hook.ps1      # Windows bootstrap
└── skills/setup-trustguard/     # guided setup inside Cursor
```

The hook command is a polyglot —
`sh ./hooks/trustguard-hook.sh || ./hooks/trustguard-hook.cmd` — so one
`hooks.json` works on every OS.

## Developing this repo

```bash
make build          # ./bin/trustguard-cursor
make test
make install-local  # copy into ~/.cursor/plugins/local/trustguard
                    # (Cursor rejects symlinks that point outside that tree)
```

Binary source: [`../cli/`](../cli/). Tag `vX.Y.Z` runs the release workflow:
cross-compiles six platform binaries, publishes a GitHub Release, and prints
the `VERSION` + SHA-256 block to paste into both bootstraps (and bump
`plugin.json`). Never ship a release without updating those pinned checksums.
