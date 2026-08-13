---
name: setup-trustguard
description: Set up the TrustGuard AI firewall for Cursor — install the trustguard-cursor binary, configure the TrustGuard endpoint and API key, and verify the hooks work. Use when the user installs the TrustGuard plugin, asks to configure TrustGuard, or when trustguard-cursor is missing from the PATH.
---

# Set up TrustGuard for Cursor

The TrustGuard plugin gates this agent with hooks that run `trustguard-cursor hook`.
For the hooks to enforce anything, the binary must be on the PATH and configured
with a TrustGuard data-plane URL and a collector API key. Walk the user through
the three steps below, verifying each one.

## 1. Install the binary

On macOS/Linux this is usually automatic: the plugin's bootstrap hook
downloads the pinned release into `~/.trustguard/bin` (SHA-256 verified) on
the first event. Check whether a binary is already available:

```bash
trustguard-cursor version || ls ~/.trustguard/bin/
```

Install manually only if both are missing (Windows, unsupported arch, or a
network that blocks GitHub releases):

- **From a release**: download the binary for the user's OS/arch from the
  NeuralTrust TrustGuard releases page (`cursor-vX.Y.Z` tags) and place it on
  the PATH (e.g. `/usr/local/bin/trustguard-cursor`, `chmod +x`).
- **From source** (requires Go): in a TrustGuard checkout run
  `make build-cursor-integration`, then copy `bin/trustguard-cursor` onto the PATH.

## 2. Configure the connection

The hook reads `~/.trustguard/cursor.json` (environment variables `TRUSTGUARD_*`
override it). Two situations:

- **The user's team already has TrustGuard**: ask the user for the data-plane
  URL and a collector API key (`tgk_…`). Do NOT ask the user to paste the key
  into the chat — have them create the file themselves, or write the file with a
  placeholder and let them fill it in:

  ```json
  {
    "data_url": "https://<trustguard-data-plane>",
    "api_key": "tgk_REPLACE_ME"
  }
  ```

  The file should be `chmod 600`.

- **Fresh TrustGuard**: an admin can provision everything (guard, detectors,
  policy, collector and API key) with
  `trustguard-cursor setup -control-url <control-plane> -data-url <data-plane> -write-config`
  using an admin JWT in `ADMIN_JWT`.

Optional keys: `fail_mode` (`open` default / `closed`), `transform_action`
(`ask` default / `deny` / `allow`), `timeout_ms` (5000), `consumer_id`.

## 3. Verify

Run a canned event through the hook and confirm TrustGuard answers:

```bash
echo '{"hook_event_name":"beforeShellExecution","command":"echo hello"}' | trustguard-cursor hook
```

Expected: `{"permission":"allow"}`. If it prints a stderr warning about a
missing API key, step 2 is incomplete. A quick block test (needs the
`code_sanitation` detector enabled in the policy):

```bash
echo '{"hook_event_name":"beforeShellExecution","command":"rm -rf /"}' | trustguard-cursor hook
```

Expected: `{"permission":"deny",...}`.

Tell the user that hooks activate for new agent conversations, and that
verdicts follow the TrustGuard policy: `block` → denied, PII/secrets
(`transform`) → confirmation prompt, `report` → allowed with a notice.
