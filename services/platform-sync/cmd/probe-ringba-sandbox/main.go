package main

// Ringba write-back SANDBOX verification — research only.
// Writes target ONLY the sandbox call ID (or a verified non-existent fake ID).
// Never prints api token or ANI.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sandboxCallID  = "RGBC281C0CEFCB973E29DDA985A8F7C5EE522897406V3GHG01"
	sandboxTarget  = "CALYX - TARGET - SANDBOX - DO - NOT - USE"
	sandboxCamp    = "CALYX-SANDBOX-DO-NOT-USE"
	sandboxTargetID = "TAd5332f67bf994754b8a23b8847feb413"
	fakeID         = "RGB0000000000000000000000000000000000SANDBOXFAKE01"
	integrationID  = "ae17d28e-3f98-4288-abf9-ef1bfa978776"
	tenantID       = "11111111-1111-1111-1111-111111111111"
)

func main() {
	phase := envOr("PROBE_PHASE", "B")
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := os.Getenv("DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
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
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	fmt.Printf("PHASE=%s BASE=https://api.ringba.com/v2/{accountId} TOKEN=REDACTED SANDBOX=%s FAKE=%s\n",
		phase, sandboxCallID, fakeID)

	switch phase {
	case "B":
		phaseB(ctx, pool, client, base, token)
	case "C":
		phaseC(client, base, token)
	case "D":
		phaseD(client, base, token)
	case "E":
		phaseE(client, base, token)
	case "E_READ":
		phaseERead(client, base, token)
	case "E_RESTORE":
		phaseERestore(client, base, token)
	case "E_VOID":
		phaseEVoid(client, base, token)
	case "F1":
		phaseF1(client, base, token)
	case "F2":
		phaseF2(client, base, token, os.Getenv("WEBHOOK_ID"))
	case "F3":
		phaseF3(os.Getenv("WEBHOOK_URL"))
	case "F4":
		phaseF4(client, base, token)
	case "F5":
		phaseF5(client, base, token, os.Getenv("WEBHOOK_ID"))
	default:
		fatal("unknown PROBE_PHASE=" + phase)
	}
}

func phaseB(ctx context.Context, pool *pgxpool.Pool, client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE B — sandbox isolation (read-only) ===")
	cols := []string{
		"inboundCallId", "campaignName", "publisherName", "targetName", "targetId", "buyer",
		"conversionAmount", "payoutAmount", "hasConverted", "hasPayout",
		"profitGross", "hasRecording", "recordingUrl",
	}
	rec, n, st, raw := findCall(client, base, token, sandboxCallID, cols, 90)
	fmt.Printf("CALLLOG_LOOKUP status=%d matched_records=%d scanned_note=see_FIND\n", st, n)
	if st != 200 {
		fmt.Printf("ABORT calllogs HTTP %d body=%s\n", st, snip(raw, 400))
		return
	}
	if rec == nil {
		fmt.Println("ABORT: sandbox call ID did not resolve to exactly one record in call log")
		fmt.Println("NOTE: ID suffix V3GHG01 differs from prior …02 production IDs — report format issue")
		return
	}
	fmt.Println("RECORD_JSON_START")
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(redactRecord(rec))
	fmt.Println("RECORD_JSON_END")

	camp, _ := rec["campaignName"].(string)
	tgt, _ := rec["targetName"].(string)
	tgtID, _ := rec["targetId"].(string)
	pub, _ := rec["publisherName"].(string)
	buyer, _ := rec["buyer"].(string)
	conv := asFloat(rec["conversionAmount"])
	pay := asFloat(rec["payoutAmount"])
	_, hasConv := rec["hasConverted"]
	_, hasPay := rec["hasPayout"]

	assert("campaignName sandbox (case-insensitive)", strings.EqualFold(camp, sandboxCamp) || strings.EqualFold(camp, "Calyx-Sandbox-DO-NOT-USE"), camp)
	assert("conversionAmount==0 (key absent or 0)", conv == 0, fmt.Sprintf("present=%v val=%v", hasKey(rec, "conversionAmount"), rec["conversionAmount"]))
	assert("payoutAmount==0 (key absent or 0)", pay == 0, fmt.Sprintf("present=%v val=%v", hasKey(rec, "payoutAmount"), rec["payoutAmount"]))
	assert("hasConverted absent", !hasConv, fmt.Sprintf("present=%v val=%v", hasConv, rec["hasConverted"]))
	assert("targetName contains SANDBOX/SANBOX", strings.Contains(strings.ToUpper(tgt), "SANBOX") || strings.Contains(strings.ToUpper(tgt), "SANDBOX"), tgt)
	assert("targetId matches sandbox", tgtID == sandboxTargetID || strings.EqualFold(tgtID, sandboxTargetID), tgtID)
	fmt.Printf("NAME_NOTE operator_campaign=%q api_campaign=%q operator_target=%q api_target=%q\n", sandboxCamp, camp, sandboxTarget, tgt)
	fmt.Printf("OBSERVED publisherName=%q buyer_present=%v buyer=%q hasPayout_present=%v hasPayout_val=%v profitGross_present=%v profitGross=%v hasRecording=%v recordingUrl_present=%v\n",
		pub, hasKey(rec, "buyer"), buyer, hasPay, rec["hasPayout"], hasKey(rec, "profitGross"), rec["profitGross"], rec["hasRecording"], hasKey(rec, "recordingUrl"))
	if hasKey(rec, "recordingUrl") {
		fmt.Printf("recordingUrl_redacted=%s\n", redactURL(fmt.Sprintf("%v", rec["recordingUrl"])))
	}
	fmt.Println("CALYX_LOOKUP deferred_to_shell_clickhouse")
	_ = ctx
	_ = pool
	fmt.Println("PHASE_B_COMPLETE")
}

func phaseC(client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE C — annotate validation ===")
	// Verbatim collection body shape; only inboundCallId / tagType vary per probe.
	collectionBody := func(callID, tagType string) map[string]interface{} {
		return map[string]interface{}{
			"inboundCallId": callID,
			"tags": []map[string]string{
				{"tagType": tagType, "tagName": "Any Name", "tagValue": "Any Value"},
			},
		}
	}
	fmt.Println("COLLECTION_BODY_TEMPLATE=", `{"inboundCallId":"<id>","tags":[{"tagType":"Annotations","tagName":"Any Name","tagValue":"Any Value"}]}`)

	// Prove fake ID absent first
	found, pages, total := proveAbsent(client, base, token, fakeID, 30)
	fmt.Printf("FAKE_ID_PROOF id=%s pages=%d scanned=%d found=%v\n", fakeID, pages, total, found)
	if found {
		fmt.Println("ABORT: fake ID found in call log")
		return
	}

	// C1 — fake ID + valid collection body
	st, loc, raw, _ := do(client, http.MethodPost, base+"/calls/annotate", token, collectionBody(fakeID, "Annotations"))
	fmt.Printf("C1_FAKE_VALID status=%d location=%q body=%s\n", st, loc, snip(raw, 600))
	if suggestsActed(raw, st) {
		fmt.Println("ABORT: annotate suggested success on fake ID")
		return
	}
	time.Sleep(400 * time.Millisecond)

	// C2 — sandbox call + valid collection body (permanent annotation — disposable call only)
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/annotate", token, collectionBody(sandboxCallID, "Annotations"))
	fmt.Printf("C2_SANDBOX_VALID status=%d location=%q body=%s\n", st, loc, snip(raw, 600))
	time.Sleep(400 * time.Millisecond)
	// read back hasAnnotations
	rec, _, _, _ := findCall(client, base, token, sandboxCallID, []string{"inboundCallId", "hasAnnotations"}, 90)
	fmt.Printf("C2_READBACK hasAnnotations_present=%v val=%v\n", hasKey(rec, "hasAnnotations"), valOr(rec, "hasAnnotations"))

	time.Sleep(400 * time.Millisecond)
	// C3 — fake + invalid tagType
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/annotate", token, collectionBody(fakeID, "NOT_A_REAL_TAGTYPE_XYZ"))
	fmt.Printf("C3_FAKE_INVALID_TAGTYPE status=%d location=%q body=%s\n", st, loc, snip(raw, 600))

	fmt.Println("PHASE_C_SUMMARY: compare C1 vs C2 vs C3 above for ID validation vs body validation")
}

func phaseD(client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE D — destructive route existence (fake ID only) ===")
	found, pages, total := proveAbsent(client, base, token, fakeID, 30)
	fmt.Printf("FAKE_ID_PROOF id=%s pages=%d scanned=%d found=%v\n", fakeID, pages, total, found)
	if found {
		fmt.Println("ABORT: fake ID found")
		return
	}

	voidBody := map[string]interface{}{
		"inboundCallId":       fakeID,
		"voidReason":          "Test call voiding",
		"voidConversion":      true,
		"voidPayout":          true,
		"voidConverionAmount": 0.03,
		"voidPayoutAmount":    0.06,
	}
	st, loc, raw, _ := do(client, http.MethodPost, base+"/calls/void", token, voidBody)
	fmt.Printf("D1_VOID status=%d class=%s location=%q body=%s\n", st, classify(st, loc, raw), loc, snip(raw, 500))
	if suggestsActed(raw, st) {
		fmt.Println("ABORT: void suggested action on fake ID")
		return
	}
	time.Sleep(400 * time.Millisecond)

	payBody := map[string]interface{}{
		"inboundCallId":           fakeID,
		"reason":                  "Call payments adjusted by acct. Admin.",
		"adjustConversion":        true,
		"adjustPayout":            true,
		"adjustConverionByAmount": -0.01,
		"adjustPayoutByAmount":    0.01,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments", token, payBody)
	fmt.Printf("D2_PAYMENTS status=%d class=%s location=%q body=%s\n", st, classify(st, loc, raw), loc, snip(raw, 500))
	if suggestsActed(raw, st) {
		fmt.Println("ABORT: payments suggested action on fake ID")
		return
	}
	time.Sleep(400 * time.Millisecond)

	ovBody := map[string]interface{}{
		"inboundCallId":      fakeID,
		"reason":             "Call payments adjusted by acct. Admin.",
		"adjustConversion":   true,
		"adjustPayout":       true,
		"newConversionAmount": 0.10,
		"newPayoutAmount":    0.10,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments/override", token, ovBody)
	fmt.Printf("D3_OVERRIDE status=%d class=%s location=%q body=%s\n", st, classify(st, loc, raw), loc, snip(raw, 500))
	if suggestsActed(raw, st) {
		fmt.Println("ABORT: override suggested action on fake ID")
		return
	}
	fmt.Println("NOTE: requestConversionAdjustment/Override NOT probed (buyer-path; admin 422 on buyer columns)")
}

func phaseE(client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE E — relative vs absolute + void (SANDBOX CALL ONLY) ===")
	moneyCols := []string{
		"inboundCallId", "campaignName", "conversionAmount", "payoutAmount",
		"hasConverted", "hasPayout", "wasConversionAdjusted", "wasPayoutAdjusted",
	}
	read := func(label string) map[string]interface{} {
		rec, n, st, raw := findCall(client, base, token, sandboxCallID, moneyCols, 90)
		fmt.Printf("%s status=%d n=%d conv=%v pay=%v hasConverted=%v/%v hasPayout=%v/%v wasConvAdj=%v/%v wasPayAdj=%v/%v\n",
			label, st, n,
			valOr(rec, "conversionAmount"), valOr(rec, "payoutAmount"),
			hasKey(rec, "hasConverted"), valOr(rec, "hasConverted"),
			hasKey(rec, "hasPayout"), valOr(rec, "hasPayout"),
			hasKey(rec, "wasConversionAdjusted"), valOr(rec, "wasConversionAdjusted"),
			hasKey(rec, "wasPayoutAdjusted"), valOr(rec, "wasPayoutAdjusted"),
		)
		if st != 200 || rec == nil {
			fmt.Printf("  raw=%s\n", snip(raw, 300))
		}
		camp, _ := rec["campaignName"].(string)
		if camp != "" && !strings.EqualFold(camp, sandboxCamp) && !strings.EqualFold(camp, "Calyx-Sandbox-DO-NOT-USE") {
			fmt.Printf("ABORT: campaignName drifted to %q — refusing write\n", camp)
			os.Exit(2)
		}
		return rec
	}

	before := read("E0_BEFORE")
	if before == nil {
		fmt.Println("ABORT: cannot read sandbox call")
		return
	}
	if asFloat(before["conversionAmount"]) != 0 || asFloat(before["payoutAmount"]) != 0 {
		fmt.Printf("ABORT: sandbox not at 0/0 before E (conv=%v pay=%v)\n", before["conversionAmount"], before["payoutAmount"])
		return
	}

	// E.1 absolute override → $10
	ov := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"reason":              "CALYX-SANDBOX absolute baseline probe",
		"adjustConversion":    true,
		"adjustPayout":        true,
		"newConversionAmount": 10.00,
		"newPayoutAmount":     10.00,
	}
	st, loc, raw, _ := do(client, http.MethodPost, base+"/calls/payments/override", token, ov)
	fmt.Printf("E1_OVERRIDE_10 status=%d location=%q body=%s\n", st, loc, snip(raw, 500))
	time.Sleep(1500 * time.Millisecond)
	after1 := read("E1_AFTER")
	c1, p1 := asFloat(after1["conversionAmount"]), asFloat(after1["payoutAmount"])
	if !(approx(c1, 10) && approx(p1, 10)) {
		fmt.Printf("STOP: E1 expected 10/10 got conv=%v pay=%v — not continuing relative probe\n", c1, p1)
		return
	}

	// E.2 relative +$5
	rel := map[string]interface{}{
		"inboundCallId":           sandboxCallID,
		"reason":                  "CALYX-SANDBOX relative probe",
		"adjustConversion":        true,
		"adjustPayout":            true,
		"adjustConverionByAmount": 5.00,
		"adjustPayoutByAmount":    5.00,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments", token, rel)
	fmt.Printf("E2_PAYMENTS_PLUS5 status=%d location=%q body=%s\n", st, loc, snip(raw, 500))
	time.Sleep(1500 * time.Millisecond)
	after2 := read("E2_AFTER")
	c2, p2 := asFloat(after2["conversionAmount"]), asFloat(after2["payoutAmount"])
	switch {
	case approx(c2, 15) && approx(p2, 15):
		fmt.Println("RELATIVE_VS_ABSOLUTE: payments=RELATIVE payments/override=ABSOLUTE [PROVEN]")
	case approx(c2, 5) && approx(p2, 5):
		fmt.Println("CRITICAL: payments behaved ABSOLUTE — documentation wrong [PROVEN]")
	default:
		fmt.Printf("STOP: unexpected E2 amounts conv=%v pay=%v\n", c2, p2)
		return
	}

	// E.3 restore to 0 via absolute
	ov0 := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"reason":              "CALYX-SANDBOX restore zero",
		"adjustConversion":    true,
		"adjustPayout":        true,
		"newConversionAmount": 0.00,
		"newPayoutAmount":     0.00,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments/override", token, ov0)
	fmt.Printf("E3_RESTORE_0 status=%d location=%q body=%s\n", st, loc, snip(raw, 500))
	time.Sleep(1500 * time.Millisecond)
	after3 := read("E3_AFTER")
	if !approx(asFloat(after3["conversionAmount"]), 0) || !approx(asFloat(after3["payoutAmount"]), 0) {
		fmt.Printf("STOP: restore failed conv=%v pay=%v\n", after3["conversionAmount"], after3["payoutAmount"])
		return
	}

	// Set known non-zero then void (void needs something to void per collection examples)
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments/override", token, ov) // back to 10
	fmt.Printf("E4_PREVOID_SET10 status=%d body=%s\n", st, snip(raw, 300))
	time.Sleep(1500 * time.Millisecond)
	_ = read("E4_BEFORE_VOID")

	voidBody := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"voidReason":          "CALYX-SANDBOX void semantics probe",
		"voidConversion":      true,
		"voidPayout":          true,
		"voidConverionAmount": 10.00,
		"voidPayoutAmount":    10.00,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/void", token, voidBody)
	fmt.Printf("E4_VOID status=%d location=%q body=%s\n", st, loc, snip(raw, 500))
	time.Sleep(1500 * time.Millisecond)
	afterVoid := read("E4_AFTER_VOID")

	// Reversibility: try override back to 5
	ov5 := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"reason":              "CALYX-SANDBOX void reversibility probe",
		"adjustConversion":    true,
		"adjustPayout":        true,
		"newConversionAmount": 5.00,
		"newPayoutAmount":     5.00,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments/override", token, ov5)
	fmt.Printf("E4_REVERT_OVERRIDE_5 status=%d body=%s\n", st, snip(raw, 400))
	time.Sleep(1500 * time.Millisecond)
	afterRev := read("E4_AFTER_REVERT")
	fmt.Printf("VOID_REVERSIBLE=%v (expect ~5/5 if yes; afterVoid conv/pay were %v/%v)\n",
		approx(asFloat(afterRev["conversionAmount"]), 5) && approx(asFloat(afterRev["payoutAmount"]), 5),
		valOr(afterVoid, "conversionAmount"), valOr(afterVoid, "payoutAmount"))

	// Final restore 0
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments/override", token, ov0)
	fmt.Printf("E5_FINAL_ZERO status=%d body=%s\n", st, snip(raw, 300))
	time.Sleep(1500 * time.Millisecond)
	final := read("E5_FINAL")
	fmt.Printf("SANDBOX_RESTORED_ZERO=%v\n", approx(asFloat(final["conversionAmount"]), 0) && approx(asFloat(final["payoutAmount"]), 0))
}

func phaseERead(client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE E_READ ===")
	cols := []string{
		"inboundCallId", "campaignName", "conversionAmount", "payoutAmount",
		"hasConverted", "hasPayout", "wasConversionAdjusted", "wasPayoutAdjusted",
	}
	rec, n, st, raw := findCall(client, base, token, sandboxCallID, cols, 90)
	fmt.Printf("status=%d n=%d\n", st, n)
	if rec != nil {
		b, _ := json.Marshal(redactRecord(rec))
		fmt.Printf("RECORD=%s\n", string(b))
	} else {
		fmt.Printf("raw=%s\n", snip(raw, 300))
	}
}

func phaseERestore(client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE E_RESTORE to 0 ===")
	ov0 := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"reason":              "CALYX-SANDBOX restore zero",
		"adjustConversion":    true,
		"adjustPayout":        true,
		"newConversionAmount": 0.00,
		"newPayoutAmount":     0.00,
	}
	st, _, raw, _ := do(client, http.MethodPost, base+"/calls/payments/override", token, ov0)
	fmt.Printf("OVERRIDE_0 status=%d body=%s\n", st, snip(raw, 400))
	time.Sleep(1500 * time.Millisecond)
	phaseERead(client, base, token)
}

func phaseEVoid(client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE E_VOID continuation (reversibility + final zero) ===")
	cols := []string{
		"inboundCallId", "campaignName", "conversionAmount", "payoutAmount",
		"hasConverted", "hasPayout", "wasConversionAdjusted", "wasPayoutAdjusted",
	}
	read := func(label string) map[string]interface{} {
		rec, n, st, _ := findCall(client, base, token, sandboxCallID, cols, 90)
		fmt.Printf("%s status=%d n=%d conv=%v pay=%v hasConverted=%v/%v hasPayout=%v/%v wasConvAdj=%v/%v wasPayAdj=%v/%v\n",
			label, st, n,
			valOr(rec, "conversionAmount"), valOr(rec, "payoutAmount"),
			hasKey(rec, "hasConverted"), valOr(rec, "hasConverted"),
			hasKey(rec, "hasPayout"), valOr(rec, "hasPayout"),
			hasKey(rec, "wasConversionAdjusted"), valOr(rec, "wasConversionAdjusted"),
			hasKey(rec, "wasPayoutAdjusted"), valOr(rec, "wasPayoutAdjusted"),
		)
		return rec
	}
	afterVoid := read("E4_AFTER_VOID_NOW")
	ov5 := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"reason":              "CALYX-SANDBOX void reversibility probe",
		"adjustConversion":    true,
		"adjustPayout":        true,
		"newConversionAmount": 5.00,
		"newPayoutAmount":     5.00,
	}
	st, _, raw, _ := do(client, http.MethodPost, base+"/calls/payments/override", token, ov5)
	fmt.Printf("E4_REVERT_OVERRIDE_5 status=%d body=%s\n", st, snip(raw, 400))
	time.Sleep(1500 * time.Millisecond)
	afterRev := read("E4_AFTER_REVERT")
	fmt.Printf("VOID_REVERSIBLE=%v (expect ~5/5 if yes; afterVoid conv/pay were %v/%v)\n",
		approx(asFloat(afterRev["conversionAmount"]), 5) && approx(asFloat(afterRev["payoutAmount"]), 5),
		valOr(afterVoid, "conversionAmount"), valOr(afterVoid, "payoutAmount"))

	ov0 := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"reason":              "CALYX-SANDBOX final zero",
		"adjustConversion":    true,
		"adjustPayout":        true,
		"newConversionAmount": 0.00,
		"newPayoutAmount":     0.00,
	}
	st, _, raw, _ = do(client, http.MethodPost, base+"/calls/payments/override", token, ov0)
	fmt.Printf("E5_FINAL_ZERO status=%d body=%s\n", st, snip(raw, 300))
	time.Sleep(1500 * time.Millisecond)
	final := read("E5_FINAL")
	fmt.Printf("SANDBOX_RESTORED_ZERO=%v\n", approx(asFloat(final["conversionAmount"]), 0) && approx(asFloat(final["payoutAmount"]), 0))
}

