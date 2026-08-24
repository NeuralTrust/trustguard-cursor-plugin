---
name: setup-trustguard
description: Set up NeuralTrust in Cursor — TrustGuard firewall (hooks + collector key) and TrustGate MCP Gateway (plugin variables). Use when the user installs the NeuralTrust/TrustGuard plugin, asks to configure TrustGuard or TrustGate MCP, or when trustguard-cursor is missing from the PATH.
---

# Set up NeuralTrust for Cursor

This plugin does two independent things:

| Piece | What | Credentials |
| --- | --- | --- |
| **TrustGuard firewall** | Hooks on prompts / tool I/O → `POST /v1/evaluate` | Org Cursor collector `tgk_…` (MDM or `~/.trustguard/cursor.json`) |
| **TrustGate MCP Gateway** | Remote MCP tools for the agent | Plugin variables: MCP URL (+ optional API key / gateway slug) |

Do **not** reuse the TrustGuard `tgk_…` key as an MCP credential. Different planes.

---

## A. TrustGuard firewall

### 1. Check for MDM (enterprise) first

Look for the managed config file:

- macOS: `/Library/Application Support/TrustGuard/cursor.json`
- Linux: `/etc/trustguard/cursor.json`
- Windows: `%ProgramData%\TrustGuard\cursor.json`

If it exists and contains an `api_key`, setup is already done by IT. Tell the
user their org firewall is managed — they cannot (and should not) override
`api_key`, `data_url` or `fail_mode`. Skip to step 3 (verify). Soft prefs such
as `transform_action` or `timeout_ms` can still live in `~/.trustguard/cursor.json`.

### 2. Install the binary (if needed)

On macOS/Linux this is usually automatic: the plugin's bootstrap hook
downloads the pinned release into `~/.trustguard/bin` (SHA-256 verified) on
the first event. Check whether a binary is already available:

```bash
trustguard-cursor version || ls ~/.trustguard/bin/
```

Install manually only if both are missing (Windows, unsupported arch, or a
network that blocks GitHub releases):

- **From a release**: download the binary for the user's OS/arch from
  https://github.com/NeuralTrust/trustguard-cursor-plugin/releases and place
  it on the PATH (e.g. `/usr/local/bin/trustguard-cursor`, `chmod +x`).
- **From source** (requires Go): in a clone of
  https://github.com/NeuralTrust/trustguard-cursor-plugin run `make build`,
  then copy `bin/trustguard-cursor` onto the PATH.

### 3. Configure the connection (BYO / non-MDM only)

Only when step 1 found no managed key. Ask the user for the data-plane URL and
the **org** Cursor collector API key (`tgk_…`) from their security/platform
team — not a personal NeuralTrust key. Do NOT ask the user to paste the key
into the chat — have them create the file themselves, or write a placeholder:

```json
{
  "data_url": "https://<trustguard-data-plane>",
  "api_key": "tgk_REPLACE_ME",
  "fail_mode": "closed"
}
```

Path: `~/.trustguard/cursor.json`, `chmod 600`.

Optional soft keys: `transform_action` (`ask` / `deny` / `allow`),
`timeout_ms` (5000), `consumer_id` (fallback only — runtime prefers the Cursor
account email from the hook payload).

### 4. Verify firewall

```bash
echo '{"hook_event_name":"preToolUse","tool_name":"Shell","tool_input":{"command":"echo hello"},"user_email":"you@company.com"}' | trustguard-cursor hook
```

Expected: `{"permission":"allow"}`. If it prints a stderr warning about a
missing API key, configuration is incomplete. A quick block test (needs the
`code_sanitation` detector enabled in the policy):

```bash
echo '{"hook_event_name":"preToolUse","tool_name":"Shell","tool_input":{"command":"rm -rf /"}}' | trustguard-cursor hook
```

Expected: `{"permission":"deny",...}` with user message
`TrustGuard blocked this action`.

Tell the user that hooks activate for new agent conversations, and that
verdicts follow the TrustGuard policy: `block` → denied, PII/secrets
(`transform`) → allowed with a warning unless `transform_action` is `deny`,
`report` → allowed with a notice. A finding on `postToolUse` cannot undo a tool
that already ran; it is injected as context flagging the result as untrusted.
Attribution uses `consumer_id = cursor:<user_email>` when Cursor provides it.

---

## B. TrustGate MCP Gateway

Shipped as the plugin’s `mcp.json` entry **TrustGate**. Values come from Cursor
plugin variables (**Customize → Plugins → NeuralTrust → Configure**, or the
team admin sets them once for Team Marketplace installs).

### 1. Get the Connect snippet

In the NeuralTrust console, open the MCP **consumer** → **Connect**. Copy:

- **MCP URL** — `https://{host}/{consumer-slug}/mcp` (required)
- **API key** — only if the consumer uses API-key auth (not OAuth)
- **Gateway slug** — only for hybrid / private data planes (`X-AG-Gateway-Slug`)

### 2. Set plugin variables

| Variable | When |
| --- | --- |
| `TRUSTGATE_MCP_URL` | Always — full URL ending in `/mcp` |
| `TRUSTGATE_MCP_API_KEY` | API-key consumers only; leave empty for OAuth |
| `TRUSTGATE_GATEWAY_SLUG` | Hybrid only; leave empty on typical SaaS |

OAuth consumers: set only the URL. On first tool use Cursor runs the OAuth /
“Login with NeuralTrust” flow.

### 3. Verify MCP

1. Reload the window if variables were just set.
2. Open **Customize** and confirm the **TrustGate** MCP server is enabled.
3. In chat, ask the agent to list available MCP tools, or run a harmless tool
   from a toolkit bound to that consumer.

If tools do not appear: wrong URL, missing API key for an API-key consumer, or
the consumer has no registries/toolkits attached in TrustGate.

### Team rollout

Admins on Teams/Enterprise can set the three variables once under the team
plugin configuration so employees do not paste URLs. Firewall credentials stay
in MDM `cursor.json` (`tgk_…`); MCP stays in plugin variables (or OAuth).
