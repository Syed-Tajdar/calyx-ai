#!/usr/bin/env node
/**
 * R1.3 — Fail if any scripts/* file that generates call IDs for webhook/ingestion
 * omits the smoke- prefix (KPI exclusion only covers apisync-/demo-/smoke-).
 *
 * Allowed: smoke-…, demo-… (fixtures), apisync-… (mock sync).
 * Forbidden: csp-live-, bare UUID CallID=, proof- when used as CallID, etc.
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptsDir = path.join(path.dirname(fileURLToPath(import.meta.url)));
const ALLOWED = /^(smoke-|demo-|apisync-)/;
const failures = [];

/** Patterns that look like call-id construction in scripts. */
const GENERATORS = [
  // template literals / strings that build CallID / call_id / callId values
  {
    re: /(?:CallID|call_id|callId|call_uuid)\s*[=:]\s*[`'"]([^`'"]+)[`'"]/gi,
    extract: (m) => m[1],
  },
  {
    re: /`(?:smoke-|csp-live-|apisync-|demo-|proof-)[^`]*\$\{/gi,
    extract: (m) => m[0].replace(/[`$\{]/g, '').slice(0, 40),
  },
  {
    re: /\$callId\s*=\s*"([^"]+)"/gi,
    extract: (m) => m[1],
  },
  {
    re: /(?:const|let|var)\s+callId\s*=\s*`([^`$]+)/gi,
    extract: (m) => m[1],
  },
  {
    re: /(?:const|let|var)\s+callId\s*=\s*['"]([^'"]+)['"]/gi,
    extract: (m) => m[1],
  },
];

function walk(dir, out = []) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    if (ent.name === 'node_modules' || ent.name.startsWith('.')) continue;
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(p, out);
    else if (/\.(mjs|js|ps1|ts|sh)$/i.test(ent.name)) out.push(p);
  }
  return out;
}

const files = walk(scriptsDir);
for (const file of files) {
  const rel = path.relative(scriptsDir, file).replace(/\\/g, '/');
  // This checker itself mentions forbidden prefixes — skip.
  if (rel === 'check-smoke-call-id-prefix.mjs') continue;
  const text = fs.readFileSync(file, 'utf8');

  // Explicit ban: historical contamination prefix
  if (/csp-live-/.test(text) && !/\/\/.*csp-live-/.test(text)) {
    // Allow comments that document the ban
    const lines = text.split(/\r?\n/);
    lines.forEach((line, i) => {
      if (/csp-live-/.test(line) && !/^\s*(\/\/|#|\*|\/\*)/.test(line)) {
        failures.push(`${rel}:${i + 1}: contains csp-live- (use smoke- prefix)`);
      }
    });
  }

  // Bare GUID CallID in PowerShell (must use smoke- prefix string)
  if (
    /CallID=\$callId/.test(text) &&
    /\$callId\s*=\s*\[guid\]::NewGuid/.test(text) &&
    !/\$callId\s*=\s*"(smoke-|demo-|apisync-)/.test(text)
  ) {
    failures.push(`${rel}: CallID=\$callId from bare NewGuid — prefix with smoke-`);
  }

  // JS/TS template call ids that don't start with allowed prefix
  const tpl = [...text.matchAll(/callId\s*=\s*`([^`$][^`]*)`/gi)];
  for (const m of tpl) {
    const prefix = m[1];
    if (prefix && !ALLOWED.test(prefix) && !/\$\{/.test(prefix)) {
      failures.push(`${rel}: callId=\`${prefix}\` missing smoke-/demo-/apisync- prefix`);
    }
  }
  const tplDyn = [...text.matchAll(/callId\s*=\s*`(smoke-|demo-|apisync-|csp-live-|proof-)([^`]*)`/gi)];
  for (const m of tplDyn) {
    if (m[1] === 'csp-live-' || m[1] === 'proof-') {
      failures.push(`${rel}: callId uses forbidden prefix ${m[1]}`);
    }
  }
}

if (failures.length) {
  console.error('SMOKE PREFIX CHECK FAIL:');
  for (const f of failures) console.error('  ' + f);
  process.exit(1);
}

console.log(`SMOKE PREFIX CHECK PASS (${files.length} scripts scanned)`);
