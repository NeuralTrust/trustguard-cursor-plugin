---
name: setup-trustguard
description: Set up the TrustGuard AI firewall for Cursor — install the trustguard-cursor binary, configure the TrustGuard endpoint and API key, and verify the hooks work. Use when the user installs the TrustGuard plugin, asks to configure TrustGuard, or when trustguard-cursor is missing from the PATH.
---

# Set up TrustGuard for Cursor

The TrustGuard plugin gates this agent with hooks that run `trustguard-cursor hook`.
Enterprise orgs ship one Cursor collector for the whole company: employees do
**not** need a NeuralTrust account. Walk the user through the steps below.

## 1. Check for MDM (enterprise) first

Look for the managed config file:

- macOS: `/Library/Application Support/TrustGuard/cursor.json`
- Linux: `/etc/trustguard/cursor.json`
- Windows: `%ProgramData%\TrustGuard\cursor.json`

If it exists and contains an `api_key`, setup is already done by IT. Tell the
user their org firewall is managed — they cannot (and should not) override
`api_key`, `data_url` or `fail_mode`. Skip to step 3 (verify). Soft prefs such
as `transform_action` or `timeout_ms` can still live in `~/.trustguard/cursor.json`.

## 2. Install the binary (if needed)

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

## 3. Configure the connection (BYO / non-MDM only)

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

## 4. Verify

Run a canned event through the hook and confirm TrustGuard answers:

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