func phaseF1(client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE F1 — create conversion webhook ===")
	st, _, before, _ := do(client, http.MethodGet, base+"/WebHooks", token, nil)
	fmt.Printf("WEBHOOKS_BEFORE status=%d body=%s\n", st, snip(redactSecrets(before), 1200))
	idsBefore := webhookIDs(before)
	fmt.Printf("WEBHOOK_IDS_BEFORE=%v\n", idsBefore)

	// Plan-specified values with collection body shape
	body := map[string]interface{}{
		"name": "CALYX-SANDBOX-CONV",
		"incomingCallIdQueryStringParameter":   "Call Token",
		"conversionAmountQueryStringParameter": "Revenue Token",
		"conversionCriteriaParameterMatch":     "Fire Conversions",
	}
	fmt.Printf("CREATE_BODY=%s\n", mustJSON(body))
	st, loc, raw, _ := do(client, http.MethodPost, base+"/WebHooks/CreateConversion", token, body)
	fmt.Printf("CREATE status=%d location=%q body=%s\n", st, loc, snip(redactSecrets(raw), 800))
	time.Sleep(500 * time.Millisecond)

	st, _, after, _ := do(client, http.MethodGet, base+"/WebHooks", token, nil)
	fmt.Printf("WEBHOOKS_AFTER status=%d body=%s\n", st, snip(redactSecrets(after), 1200))
	idsAfter := webhookIDs(after)
	fmt.Printf("WEBHOOK_IDS_AFTER=%v\n", idsAfter)
	created := diffIDs(idsBefore, idsAfter)
	fmt.Printf("CREATED_IDS=%v\n", created)
	untouched := true
	for _, id := range idsBefore {
		if !contains(idsAfter, id) {
			untouched = false
		}
	}
	fmt.Printf("PREEXISTING_UNTOUCHED=%v\n", untouched)
}

