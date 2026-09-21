// Ringba write-back Phase 0.5 — READ-ONLY two unknowns.
// Calls ONLY POST …/calllogs. Never convert/void/payments/annotate/webhooks.
// Never prints tokens or ANI. Does not write to the G-void call.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	gvoidCallID   = "RGB0B96293B9F8915CA75C5CCDCF7BCFD0CFAAEDB9CV3NYI01"
	integrationID = "ce6fd338-af48-47d8-850a-bd10b855e2f4"
	tenantID      = "11111111-1111-1111-1111-111111111111"
)

func main() {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := envOr("DATABASE_URL", "postgres://callloom:callloom_dev@postgres:5432/callloom?sslmode=disable")
	if key == "" {
		fatal("CREDENTIAL_ENCRYPTION_KEY required")
	}
	integ := envOr("INTEGRATION_ID", integrationID)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	must(err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `SELECT set_config('app.current_tenant', $1, false)`, tenantID)
	must(err)

	var blob []byte
	must(pool.QueryRow(ctx, `SELECT api_credentials_blob FROM integrations WHERE id=$1::uuid`, integ).Scan(&blob))
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
	fmt.Println("PHASE=0.5 READ-ONLY BASE=https://api.ringba.com/v2/{accountId} TOKEN=REDACTED")
	fmt.Println("ENDPOINTS=POST /calllogs")
	fmt.Println("NO_WRITE_ENDPOINTS")
	fmt.Printf("GVOID_ID=%s\n", gvoidCallID)

	u1Cols := []string{
		"inboundCallId", "campaignName",
		"hasConverted", "conversionAmount", "payoutAmount",
		"wasConversionAdjusted", "wasPayoutAdjusted", "hasConnected",
	}
	minCols := []string{"inboundCallId", "hasConverted", "conversionAmount"}
	fullCols := []string{
		"inboundCallId", "campaignName", "campaignId", "targetName",
		"hasConverted", "conversionAmount", "payoutAmount",
		"wasConversionAdjusted", "wasPayoutAdjusted", "hasConnected",
		"hasAnnotations", "hasPayout",
	}

	// U1.1 three spaced reads
	fmt.Println("U1_REPEAT_START")
	r1 := mustFind(client, base, token, gvoidCallID, u1Cols)
	printU1("R1", time.Now().UTC(), r1)
	fmt.Println("U1_SLEEP_30s")
	time.Sleep(30 * time.Second)
	r2 := mustFind(client, base, token, gvoidCallID, u1Cols)
	printU1("R2", time.Now().UTC(), r2)
	fmt.Println("U1_SLEEP_180s")
	time.Sleep(180 * time.Second)
	r3 := mustFind(client, base, token, gvoidCallID, u1Cols)
	printU1("R3", time.Now().UTC(), r3)
	fmt.Println("U1_REPEAT_END")

	sameHC := samePresentVal(r1, r2, "hasConverted") && samePresentVal(r2, r3, "hasConverted")
	sameAmt := samePresentVal(r1, r2, "conversionAmount") && samePresentVal(r2, r3, "conversionAmount")
	fmt.Printf("U1_ACROSS_THREE hasConverted_identical=%v conversionAmount_identical=%v\n", sameHC, sameAmt)

	// U1.2 minimal vs full, back-to-back
	fmt.Println("U1_MIN_VS_FULL_START")
	minRec := mustFind(client, base, token, gvoidCallID, minCols)
	printU1("MIN", time.Now().UTC(), minRec)
	time.Sleep(250 * time.Millisecond)
	fullRec := mustFind(client, base, token, gvoidCallID, fullCols)
	printU1("FULL", time.Now().UTC(), fullRec)
	fmt.Printf("U1_MIN_VS_FULL hasConverted_identical=%v conversionAmount_identical=%v\n",
		samePresentVal(minRec, fullRec, "hasConverted"),
		samePresentVal(minRec, fullRec, "conversionAmount"))
	fmt.Println("U1_MIN_VS_FULL_END")

	// U2 scan 14d
	scanCols := []string{
		"inboundCallId", "campaignName", "campaignId", "targetName",
		"hasConverted", "hasConnected", "wasBlocked",
		"blockReason", "noConversionReason", "noPayoutReason",
		"incompleteCallReason", "offlineConversionUploaded",
		"conversionAmount", "payoutAmount",
		"wasConversionAdjusted", "wasPayoutAdjusted", "hasAnnotations",
		"hasPreviouslyConnected", "isIncomplete",
	}
	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-14 * 24 * time.Hour)
	fmt.Printf("U2_WINDOW start=%s end=%s\n", start.Format(time.RFC3339), end.Format(time.RFC3339))
	recs, pages, scanned := scanCalllogs(client, base, token, start, end, scanCols, 8)
	fmt.Printf("U2_SCAN pages=%d records=%d scanned=%d\n", pages, len(recs), scanned)

	reasonFields := []string{
		"noConversionReason", "noPayoutReason", "incompleteCallReason",
		"offlineConversionUploaded", "blockReason",
	}
	fmt.Println("U2_PRESENCE_START")
	for _, f := range reasonFields {
		present := 0
		var ex map[string]interface{}
		valCounts := map[string]int{}
		for _, r := range recs {
			if hasKey(r, f) {
				present++
				v := fmt.Sprint(r[f])
				valCounts[v]++
				if ex == nil {
					ex = r
				}
			}
		}
		fmt.Printf("U2_FIELD %s present=%d/%d distinct_vals=%d\n", f, present, len(recs), len(valCounts))
		if ex != nil {
			fmt.Printf("U2_EXAMPLE_FIELD=%s\n", f)
			printRecord("EX_"+f, ex)
			ks := make([]string, 0, len(valCounts))
			for k := range valCounts {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			for _, k := range ks {
				fmt.Printf("U2_VALCOUNT field=%s val=%q n=%d\n", f, k, valCounts[k])
			}
		} else {
			fmt.Printf("U2_FIELD %s ZERO_PRESENT cannot characterise from this window\n", f)
		}
	}
	fmt.Println("U2_PRESENCE_END")

	fmt.Println("U2_TYPE_B_START")
	typeB := 0
	shown := 0
	for _, r := range recs {
		id, _ := r["inboundCallId"].(string)
		if id == gvoidCallID {
			continue
		}
		if !isYes(r["hasConnected"]) {
			continue
		}
		if hasKey(r, "hasConverted") && isYes(r["hasConverted"]) {
			continue
		}
		if hasKey(r, "blockReason") && strings.TrimSpace(fmt.Sprint(r["blockReason"])) != "" {
			continue
		}
		if hasKey(r, "wasBlocked") && isYes(r["wasBlocked"]) {
			continue
		}
		typeB++
		if shown < 5 {
			shown++
			fmt.Printf("TYPE_B_SAMPLE n=%d\n", shown)
			printRecord(fmt.Sprintf("TYPE_B_%d", shown), r)
		}
	}
	fmt.Printf("TYPE_B_COUNT=%d shown=%d\n", typeB, shown)
	fmt.Println("U2_TYPE_B_END")
	fmt.Println("PHASE_05_READ_ONLY_COMPLETE")
}

