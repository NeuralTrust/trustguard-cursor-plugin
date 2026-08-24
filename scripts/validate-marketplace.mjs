#!/usr/bin/env node
/**
 * Validates marketplace + plugin manifests against the official Cursor schemas
 * vendored under scripts/. Exit 0 on success; print every failure otherwise.
 */
import { readFileSync, existsSync, readdirSync, statSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

async function loadAjv() {
  try {
    const Ajv = (await import("ajv")).default;
    const addFormats = (await import("ajv-formats")).default;
    return { Ajv, addFormats };
  } catch {
    console.error(
      "ajv + ajv-formats are required. Run: npm i --no-save ajv ajv-formats"
    );
    process.exit(2);
  }
}

function loadJSON(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function fail(msg) {
  console.error(`✗ ${msg}`);
  return false;
}

function ok(msg) {
  console.log(`✓ ${msg}`);
  return true;
}

const { Ajv, addFormats } = await loadAjv();
const ajv = new Ajv({ allErrors: true, strict: false });
addFormats(ajv);

const pluginSchema = loadJSON(join(root, "scripts/plugin.schema.json"));
const marketplaceSchema = loadJSON(join(root, "scripts/marketplace.schema.json"));
const validatePlugin = ajv.compile(pluginSchema);
const validateMarketplace = ajv.compile(marketplaceSchema);

let passed = true;

const marketplacePath = join(root, ".cursor-plugin/marketplace.json");
if (!existsSync(marketplacePath)) {
  passed = fail("missing .cursor-plugin/marketplace.json") || passed;
} else {
  const marketplace = loadJSON(marketplacePath);
  if (!validateMarketplace(marketplace)) {
    passed = fail(
      `marketplace.json: ${ajv.errorsText(validateMarketplace.errors)}`
    );
  } else {
    ok("marketplace.json schema");
  }

  const names = new Set();
  for (const entry of marketplace.plugins ?? []) {
    if (names.has(entry.name)) {
      passed = fail(`duplicate plugin name in marketplace: ${entry.name}`);
    }
    names.add(entry.name);

    const pluginDir = join(root, entry.source);
    const manifestPath = join(pluginDir, ".cursor-plugin/plugin.json");
    if (!existsSync(manifestPath)) {
      passed = fail(`${entry.name}: missing ${manifestPath}`);
      continue;
    }
    const manifest = loadJSON(manifestPath);
    if (!validatePlugin(manifest)) {
      passed = fail(
        `${entry.name} plugin.json: ${ajv.errorsText(validatePlugin.errors)}`
      );
    } else if (manifest.name !== entry.name) {
      passed = fail(
        `${entry.name}: marketplace name ≠ plugin.json name (${manifest.name})`
      );
    } else {
      ok(`${entry.name} plugin.json schema`);
    }

    if (manifest.logo) {
      const logoPath = join(pluginDir, manifest.logo);
      if (!existsSync(logoPath)) {
        passed = fail(`${entry.name}: logo not found at ${manifest.logo}`);
      } else {
        ok(`${entry.name} logo ${manifest.logo}`);
      }
    }

    const hooksPath = join(pluginDir, "hooks/hooks.json");
    if (existsSync(hooksPath)) {
      const hooks = loadJSON(hooksPath);
      if (!hooks.hooks || typeof hooks.hooks !== "object") {
        passed = fail(`${entry.name}: hooks/hooks.json missing hooks map`);
      } else {
        ok(`${entry.name} hooks/hooks.json`);
      }
      for (const [event, list] of Object.entries(hooks.hooks ?? {})) {
        for (const h of list) {
          // Commands are shell strings; just ensure they reference relative paths.
          if (typeof h.command === "string" && h.command.includes("..")) {
            passed = fail(
              `${entry.name}: hook ${event} command must not contain '..'`
            );
          }
        }
      }
    }

    const skillsDir = join(pluginDir, "skills");
    if (existsSync(skillsDir)) {
      for (const name of readdirSync(skillsDir)) {
        const skillPath = join(skillsDir, name, "SKILL.md");
        if (!existsSync(skillPath)) {
          passed = fail(`${entry.name}: skills/${name} missing SKILL.md`);
          continue;
        }
        const body = readFileSync(skillPath, "utf8");
        if (!body.startsWith("---")) {
          passed = fail(`${entry.name}: skills/${name}/SKILL.md missing frontmatter`);
          continue;
        }
        const end = body.indexOf("---", 3);
        const fm = body.slice(3, end);
        if (!/name:\s*\S+/.test(fm) || !/description:\s*\S+/.test(fm)) {
          passed = fail(
            `${entry.name}: skills/${name}/SKILL.md needs name + description`
          );
        } else {
          ok(`${entry.name} skill ${name}`);
        }
      }
    }

    // MCP config: discover mcp.json (or manifest mcpServers path) and ensure
    // every ${VAR} placeholder is declared under plugin.json variables.
    const mcpRel =
      typeof manifest.mcpServers === "string"
        ? manifest.mcpServers
        : existsSync(join(pluginDir, "mcp.json"))
          ? "mcp.json"
          : null;
    if (mcpRel) {
      const mcpPath = join(pluginDir, mcpRel);
      if (!existsSync(mcpPath)) {
        passed = fail(`${entry.name}: mcpServers path missing: ${mcpRel}`);
      } else {
        const mcpRaw = readFileSync(mcpPath, "utf8");
        try {
          loadJSON(mcpPath);
          ok(`${entry.name} ${mcpRel} JSON`);
        } catch (e) {
          passed = fail(`${entry.name}: ${mcpRel} invalid JSON: ${e.message}`);
        }
        const placeholders = [...mcpRaw.matchAll(/\$\{([A-Z][A-Z0-9_]*)\}/g)].map(
          (m) => m[1]
        );
        const declared = new Set(
          Object.keys(manifest.variables?.properties ?? {})
        );
        for (const name of new Set(placeholders)) {
          if (!declared.has(name)) {
            passed = fail(
              `${entry.name}: ${mcpRel} uses \${${name}} but variables.properties lacks it`
            );
          }
        }
        if (placeholders.length && declared.size) {
          ok(`${entry.name} MCP variable placeholders declared`);
        }
      }
    }

    if (!existsSync(join(pluginDir, "README.md")) && !existsSync(join(root, "README.md"))) {
      passed = fail(`${entry.name}: no README.md`);
    } else {
      ok(`${entry.name} README present`);
    }
  }
}

if (!existsSync(join(root, "LICENSE"))) {
  passed = fail("missing LICENSE at repo root");
} else {
  ok("LICENSE present");
}

// Ensure bootstrap scripts are marked executable in git (unix).
for (const rel of [
  "trustguard/hooks/trustguard-hook.sh",
  "trustguard/hooks/trustguard-hook.cmd",
]) {
  const mode = (statSync(join(root, rel)).mode & 0o111) !== 0;
  if (!mode) {
    passed = fail(`${rel} is not executable on disk`);
  } else {
    ok(`${rel} executable`);
  }
}

if (!passed) {
  console.error("\nMarketplace readiness: FAILED");
  process.exit(1);
}
console.log("\nMarketplace readiness: OK");
