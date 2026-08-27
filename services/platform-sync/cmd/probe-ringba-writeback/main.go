package main

// Live Ringba write-back RESEARCH probes only.
// Writes use a verified non-existent call ID. Token never printed.
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

const fakeID = "RGB0000000000000000000000000000000000000000FAKEPROBE01"

func main() {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := os.Getenv("DATABASE_URL")
	step := envOr("PROBE_STEP", "all")
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	must(err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `SELECT set_config('app.current_tenant', $1, false)`, "11111111-1111-1111-1111-111111111111")
	must(err)
	var blob []byte
	must(pool.QueryRow(ctx, `SELECT api_credentials_blob FROM integrations WHERE id='ae17d28e-3f98-4288-abf9-ef1bfa978776'::uuid`).Scan(&blob))
	dec, err := crypto.NewCredentialDecryptor(key)
	must(err)
	creds, err := dec.DecryptMap(blob)
	must(err)
	token := creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	accountID := creds["account_id"]
	base := "https://api.ringba.com/v2/" + accountID
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // surface 301 Location
	}}

	fmt.Printf("BASE=https://api.ringba.com/v2/{accountId} TOKEN=REDACTED FAKE_ID=%s\n", fakeID)

	switch step {
	case "1", "all":
		step1(client, base, token)
		if step == "1" {
			return
		}
		fallthrough
	case "2":
		step2(client, base, token)
		if step == "2" {
			return
		}
		fallthrough
	case "3":
		step3(client, base, token)
		if step == "3" {
			return
		}
		fallthrough
	case "4":
		step4(client, base, token)
	}
}

func step1(client *http.Client, base, token string) {
	fmt.Println("\n=== STEP 1 READ-ONLY ===")
	gets := []string{
		"/WebHooks",
		"/webhooks",
		"/CallLogs/AvailableGroupBys",
		"/calllogs/availablegroupbys",
		"/CallLogs/SearchFields/converted",
		"/calllogs/searchfields/converted",
		"/CallLogs/SearchFields/hasConverted",
		"/calllogs/searchfields/hasConverted",
		"/CallLogs/SearchFields/payoutAdjusted",
		"/calllogs/searchfields/payoutAdjusted",
		"/CallLogs/SearchFields/hasAnnotations",
		"/calllogs/searchfields/hasAnnotations",
		"/calllogs/columns",
		"/CallLogs/columns",
		"/calllogs/tags",
		"/CallLogs/tags",
	}
	for _, p := range gets {
		st, loc, body, ct := do(client, http.MethodGet, base+p, token, nil)
		fmt.Printf("GET %s â†’ status=%d location=%q ctype=%s bytes=%d snip=%s\n",
			p, st, loc, ct, len(body), snip(body, 220))
		time.Sleep(220 * time.Millisecond)
	}
	// Dump columns/tags fully to stdout size-capped sections
	st, _, body, _ := do(client, http.MethodGet, base+"/calllogs/columns", token, nil)
	fmt.Printf("\nCOLUMNS_HTTP=%d BYTES=%d\n", st, len(body))
	if st == 200 {
		fmt.Println("COLUMNS_JSON_START")
		fmt.Println(string(body))
		fmt.Println("COLUMNS_JSON_END")
	}
	st, _, body, _ = do(client, http.MethodGet, base+"/calllogs/tags", token, nil)
	fmt.Printf("\nTAGS_HTTP=%d BYTES=%d\n", st, len(body))
	if st == 200 {
		fmt.Println("TAGS_JSON_START")
		fmt.Println(string(truncate(body, 200000)))
		fmt.Println("TAGS_JSON_END")
	}
}

