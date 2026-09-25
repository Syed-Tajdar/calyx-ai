import fs from 'node:fs';

function analyze(label, path) {
  const h = fs.readFileSync(path, 'utf8');
  const scripts = [...h.matchAll(/<script([^>]*)>([\s\S]*?)<\/script>/gi)];
  const inline = scripts.filter((m) => !/\bsrc=/.test(m[1]));
  const withNonce = scripts.filter((m) => /nonce=/.test(m[1]));
  console.log(`\n=== ${label} ===`);
  console.log(`total_scripts=${scripts.length} inline=${inline.length} with_nonce=${withNonce.length}`);
  inline.forEach((m, i) => {
    const attrs = m[1].trim().slice(0, 220) || '(no attrs)';
    const body = (m[2] || '').trim().replace(/\s+/g, ' ').slice(0, 200);
    console.log(`inline[${i}] attrs=${attrs}`);
    console.log(`  body=${body}`);
  });
}

analyze('BEFORE', 'docs/evidence/s2-csp-fix/login_body_before.html');
analyze('AFTER', 'docs/evidence/s2-csp-fix/login_body_after.html');