func phaseF2(client *http.Client, base, token, whID string) {
	fmt.Println("\n=== PHASE F2 — get webhook URL ===")
	if whID == "" {
		fatal("WEBHOOK_ID required")
	}
	// Collection does not list getUrl; try plan path then details GET
	st, loc, raw, _ := do(client, http.MethodPost, base+"/WebHooks/"+whID+"/getUrl", token, map[string]interface{}{})
	fmt.Printf("GETURL_POST status=%d location=%q body=%s\n", st, loc, snip(redactSecrets(raw), 800))
	st, loc, raw, _ = do(client, http.MethodGet, base+"/WebHooks/"+whID, token, nil)
	fmt.Printf("GET_DETAILS status=%d location=%q body=%s\n", st, loc, snip(redactSecrets(raw), 1200))
}

func phaseF3(whURL string) {
	fmt.Println("\n=== PHASE F3 — fire conversion webhook ===")
	// Prefer explicit URL from F2; allow empty to use known sandbox webhook path from env HOOK_ID
	hookID := os.Getenv("WEBHOOK_ID")
	if whURL == "" && hookID != "" {
		whURL = "https://webhook.ringba.com/hook/" + hookID
	}
	if whURL == "" {
		fatal("WEBHOOK_URL or WEBHOOK_ID required")
	}
	// Strip any template query from getUrl response; rebuild with real values
	if i := strings.Index(whURL, "?"); i >= 0 {
		whURL = whURL[:i]
	}
	q := url.Values{}
	q.Set("Call Token", sandboxCallID)
	q.Set("Revenue Token", "1.00")
	q.Set("Fire Conversions", "")
	fire := whURL + "?" + q.Encode()
	fmt.Printf("FIRE_URL_SHAPE=%s\n", redactURL(fire))
	fmt.Printf("FIRE_URL_FULL_REDACTED_HOST_PATH=%s?Call%%20Token=REDACTED_CALL&Revenue%%20Token=1.00&Fire%%20Conversions=\n", whURL)
	req, err := http.NewRequest(http.MethodPost, fire, nil)
	must(err)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("FIRE_ERR %v\n", err)
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	fmt.Printf("FIRE status=%d body=%s\n", resp.StatusCode, snip(raw, 500))
}