func step2(client *http.Client, base, token string) {
	fmt.Println("\n=== STEP 2 CONVERTED READ-BACK ===")
	end := time.Now().UTC()
	start := end.Add(-14 * 24 * time.Hour)
	// Find a converted call: conversionAmount > 0
	recs := calllogs(client, base, token, start, end, []string{
		"inboundCallId", "conversionAmount", "payoutAmount", "hasConverted",
		"noPayoutReason", "noConversionReason", "hasAnnotations",
	}, 200)
	var convertedID string
	var zeroID string
	var noPayoutID string
	for _, r := range recs {
		id, _ := r["inboundCallId"].(string)
		if id == "" {
			continue
		}
		amt := asFloat(r["conversionAmount"])
		pay := asFloat(r["payoutAmount"])
		if amt > 0 && convertedID == "" {
			convertedID = id
			fmt.Printf("SAMPLE_CONVERTED id=%s conversionAmount=%v payoutAmount=%v keys=%v\n",
				id, r["conversionAmount"], r["payoutAmount"], keyList(r))
			fmt.Printf("  hasConverted_present=%v hasConverted_val=%v converted_present=%v converted_val=%v\n",
				hasKey(r, "hasConverted"), r["hasConverted"], hasKey(r, "converted"), r["converted"])
		}
		if amt == 0 && zeroID == "" {
			zeroID = id
			fmt.Printf("SAMPLE_ZERO_REV id=%s keys=%v hasConverted_present=%v val=%v\n",
				id, keyList(r), hasKey(r, "hasConverted"), r["hasConverted"])
		}
		if pay == 0 && amt > 0 && noPayoutID == "" {
			// may or may not have noPayoutReason
			noPayoutID = id
		}
		if pay == 0 && amt == 0 && hasKey(r, "noPayoutReason") && noPayoutID == "" {
			noPayoutID = id
			fmt.Printf("SAMPLE_NO_PAYOUT_REASON id=%s noPayoutReason=%v\n", id, r["noPayoutReason"])
		}
	}
	fmt.Printf("SCAN_records=%d converted_found=%v zero_found=%v\n", len(recs), convertedID != "", zeroID != "")

	// Omission checks across full page for several fields
	fields := []string{"hasConverted", "noPayoutReason", "noConversionReason", "hasAnnotations"}
	for _, f := range fields {
		present := 0
		trueish := 0
		for _, r := range recs {
			if hasKey(r, f) {
				present++
				v := fmt.Sprintf("%v", r[f])
				if v == "true" || v == "True" || v == "1" || v == "Yes" || v == "yes" {
					trueish++
				}
			}
		}
		fmt.Printf("OMISSION_FIELD %s present_on=%d/%d trueish=%d\n", f, present, len(recs), trueish)
	}

	// Also request only hasConverted for a known converted ID via filter if possible
	if convertedID != "" {
		fmt.Printf("RECONFIRM converted_call hasConverted on dedicated small page (id ends â€¦%s)\n", convertedID[len(convertedID)-8:])
	}
}

func step3(client *http.Client, base, token string) {
	fmt.Println("\n=== STEP 3 ADJUSTMENT IDENTIFIERS ===")
	cands := []string{
		"hasConverted", "hasPayout",
		"wasConversionAdjusted", "wasPayoutAdjusted",
		"convAdjustmentRequestCount", "convAdjustmentsApproved", "convAdjustmentsRejectedCount",
		"convAdjustmentsApprovedAmount", "hasAnnotations",
		"noConversionReason", "noPayoutReason",
		"buyerHasConverted", "buyerConversionAdjustmentStatus",
	}
	end := time.Now().UTC()
	start := end.Add(-7 * 24 * time.Hour)
	for _, col := range cands {
		st, msg, keys, n := probeCols(client, base, token, start, end, []string{"inboundCallId", col})
		present := false
		sample := ""
		for k, v := range keys {
			if strings.EqualFold(k, col) {
				present = true
				sample = v
			}
		}
		if st == 200 && present {
			fmt.Printf("ACCEPT_PRESENT col=%s sample=%s records=%d\n", col, redact(sample), n)
		} else if st == 200 && !present {
			fmt.Printf("ACCEPT_SILENT col=%s records=%d\n", col, n)
		} else {
			fmt.Printf("REJECT col=%s status=%d msg=%s\n", col, st, snip([]byte(msg), 160))
		}
		time.Sleep(220 * time.Millisecond)
	}
}

