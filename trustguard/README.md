# NeuralTrust for Cursor

[NeuralTrust](https://neuraltrust.ai) in one Cursor plugin:

1. **TrustGuard** — AI firewall on agent hooks (prompts, tool calls, tool results).
2. **TrustGate MCP Gateway** — remote MCP tools through your org’s MCP plane.

| Surface | When it runs | Backend |
|---|---|---|
| Hooks (`beforeSubmitPrompt`, `preToolUse`, `postToolUse`) | Every agent turn / tool | TrustGuard `POST /v1/evaluate` |
| MCP server **TrustGate** | When the agent uses MCP tools | TrustGate MCP plane `…/{consumer}/mcp` |

An admin creates **one Cursor collector** (firewall) and **one MCP consumer**
(tools). IT deploys firewall credentials by MDM and MCP connection values as
plugin variables (or OAuth). Developers do not need a NeuralTrust account for
the firewall path.

## Enterprise rollout

### 1. TrustGuard firewall (hooks)

1. In NeuralTrust, create a **Cursor** collector and mint a collector API key (`tgk_…`).
2. Deploy this plugin (Team Marketplace, marketplace, or image).
3. Push managed config with MDM:

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

Attribution uses the Cursor account email (`consumer_id = cursor:<email>`).

### 2. TrustGate MCP Gateway (tools)

1. In NeuralTrust, create/select an **MCP consumer** and open **Connect**.
2. Copy the MCP URL (`https://{host}/{consumer-slug}/mcp`).
3. Set plugin variables (team admin once, or each developer under
   **Customize → Plugins → NeuralTrust → Configure**):

| Variable | Required | Notes |
|---|---|---|
| `TRUSTGATE_MCP_URL` | Yes | Full URL from Connect |
| `TRUSTGATE_MCP_API_KEY` | If API-key auth | Header `X-AG-API-Key`. Leave empty for **OAuth** |
| `TRUSTGATE_GATEWAY_SLUG` | Hybrid only | Header `X-AG-Gateway-Slug` |

OAuth consumers only need the URL — Cursor opens the login flow on first use.

**Do not** put the TrustGuard `tgk_…` key in MCP variables. Different product,
different credential.

## What the firewall evaluates

| Cursor event | When it runs | What TrustGuard sees |
|---|---|---|
| `beforeSubmitPrompt` | User hits send | The prompt (`llm` / input) |
| `preToolUse` | Before any tool runs | Shell command (`all`) or tool call (`mcp` / input) |
| `postToolUse` | After a tool returns | Tool output (`mcp` / output) |

`preToolUse` is generic: Shell, Read, Write, MCP, Task/subagents. When the agent
calls tools via the bundled TrustGate MCP server, those calls are still gated by
the same hooks.

### Verdicts

| TrustGuard status | Prompt (`beforeSubmitPrompt`) | Tool call (`preToolUse`) | Tool result (`postToolUse`) |
|---|---|---|---|
| `block` | Dropped — `TrustGuard blocked this action` | Denied | Context injected: treat result as untrusted |
| `ask` | Submitted with warning (no confirmation UI) | `permission: "ask"` | Allowed — must not revoke an already-approved tool |
| `transform` | Submitted with warning (or dropped if `transform_action=deny`) | `permission: "ask"` by default (or denied if `transform_action=deny`) | Context injected when configured to deny/ask |
| `report` | Allowed, optional notice | Allowed, optional notice | No-op unless notice applies |
| `allow` / `skip` | Allowed | Allowed | No-op |

Notes:

- Cursor has no confirmation UI for prompts. A gate `ask` on
  `beforeSubmitPrompt` still submits, with a warning.
- `preToolUse` now emits `permission: "ask"` for gate `ask` and for
  `transform` when `transform_action` is `ask` (the default).
  Set `transform_action: "deny"` to hard-stop PII/secrets.
- `postToolUse` cannot revoke a tool that already ran. Detector findings
  become `additional_context`. A gate `ask` on output is ignored so an
  approved PreToolUse is not re-challenged.

### Managed mode (firewall)

When the managed file ships an `api_key`, the install is **managed**:

- Locked: `api_key`, `data_url`, `fail_mode` — user file and env cannot replace
  them. A developer cannot disable or redirect the org firewall.
- Soft prefs still layer: `timeout_ms`, `transform_action`, `events`,
  `consumer_id`.

## Local / BYO setup

### Firewall

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

### MCP Gateway

1. Set **TrustGate MCP URL** (and API key if not OAuth) in plugin Configure.
2. Reload the window; confirm **TrustGate** under MCP servers in Customize.
3. Ask the agent to use a tool from a toolkit bound to that consumer.

The plugin skill `setup-trustguard` walks through firewall + MCP setup inside Cursor.

## Configuration reference

### Firewall (hooks)

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

### MCP Gateway (plugin variables)

Set in Cursor **Plugins → Configure** (not in `cursor.json`):

| Variable | Header / field | Notes |
|---|---|---|
| `TRUSTGATE_MCP_URL` | MCP `url` | Required |
| `TRUSTGATE_MCP_API_KEY` | `X-AG-API-Key` | Optional; empty for OAuth |
| `TRUSTGATE_GATEWAY_SLUG` | `X-AG-Gateway-Slug` | Optional; hybrid |

## Plugin layout

```
trustguard/
├── .cursor-plugin/plugin.json   # manifest, variables, mcpServers
├── mcp.json                     # TrustGate remote MCP entry (${…} placeholders)
├── assets/logo.svg
├── hooks/
│   ├── hooks.json               # beforeSubmitPrompt, preToolUse, postToolUse
│   ├── trustguard-hook.sh       # macOS / Linux (+ Git Bash)
│   ├── trustguard-hook.cmd      # Windows entry
│   └── trustguard-hook.ps1      # Windows bootstrap
└── skills/setup-trustguard/     # guided setup (firewall + MCP)
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

Binary source: [`../cli/`](../cli/). Every push to `main` auto-releases: bumps
the patch version, publishes the six platform binaries, and pins `VERSION` +
SHA-256 into both bootstraps (plus `plugin.json`). See the root README for
skip flags and branch-protection notes.