func phaseF4(client *http.Client, base, token string) {
	fmt.Println("\n=== PHASE F4 — read back conversion ===")
	rec, n, st, raw := findCall(client, base, token, sandboxCallID, []string{
		"inboundCallId", "campaignName", "conversionAmount", "payoutAmount", "hasConverted", "hasPayout",
	}, 90)
	fmt.Printf("status=%d n=%d body_snip_if_fail=%s\n", st, n, snip(raw, 200))
	fmt.Printf("hasConverted_present=%v val=%v conversionAmount=%v payoutAmount=%v\n",
		hasKey(rec, "hasConverted"), valOr(rec, "hasConverted"),
		valOr(rec, "conversionAmount"), valOr(rec, "payoutAmount"))
}

func phaseF5(client *http.Client, base, token, whID string) {
	fmt.Println("\n=== PHASE F5 — cleanup ===")
	// zero sandbox
	ov0 := map[string]interface{}{
		"inboundCallId":       sandboxCallID,
		"reason":              "CALYX-SANDBOX F5 cleanup zero",
		"adjustConversion":    true,
		"adjustPayout":        true,
		"newConversionAmount": 0.00,
		"newPayoutAmount":     0.00,
	}
	st, _, raw, _ := do(client, http.MethodPost, base+"/calls/payments/override", token, ov0)
	fmt.Printf("ZERO status=%d body=%s\n", st, snip(raw, 300))
	time.Sleep(1200 * time.Millisecond)
	rec, _, _, _ := findCall(client, base, token, sandboxCallID, []string{"inboundCallId", "conversionAmount", "payoutAmount", "hasConverted"}, 90)
	fmt.Printf("ZERO_READBACK conv=%v pay=%v hasConverted=%v/%v\n",
		valOr(rec, "conversionAmount"), valOr(rec, "payoutAmount"),
		hasKey(rec, "hasConverted"), valOr(rec, "hasConverted"))

	st, _, before, _ := do(client, http.MethodGet, base+"/WebHooks", token, nil)
	fmt.Printf("WEBHOOKS_BEFORE_DELETE status=%d ids=%v\n", st, webhookIDs(before))
	if whID != "" {
		stDel, locDel, rawDel, _ := do(client, http.MethodDelete, base+"/WebHooks/"+whID, token, nil)
		fmt.Printf("DELETE status=%d location=%q body=%s\n", stDel, locDel, snip(redactSecrets(rawDel), 400))
	} else {
		fmt.Println("NO WEBHOOK_ID — skip delete")
	}
	time.Sleep(400 * time.Millisecond)
	st, _, after, _ := do(client, http.MethodGet, base+"/WebHooks", token, nil)
	fmt.Printf("WEBHOOKS_AFTER_DELETE status=%d ids=%v body=%s\n", st, webhookIDs(after), snip(redactSecrets(after), 800))
}