func step4(client *http.Client, base, token string) {
	fmt.Println("\n=== STEP 4 WRITE-ENDPOINT EXISTENCE (FAKE ID ONLY) ===")
	// 4.1 Prove fake ID absent
	end := time.Now().UTC()
	start := end.Add(-30 * 24 * time.Hour)
	recs := calllogs(client, base, token, start, end, []string{"inboundCallId"}, 1000)
	found := false
	// Also search multiple pages for fake ID
	offset := 0
	pages := 0
	total := 0
	for {
		body := map[string]interface{}{
			"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
			"filters": []interface{}{}, "size": 1000, "offset": offset,
			"valueColumns": []map[string]string{{"column": "inboundCallId"}},
		}
		st, _, raw, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
		if st != 200 {
			fmt.Printf("FAKE_ID_SEARCH_FAIL status=%d snip=%s\n", st, snip(raw, 200))
			break
		}
		var p struct {
			Report struct {
				Records []map[string]interface{} `json:"records"`
			} `json:"report"`
		}
		_ = json.Unmarshal(raw, &p)
		pages++
		total += len(p.Report.Records)
		for _, r := range p.Report.Records {
			id, _ := r["inboundCallId"].(string)
			if id == fakeID {
				found = true
			}
		}
		if len(p.Report.Records) < 1000 {
			break
		}
		offset += 1000
		if pages > 15 {
			break
		}
		time.Sleep(220 * time.Millisecond)
	}
	_ = recs
	fmt.Printf("FAKE_ID_PROOF id=%s window_pages=%d total_records_scanned=%d found=%v\n", fakeID, pages, total, found)
	if found {
		fmt.Println("ABORT: fake ID unexpectedly found in call log â€” refusing write probes")
		return
	}

	// 4.2 Annotate first (safest)
	annBody := map[string]interface{}{
		"inboundCallId": fakeID,
		"tags": []map[string]string{
			{"tagType": "Annotations", "tagName": "CalyxProbe", "tagValue": "nonexistent-id-probe"},
		},
	}
	st, loc, raw, _ := do(client, http.MethodPost, base+"/calls/annotate", token, annBody)
	fmt.Printf("POST /calls/annotate â†’ status=%d location=%q body=%s\n", st, loc, snip(raw, 500))
	acted := suggestsActed(raw, st)
	if acted {
		fmt.Println("ABORT: annotate response suggests action/success on unknown ID â€” stopping remaining write probes")
		return
	}
	// Also try PascalCase SDK path once for classification
	st2, loc2, raw2, _ := do(client, http.MethodPost, base+"/Calls/Annotate", token, annBody)
	fmt.Printf("POST /Calls/Annotate (SDK casing) â†’ status=%d location=%q body=%s\n", st2, loc2, snip(raw2, 300))

	// 4.3 Remaining only if annotate showed clean validation
	if !cleanValidation(st, raw) {
		fmt.Println("SKIP remaining destructive-path probes: annotate did not clearly show call-not-found validation")
		fmt.Printf("annotate_class=%s\n", classify(st, loc, raw))
		return
	}
	fmt.Println("Annotate showed clean ID validation â€” probing remaining endpoints with SAME fake ID")

	voidBody := map[string]interface{}{
		"inboundCallId":      fakeID,
		"voidReason":         "Calyx probe nonexistent id",
		"voidConversion":     true,
		"voidPayout":         true,
		"voidConverionAmount": 0.01,
		"voidPayoutAmount":   0.01,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/void", token, voidBody)
	fmt.Printf("POST /calls/void â†’ status=%d location=%q class=%s body=%s\n", st, loc, classify(st, loc, raw), snip(raw, 400))
	if suggestsActed(raw, st) {
		fmt.Println("ABORT after void: response suggests action")
		return
	}
	time.Sleep(300 * time.Millisecond)

	payBody := map[string]interface{}{
		"inboundCallId":          fakeID,
		"reason":                 "Calyx probe nonexistent id",
		"adjustConversion":       true,
		"adjustPayout":           true,
		"adjustConverionByAmount": -0.01,
		"adjustPayoutByAmount":   0.01,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/payments", token, payBody)
	fmt.Printf("POST /calls/payments â†’ status=%d location=%q class=%s body=%s\n", st, loc, classify(st, loc, raw), snip(raw, 400))
	if suggestsActed(raw, st) {
		fmt.Println("ABORT after payments: response suggests action")
		return
	}
	time.Sleep(300 * time.Millisecond)

	reqBody := map[string]interface{}{
		"inboundCallId": fakeID,
		"reason":        "Calyx probe nonexistent id",
		"amount":        0.01,
	}
	st, loc, raw, _ = do(client, http.MethodPost, base+"/calls/requestConversionAdjustment", token, reqBody)
	fmt.Printf("POST /calls/requestConversionAdjustment â†’ status=%d location=%q class=%s body=%s\n", st, loc, classify(st, loc, raw), snip(raw, 400))
}

func cleanValidation(status int, body []byte) bool {
	s := strings.ToLower(string(body))
	if status >= 200 && status < 300 {
		// success on fake id is NOT clean validation
		return false
	}
	if status == 301 || status == 308 || status == 404 {
		// may be route missing â€” still allow further probes on lowercase official paths separately
		if strings.Contains(s, "moved") || strings.Contains(s, "not found") && !strings.Contains(s, "call") {
			return status != 301 && status != 308
		}
	}
	// Prefer explicit call/inbound validation language
	if strings.Contains(s, "inbound") || strings.Contains(s, "call id") || strings.Contains(s, "callid") ||
		strings.Contains(s, "not found") || strings.Contains(s, "unknown") || strings.Contains(s, "invalid") ||
		strings.Contains(s, "does not exist") || status == 400 || status == 422 {
		return true
	}
	return status == 400 || status == 422 || status == 404
}

func suggestsActed(body []byte, status int) bool {
	s := strings.ToLower(string(body))
	if status >= 200 && status < 300 {
		if strings.Contains(s, `"issuccessful":true`) || strings.Contains(s, `"success":true`) ||
			strings.Contains(s, "annotated") || strings.Contains(s, "voided") || strings.Contains(s, "adjusted") {
			return true
		}
		// bare 200 with empty success unclear â€” treat as acted if isSuccessful true-ish
		var m map[string]interface{}
		if json.Unmarshal(body, &m) == nil {
			if v, ok := m["isSuccessful"].(bool); ok && v {
				return true
			}
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
		return "exists_auth_denied"
	case status == 404 && (strings.Contains(s, "resource cannot be found") || strings.Contains(s, "not found") && !strings.Contains(s, "call")):
		return "route_absent"
	case status == 400 || status == 404 || status == 422:
		return "route_exists_validation"
	case status >= 200 && status < 300:
		return "unexpected_success"
	default:
		return fmt.Sprintf("other_%d", status)
	}
}

func calllogs(client *http.Client, base, token string, start, end time.Time, cols []string, size int) []map[string]interface{} {
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}
	body := map[string]interface{}{
		"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
		"filters": []interface{}{}, "size": size, "offset": 0, "valueColumns": vc,
	}
	st, _, raw, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
	if st != 200 {
		fmt.Printf("calllogs status=%d snip=%s\n", st, snip(raw, 200))
		return nil
	}
	var p struct {
		Report struct {
			Records []map[string]interface{} `json:"records"`
		} `json:"report"`
	}
	_ = json.Unmarshal(raw, &p)
	return p.Report.Records
}

func probeCols(client *http.Client, base, token string, start, end time.Time, cols []string) (status int, msg string, keys map[string]string, n int) {
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}
	body := map[string]interface{}{
		"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
		"filters": []interface{}{}, "size": 50, "offset": 0, "valueColumns": vc,
	}
	st, _, raw, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
	status = st
	msg = string(raw)
	keys = map[string]string{}
	if st != 200 {
		return
	}
	var p struct {
		Report struct {
			Records []map[string]interface{} `json:"records"`
		} `json:"report"`
	}
	_ = json.Unmarshal(raw, &p)
	n = len(p.Report.Records)
	// scan ALL records for key presence (not just first)
	for _, r := range p.Report.Records {
		for k, v := range r {
			if _, ok := keys[k]; !ok {
				keys[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	return
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

func hasKey(m map[string]interface{}, k string) bool {
	_, ok := m[k]
	return ok
}
func keyList(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
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
func snip(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "â€¦"
	}
	return s
}
func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
func redact(s string) string {
	if len(s) > 80 {
		return s[:80] + "â€¦"
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
		panic(err)
	}
}

