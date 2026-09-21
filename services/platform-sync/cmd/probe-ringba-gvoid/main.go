// Ringba write-back G-Void — read-back signal after void on the Phase G sandbox call.
// Research only. Writes: void + payments/override $0 on ONE inbound call ID.
// Never webhooks. Never other RGB IDs. Never prints tokens or ANI.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sandboxCallID   = "RGB0B96293B9F8915CA75C5CCDCF7BCFD0CFAAEDB9CV3NYI01"
	sandboxCamp     = "Calyx-Sandbox-DO-NOT-USE"
	sandboxTargetID = "TAd5332f67bf994754b8a23b8847feb413"
	integrationID   = "ce6fd338-af48-47d8-850a-bd10b855e2f4"
	tenantID        = "11111111-1111-1111-1111-111111111111"
)

func main() {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := envOr("DATABASE_URL", "postgres://callloom:callloom_dev@postgres:5432/callloom?sslmode=disable")
	if key == "" {
		fatal("CREDENTIAL_ENCRYPTION_KEY required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	must(err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `SELECT set_config('app.current_tenant', $1, false)`, tenantID)
	must(err)

	var blob []byte
	must(pool.QueryRow(ctx, `SELECT api_credentials_blob FROM integrations WHERE id=$1::uuid`, integrationID).Scan(&blob))
	dec, err := crypto.NewCredentialDecryptor(key)
	must(err)
	creds, err := dec.DecryptMap(blob)
	must(err)
	token := creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	accountID := creds["account_id"]
	if token == "" || accountID == "" {
		fatal("missing token/account_id")
	}

	base := "https://api.ringba.com/v2/" + accountID
	client := &http.Client{Timeout: 90 * time.Second}
	fmt.Printf("PHASE=G-VOID BASE=https://api.ringba.com/v2/{accountId} TOKEN=REDACTED SANDBOX=%s\n", sandboxCallID)

	cols := []string{
		"inboundCallId", "campaignName", "campaignId", "publisherName", "targetName", "targetId",
		"conversionAmount", "payoutAmount", "hasConverted", "hasPayout",
		"wasConversionAdjusted", "wasPayoutAdjusted",
	}

	v0 := mustFind(client, base, token, sandboxCallID, cols)
	camp, _ := v0["campaignName"].(string)
	tgtID, _ := v0["targetId"].(string)
	gotID, _ := v0["inboundCallId"].(string)
	printStep("V0", v0)
	if gotID != sandboxCallID {
		fatal(fmt.Sprintf("STOP: inboundCallId %q is not the G-Void target — no writes", gotID))
	}
	if !strings.EqualFold(camp, sandboxCamp) {
		fatal(fmt.Sprintf("STOP: campaign %q is not sandbox — no writes", camp))
	}
	if tgtID != sandboxTargetID {
		fatal(fmt.Sprintf("STOP: targetId %q is not sandbox — no writes", tgtID))
	}
	if !approx(asFloat(v0["conversionAmount"]), 1.00) {
		fatal(fmt.Sprintf("STOP: V0 conv=%v expected ≈1.00 — no writes", valOr(v0, "conversionAmount")))
	}
	if !hasKey(v0, "hasConverted") || !isTrue(v0["hasConverted"]) {
		fatal(fmt.Sprintf("STOP: V0 hasConverted present=%v val=%v expected present true — no writes", hasKey(v0, "hasConverted"), valOr(v0, "hasConverted")))
	}

	voidAmt := asFloat(v0["conversionAmount"])
	payAmt := asFloat(v0["payoutAmount"])
	voidBody := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"voidReason":          "CALYX-SANDBOX G-Void read-back signal",
		"voidConversion":      true,
		"voidPayout":          true,
		"voidConverionAmount": voidAmt,
		"voidPayoutAmount":    payAmt,
	}
	st, loc, raw, _ := do(client, http.MethodPost, base+"/calls/void", token, voidBody)
	fmt.Printf("V1_VOID status=%d location=%q body=%s\n", st, loc, string(redactSecrets(raw)))
	if st < 200 || st >= 300 {
		fatal(fmt.Sprintf("STOP: void HTTP %d — not continuing to override", st))
	}
	time.Sleep(2 * time.Second)

	v2 := mustFind(client, base, token, sandboxCallID, cols)
	printStep("V2", v2)
	if id, _ := v2["inboundCallId"].(string); id != sandboxCallID {
		fatal("STOP: V2 read-back id mismatch")
	}

	ov0 := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"reason":              "CALYX-SANDBOX G-Void ensure clean zero",
		"adjustConversion":    true,
		"adjustPayout":        true,
		"newConversionAmount": 0.00,
		"newPayoutAmount":     0.00,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments/override", token, ov0)
	fmt.Printf("V3_OVERRIDE_0 status=%d location=%q body=%s\n", st, loc, string(redactSecrets(raw)))
	if st < 200 || st >= 300 {
		fatal(fmt.Sprintf("STOP: override HTTP %d — V4 not run", st))
	}
	time.Sleep(2 * time.Second)

	v4 := mustFind(client, base, token, sandboxCallID, cols)
	printStep("V4", v4)
	fmt.Printf("G_VOID_LEFT_CONV=%v hasConverted_present=%v val=%v\n",
		valOr(v4, "conversionAmount"), hasKey(v4, "hasConverted"), valOr(v4, "hasConverted"))
	fmt.Println("PHASE_G_VOID_COMPLETE")
}

func printStep(label string, rec map[string]interface{}) {
	fmt.Printf("%s conv=%v hasConverted_present=%v val=%v pay=%v campaign=%v\n",
		label, valOr(rec, "conversionAmount"),
		hasKey(rec, "hasConverted"), valOr(rec, "hasConverted"),
		valOr(rec, "payoutAmount"), valOr(rec, "campaignName"))
}

func mustFind(client *http.Client, base, token, callID string, cols []string) map[string]interface{} {
	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-7 * 24 * time.Hour)
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}
	offset := 0
	for page := 0; page < 8; page++ {
		body := map[string]interface{}{
			"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
			"filters": []interface{}{}, "size": 1000, "offset": offset, "valueColumns": vc,
		}
		st, _, raw, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
		if st != 200 {
			fatal(fmt.Sprintf("calllogs HTTP %d %s", st, snip(raw, 200)))
		}
		var p struct {
			Report struct {
				Records []map[string]interface{} `json:"records"`
			} `json:"report"`
		}
		_ = json.Unmarshal(raw, &p)
		fmt.Printf("FIND page=%d records=%d offset=%d\n", page, len(p.Report.Records), offset)
		for _, row := range p.Report.Records {
			if id, _ := row["inboundCallId"].(string); id == callID {
				return row
			}
		}
		if len(p.Report.Records) < 1000 {
			break
		}
		offset += 1000
		time.Sleep(200 * time.Millisecond)
	}
	fatal("sandbox call ID not found in 7d call log")
	return nil
}

func do(client *http.Client, method, u, token string, body interface{}) (status int, location string, raw []byte, ctype string) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, rdr)
	must(err)
	req.Header.Set("Authorization", "Token "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", []byte(err.Error()), ""
	}
	defer resp.Body.Close()
	raw, _ = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return resp.StatusCode, resp.Header.Get("Location"), raw, resp.Header.Get("Content-Type")
}

func redactSecrets(b []byte) []byte {
	return []byte(strings.ReplaceAll(string(b), "Token ", "Token REDACTED"))
}

func asFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		_, _ = fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return 0
	}
}

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.001
}

func isTrue(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}

func hasKey(m map[string]interface{}, k string) bool {
	if m == nil {
		return false
	}
	_, ok := m[k]
	return ok
}

func valOr(m map[string]interface{}, k string) interface{} {
	if m == nil {
		return nil
	}
	return m[k]
}

func snip(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func must(err error) {
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "FATAL:", msg)
	os.Exit(1)
}