// --- helpers ---

func findCall(client *http.Client, base, token, callID string, cols []string, days int) (rec map[string]interface{}, matched int, status int, raw []byte) {
	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-time.Duration(days) * 24 * time.Hour)
	vc := colsVC(cols)
	offset := 0
	var lastRaw []byte
	var lastSt int
	for page := 0; page < 20; page++ {
		body := map[string]interface{}{
			"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
			"filters": []interface{}{}, "size": 1000, "offset": offset, "valueColumns": vc,
		}
		st, _, r, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
		lastSt, lastRaw = st, r
		if st != 200 {
			fmt.Printf("FIND page=%d status=%d snip=%s\n", page, st, snip(r, 200))
			return nil, 0, st, r
		}
		var p struct {
			Report struct {
				Records []map[string]interface{} `json:"records"`
			} `json:"report"`
		}
		_ = json.Unmarshal(r, &p)
		fmt.Printf("FIND page=%d records=%d offset=%d\n", page, len(p.Report.Records), offset)
		for _, row := range p.Report.Records {
			id, _ := row["inboundCallId"].(string)
			if id == callID {
				matched++
				rec = row
			}
		}
		if matched > 0 {
			break
		}
		if len(p.Report.Records) < 1000 {
			break
		}
		offset += 1000
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Printf("FIND_RESULT callID_suffix=%s matched=%d\n", callID[len(callID)-8:], matched)
	if matched != 1 {
		return nil, matched, lastSt, lastRaw
	}
	return rec, matched, lastSt, lastRaw
}

