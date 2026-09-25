import { createHmac, randomUUID } from "node:crypto";
const secret = "dev_ringba_hmac_secret_change_me";
const base = "http://localhost:8080/webhooks/ringba/dev_ringba_token_change_me";
for (let i = 0; i < 8; i++) {
  const callId = `smoke-b4size-${randomUUID()}`;
  const q = `CallID=${callId}&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50`;
  const sig = createHmac("sha256", secret).update(q, "utf8").digest("hex");
  const res = await fetch(`${base}?${q}`, { headers: { "X-Ringba-Signature": sig } });
  console.log(callId, res.status);
}
