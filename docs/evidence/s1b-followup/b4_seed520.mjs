import { createHmac, randomUUID } from "node:crypto";
const secret = "dev_ringba_hmac_secret_change_me";
const base = "http://localhost:8080/webhooks/ringba/dev_ringba_token_change_me";
const ids = [];
const N = 520;
const conc = 20;
async function one() {
  const callId = `smoke-s1bf-${randomUUID()}`;
  ids.push(callId);
  const q = `CallID=${callId}&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50`;
  const sig = createHmac("sha256", secret).update(q, "utf8").digest("hex");
  const res = await fetch(`${base}?${q}`, { headers: { "X-Ringba-Signature": sig } });
  if (!res.ok) throw new Error(`http ${res.status}`);
}
const pending = [];
for (let i = 0; i < N; i++) {
  pending.push(one());
  if (pending.length >= conc) { await Promise.all(pending.splice(0)); }
}
await Promise.all(pending);
console.log(ids.join(","));
console.error(`seeded=${ids.length}`);