func proveAbsent(client *http.Client, base, token, callID string, days int) (found bool, pages, total int) {
	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-time.Duration(days) * 24 * time.Hour)
	offset := 0
	for pages = 0; pages < 10; pages++ {
		body := map[string]interface{}{
			"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
			"filters": []interface{}{}, "size": 1000, "offset": offset,
			"valueColumns": []map[string]string{{"column": "inboundCallId"}},
		}
		st, _, r, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
		if st != 200 {
			fmt.Printf("ABSENT_PROOF status=%d snip=%s\n", st, snip(r, 200))
			return true, pages, total // fail closed
		}
		var p struct {
			Report struct {
				Records []map[string]interface{} `json:"records"`
			} `json:"report"`
		}
		_ = json.Unmarshal(r, &p)
		total += len(p.Report.Records)
		for _, row := range p.Report.Records {
			if id, _ := row["inboundCallId"].(string); id == callID {
				return true, pages + 1, total
			}
		}
		if len(p.Report.Records) < 1000 {
			return false, pages + 1, total
		}
		offset += 1000
		time.Sleep(200 * time.Millisecond)
	}
	return false, pages, total
}

func colsVC(cols []string) []map[string]string {
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}
	return vc
}

func do(client *http.Client, method, url, token string, body interface{}) (status int, location string, raw []byte, ctype string) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
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