func printU1(label string, t time.Time, rec map[string]interface{}) {
	fmt.Printf("%s time=%s hasConverted_present=%v val=%v conversionAmount_present=%v val=%v payoutAmount_present=%v val=%v wasConversionAdjusted_present=%v val=%v wasPayoutAdjusted_present=%v val=%v hasConnected_present=%v val=%v keys=%s campaign=%v\n",
		label, t.Format(time.RFC3339),
		hasKey(rec, "hasConverted"), valOr(rec, "hasConverted"),
		hasKey(rec, "conversionAmount"), valOr(rec, "conversionAmount"),
		hasKey(rec, "payoutAmount"), valOr(rec, "payoutAmount"),
		hasKey(rec, "wasConversionAdjusted"), valOr(rec, "wasConversionAdjusted"),
		hasKey(rec, "wasPayoutAdjusted"), valOr(rec, "wasPayoutAdjusted"),
		hasKey(rec, "hasConnected"), valOr(rec, "hasConnected"),
		keyList(rec), valOr(rec, "campaignName"))
	fmt.Printf("%s_JSON ", label)
	printRecord(label, rec)
}

func samePresentVal(a, b map[string]interface{}, k string) bool {
	pa, pb := hasKey(a, k), hasKey(b, k)
	if pa != pb {
		return false
	}
	if !pa {
		return true
	}
	return fmt.Sprint(a[k]) == fmt.Sprint(b[k])
}

