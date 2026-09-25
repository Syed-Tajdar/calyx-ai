import { createHmac, randomUUID } from "node:crypto";
const secret = "dev_ringba_hmac_secret_change_me";
const base = "http://localhost:8080/webhooks/ringba/dev_ringba_token_change_me";
const ids = [];
for (let i = 0; i < 6; i++) {
  const callId = `smoke-b6shut-${randomUUID()}`;
  ids.push(callId);
  const q = `CallID=${callId}&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50`;
  const sig = createHmac("sha256", secret).update(q, "utf8").digest("hex");
  await fetch(`${base}?${q}`, { headers: { "X-Ringba-Signature": sig } });
}
console.log(ids.join(","));