func suggestsActed(body []byte, status int) bool {
	s := strings.ToLower(string(body))
	if status >= 200 && status < 300 {
		var m map[string]interface{}
		if json.Unmarshal(body, &m) == nil {
			if v, ok := m["isSuccessful"].(bool); ok && v {
				return true
			}
		}
		if strings.Contains(s, `"issuccessful":true`) {
			return true
		}
	}
	return false
}

func classify(status int, loc string, body []byte) string {
	s := strings.ToLower(string(body))
	switch {
	case status == 301 || status == 308:
		return "moved:" + loc
	case status == 401 || status == 403:
		return "forbidden"
	case status == 404 && (strings.Contains(s, "resource cannot be found") || (strings.Contains(s, "not found") && !strings.Contains(s, "call"))):
		return "absent"
	case status == 400 || status == 404 || status == 422 || status == 500:
		if strings.Contains(s, "call") || strings.Contains(s, "inbound") || strings.Contains(s, "not found") || strings.Contains(s, "invalid") || strings.Contains(s, "failed") {
			return "live_validation"
		}
		return fmt.Sprintf("live_other_%d", status)
	case status >= 200 && status < 300:
		return "unexpected_success"
	default:
		return fmt.Sprintf("other_%d", status)
	}
}

func assert(name string, ok bool, got string) {
	if ok {
		fmt.Printf("ASSERT PASS %s got=%q\n", name, got)
	} else {
		fmt.Printf("ASSERT FAIL %s got=%q\n", name, got)
	}
}