func mustFind(client *http.Client, base, token, callID string, cols []string) map[string]interface{} {
	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-14 * 24 * time.Hour)
	recs, _, _ := scanCalllogs(client, base, token, start, end, cols, 8)
	for _, r := range recs {
		if id, _ := r["inboundCallId"].(string); id == callID {
			return r
		}
	}
	fatal("call ID not found in 14d call log: " + callID)
	return nil
}

func scanCalllogs(client *http.Client, base, token string, start, end time.Time, cols []string, maxPages int) ([]map[string]interface{}, int, int) {
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}
	var all []map[string]interface{}
	offset := 0
	pages := 0
	scanned := 0
	for page := 0; page < maxPages; page++ {
		body := map[string]interface{}{
			"reportStart":  start.Format(time.RFC3339),
			"reportEnd":    end.Format(time.RFC3339),
			"filters":      []interface{}{},
			"size":         1000,
			"offset":       offset,
			"valueColumns": vc,
		}
		st, _, raw, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
		pages++
		if st != 200 {
			fmt.Printf("CALLLOGS_HTTP=%d page=%d body=%s\n", st, page, snip(redactSecrets(raw), 400))
			break
		}
		var p struct {
			Report struct {
				Records []map[string]interface{} `json:"records"`
			} `json:"report"`
		}
		_ = json.Unmarshal(raw, &p)
		n := len(p.Report.Records)
		scanned += n
		fmt.Printf("CALLLOGS page=%d records=%d offset=%d cols=%d\n", page, n, offset, len(cols))
		all = append(all, p.Report.Records...)
		if n < 1000 {
			break
		}
		offset += 1000
		time.Sleep(250 * time.Millisecond)
	}
	return all, pages, scanned
}

func printRecord(label string, rec map[string]interface{}) {
	red := redactRecord(rec)
	keys := make([]string, 0, len(red))
	for k := range red {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("%s keys=%s\n", label, strings.Join(keys, ","))
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(red)
}

func keyList(rec map[string]interface{}) string {
	keys := make([]string, 0, len(rec))
	for k := range rec {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func isYes(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || strings.EqualFold(t, "yes") || t == "1"
	case float64:
		return t != 0
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		return strings.EqualFold(s, "true") || strings.EqualFold(s, "yes") || s == "1"
	}
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
	raw, _ = io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	return resp.StatusCode, resp.Header.Get("Location"), raw, resp.Header.Get("Content-Type")
}

func redactSecrets(b []byte) []byte {
	return []byte(strings.ReplaceAll(string(b), "Token ", "Token REDACTED"))
}

func redactRecord(rec map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range rec {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "phone") || strings.Contains(lk, "ani") || k == "callerId" || strings.Contains(lk, "inboundnumber") {
			out[k] = "REDACTED_PII"
			continue
		}
		out[k] = v
	}
	return out
}

func hasKey(m map[string]interface{}, k string) bool {
	if m == nil || k == "" {
		return false
	}
	_, ok := m[k]
	return ok
}

func valOr(m map[string]interface{}, k string) interface{} {
	if m == nil || k == "" {
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