func redactRecord(rec map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range rec {
		if k == "inboundPhoneNumber" || k == "callerId" || strings.Contains(strings.ToLower(k), "phone") {
			out[k] = "REDACTED_PII"
			continue
		}
		if k == "recordingUrl" {
			out[k] = redactURL(fmt.Sprintf("%v", v))
			continue
		}
		out[k] = v
	}
	return out
}

func redactURL(u string) string {
	// keep host + path shape; strip query secrets
	if i := strings.Index(u, "?"); i >= 0 {
		return u[:i] + "?REDACTED"
	}
	if len(u) > 120 {
		return u[:120] + "…"
	}
	return u
}

func redactSecrets(b []byte) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, "Token ", "Token REDACTED")
	return []byte(s)
}

func webhookIDs(raw []byte) []string {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	arr, _ := m["webHooks"].([]interface{})
	if arr == nil {
		arr, _ = m["webhooks"].([]interface{})
	}
	out := []string{}
	for _, it := range arr {
		obj, _ := it.(map[string]interface{})
		if id, ok := obj["id"].(string); ok {
			out = append(out, id)
		}
	}
	return out
}

func diffIDs(before, after []string) []string {
	out := []string{}
	for _, a := range after {
		if !contains(before, a) {
			out = append(out, a)
		}
	}
	return out
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}

func asFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
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

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "FATAL:", msg)
	os.Exit(1)
}
