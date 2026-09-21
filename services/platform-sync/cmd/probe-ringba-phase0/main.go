// Ringba write-back Phase 0 — READ-ONLY field verification.
// Calls ONLY GET …/calllogs/columns and POST …/calllogs.
// Never convert/void/payments/annotate/webhooks. Never prints tokens or ANI.
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
	bidExpired    = "The bid has expired"
)

type colMeta struct {
	ID    string
	Title string
	Type  string
	Roles []string
}

func main() {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := envOr("DATABASE_URL", "postgres://callloom:callloom_dev@postgres:5432/callloom?sslmode=disable")
	if key == "" {
		fatal("CREDENTIAL_ENCRYPTION_KEY required")
	}
	integ := envOr("INTEGRATION_ID", integrationID)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
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
	fmt.Println("PHASE=0 READ-ONLY BASE=https://api.ringba.com/v2/{accountId} TOKEN=REDACTED")
	fmt.Println("ENDPOINTS=GET /calllogs/columns ; POST /calllogs")
	fmt.Println("NO_WRITE_ENDPOINTS")

	st, _, raw, ctype := do(client, http.MethodGet, base+"/calllogs/columns", token, nil)
	fmt.Printf("P01_COLUMNS_HTTP=%d ctype=%s bytes=%d\n", st, ctype, len(raw))
	if st != 200 {
		fatal(fmt.Sprintf("columns HTTP %d %s", st, snip(redactSecrets(raw), 300)))
	}
	cols, parseNote := parseColumns(raw)
	fmt.Printf("P01_COLUMNS_COUNT=%d parse=%s\n", len(cols), parseNote)
	fmt.Println("COLUMNS_DUMP_START")
	for _, c := range cols {
		fmt.Printf("COL id=%q title=%q type=%q roles=%s\n", c.ID, c.Title, c.Type, strings.Join(c.Roles, ","))
	}
	fmt.Println("COLUMNS_DUMP_END")

	wantedTitles := []string{
		"Converted",
		"Block Reason",
		"No Conversion Reason",
		"No Payout Reason",
		"Incomplete Call Reason",
		"Offline Conversion Uploaded",
		"Blocked",
		"Connected",
		"Previously Connected",
		"Conversion Adjusted",
		"Payout Adjusted",
		"Has Annotations",
		"TCPA Shield Count",
		"TCPA Shield Blocked",
		"TCPA Shield Cost",
		"Error Message",
		"Provider Id",
	}
	fmt.Println("P01_TITLE_MATCH_START")
	confirmed := map[string]colMeta{}
	ambiguousTitles := map[string][]colMeta{}
	for _, title := range wantedTitles {
		hits := exactTitleMatches(cols, title)
		if len(hits) == 0 {
			fmt.Printf("TITLE %q matches=0\n", title)
			continue
		}
		if len(hits) > 1 {
			ambiguousTitles[title] = hits
			fmt.Printf("TITLE %q AMBIGUOUS count=%d\n", title, len(hits))
			for _, h := range hits {
				fmt.Printf("  id=%q type=%q roles=%s\n", h.ID, h.Type, strings.Join(h.Roles, ","))
			}
			continue
		}
		h := hits[0]
		confirmed[title] = h
		fmt.Printf("TITLE %q CONFIRMED id=%q type=%q roles=%s\n", title, h.ID, h.Type, strings.Join(h.Roles, ","))
	}
	fmt.Println("P01_TITLE_MATCH_END")

	// Title collisions: do not pick by role. Live accept/reject each id (size=1).
	endProbe := time.Now().UTC().Add(2 * time.Hour)
	startProbe := endProbe.Add(-14 * 24 * time.Hour)
	stillAmbiguous := false
	for title, hits := range ambiguousTitles {
		accepted := []colMeta{}
		for _, h := range hits {
			stAcc, msg := probeColumn(client, base, token, startProbe, endProbe, h.ID)
			fmt.Printf("ACCEPT_PROBE title=%q id=%q http=%d snip=%s\n", title, h.ID, stAcc, msg)
			if stAcc == 200 {
				accepted = append(accepted, h)
			}
		}
		if len(accepted) == 1 {
			confirmed[title] = accepted[0]
			fmt.Printf("TITLE %q LIVE_ACCEPT id=%q type=%q roles=%s\n", title, accepted[0].ID, accepted[0].Type, strings.Join(accepted[0].Roles, ","))
			continue
		}
		stillAmbiguous = true
		fmt.Printf("TITLE %q STILL_AMBIGUOUS live_accept_count=%d — not picking\n", title, len(accepted))
	}
	if stillAmbiguous {
		fmt.Println("STOP_AMBIGUOUS=true live accept did not uniquely resolve a title — not inferring")
		os.Exit(2)
	}

	reqIDs := uniqueStrings([]string{
		"inboundCallId", "campaignName", "campaignId", "targetName",
		"conversionAmount", "payoutAmount",
	})
	for _, c := range confirmed {
		reqIDs = append(reqIDs, c.ID)
	}
	reqIDs = uniqueStrings(reqIDs)
	sort.Strings(reqIDs)
	fmt.Printf("P02_REQUEST_COLUMNS=%s\n", strings.Join(reqIDs, ","))

	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-14 * 24 * time.Hour)
	fmt.Printf("P02_WINDOW start=%s end=%s\n", start.Format(time.RFC3339), end.Format(time.RFC3339))

	recs, pages, scanned := scanCalllogs(client, base, token, start, end, reqIDs, 8)
	fmt.Printf("P02_SCAN pages=%d records=%d scanned=%d\n", pages, len(recs), scanned)

	blockID := idFor(confirmed, "Block Reason")
	convID := idFor(confirmed, "Converted")
	connID := idFor(confirmed, "Connected")
	blockedID := idFor(confirmed, "Blocked")
	offlineID := idFor(confirmed, "Offline Conversion Uploaded")
	noConvID := idFor(confirmed, "No Conversion Reason")
	incompID := idFor(confirmed, "Incomplete Call Reason")
	tcpaCountID := idFor(confirmed, "TCPA Shield Count")
	tcpaBlockID := idFor(confirmed, "TCPA Shield Blocked")
	tcpaCostID := idFor(confirmed, "TCPA Shield Cost")

	fmt.Printf("P01_RESOLVED blockReason_id=%q converted_id=%q connected_id=%q blocked_id=%q offline_id=%q\n",
		blockID, convID, connID, blockedID, offlineID)

	var typeA map[string]interface{}
	typeACount := 0
	if blockID == "" {
		fmt.Println("TYPE_A Block Reason id not confirmed by exact title — not searching other fields")
	} else {
		for _, r := range recs {
			if fmt.Sprint(r[blockID]) == bidExpired {
				typeACount++
				if typeA == nil {
					typeA = r
				}
			}
		}
	}
	fmt.Printf("TYPE_A_COUNT=%d\n", typeACount)
	if typeA != nil {
		fmt.Println("TYPE_A_RECORD_START")
		printRecord("TYPE_A", typeA)
		fmt.Println("TYPE_A_RECORD_END")
	} else {
		fmt.Println("TYPE_A_RECORD=not-found")
	}

	var typeB map[string]interface{}
	typeBCount := 0
	if connID == "" {
		fmt.Println("TYPE_B_SKIP reason=\"Connected API id not confirmed by exact title match — not inferring a substitute\"")
	} else {
		for _, r := range recs {
			id, _ := r["inboundCallId"].(string)
			if id == gvoidCallID {
				continue
			}
			if !isYes(r[connID]) {
				continue
			}
			if convID != "" && isConvertedYes(r, convID) {
				continue
			}
			if blockID != "" && hasKey(r, blockID) && strings.TrimSpace(fmt.Sprint(r[blockID])) != "" {
				continue
			}
			if blockedID != "" && isYes(r[blockedID]) {
				continue
			}
			typeBCount++
			if typeB == nil {
				typeB = r
			}
		}
		fmt.Printf("TYPE_B_COUNT=%d\n", typeBCount)
		if typeB != nil {
			fmt.Println("TYPE_B_RECORD_START")
			printRecord("TYPE_B", typeB)
			fmt.Println("TYPE_B_RECORD_END")
		} else {
			fmt.Println("TYPE_B_RECORD=not-found")
		}
	}

	for _, id := range []string{tcpaCountID, tcpaBlockID, tcpaCostID} {
		if id == "" {
			continue
		}
		present, nonempty := 0, 0
		var sample interface{}
		for _, r := range recs {
			if hasKey(r, id) {
				present++
				s := strings.TrimSpace(fmt.Sprint(r[id]))
				if s != "" && s != "<nil>" {
					nonempty++
					if sample == nil {
						sample = r[id]
					}
				}
			}
		}
		fmt.Printf("TCPA_FIELD id=%q present=%d/%d nonempty=%d sample=%v\n", id, present, len(recs), nonempty, sample)
	}

	var gvoid map[string]interface{}
	for _, r := range recs {
		if id, _ := r["inboundCallId"].(string); id == gvoidCallID {
			gvoid = r
			break
		}
	}
	if gvoid == nil {
		gvoid = findCall(client, base, token, gvoidCallID, reqIDs)
	}
	if gvoid != nil {
		fmt.Println("GVOID_RECORD_START")
		printRecord("GVOID", gvoid)
		fmt.Println("GVOID_RECORD_END")
	} else {
		fmt.Println("GVOID_RECORD=not-found")
	}

	fmt.Printf("P05_OFFLINE_ID=%q TYPE_A_offline_present=%v val=%v TYPE_B_offline_present=%v val=%v\n",
		offlineID,
		hasKey(typeA, offlineID), valOr(typeA, offlineID),
		hasKey(typeB, offlineID), valOr(typeB, offlineID))
	fmt.Printf("P05_NOCONV_ID=%q TYPE_A_present=%v val=%v TYPE_B_present=%v val=%v\n",
		noConvID,
		hasKey(typeA, noConvID), valOr(typeA, noConvID),
		hasKey(typeB, noConvID), valOr(typeB, noConvID))
	fmt.Printf("P05_INCOMP_ID=%q TYPE_A_present=%v val=%v TYPE_B_present=%v val=%v\n",
		incompID,
		hasKey(typeA, incompID), valOr(typeA, incompID),
		hasKey(typeB, incompID), valOr(typeB, incompID))

	fmt.Println("PHASE_0_READ_ONLY_COMPLETE")
}

func parseColumns(raw []byte) ([]colMeta, string) {
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		var direct []map[string]interface{}
		if json.Unmarshal(raw, &direct) == nil && len(direct) > 0 {
			return mapsToCols(direct), "root-array"
		}
		return nil, "unmarshal-root-fail"
	}
	var arr json.RawMessage
	for _, k := range []string{"columns", "Columns", "reportColumns", "data"} {
		if v, ok := asMap[k]; ok {
			arr = v
			break
		}
	}
	if arr == nil {
		return nil, "no-columns-array keys=" + keysOf(asMap)
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(arr, &items); err != nil {
		return nil, "columns-not-array"
	}
	return mapsToCols(items), "ok"
}

func mapsToCols(items []map[string]interface{}) []colMeta {
	out := make([]colMeta, 0, len(items))
	for _, it := range items {
		c := colMeta{}
		c.ID = firstString(it, "id", "Id", "columnId", "name")
		c.Title = firstString(it, "title", "Title", "displayName", "label")
		c.Type = firstString(it, "type", "Type", "dataType")
		c.Roles = asStringSlice(it["roles"])
		if len(c.Roles) == 0 {
			c.Roles = asStringSlice(it["Roles"])
		}
		out = append(out, c)
	}
	return out
}

func exactTitleMatches(cols []colMeta, title string) []colMeta {
	want := strings.TrimSpace(strings.ToLower(title))
	var hits []colMeta
	for _, c := range cols {
		if strings.TrimSpace(strings.ToLower(c.Title)) == want {
			hits = append(hits, c)
		}
	}
	return hits
}

func idFor(confirmed map[string]colMeta, title string) string {
	if c, ok := confirmed[title]; ok {
		return c.ID
	}
	return ""
}

func probeColumn(client *http.Client, base, token string, start, end time.Time, col string) (int, string) {
	body := map[string]interface{}{
		"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
		"filters": []interface{}{}, "size": 1, "offset": 0,
		"valueColumns": []map[string]string{{"column": "inboundCallId"}, {"column": col}},
	}
	st, _, raw, _ := do(client, http.MethodPost, base+"/calllogs", token, body)
	time.Sleep(220 * time.Millisecond)
	return st, snip(redactSecrets(raw), 160)
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
		fmt.Printf("CALLLOGS page=%d records=%d offset=%d\n", page, n, offset)
		all = append(all, p.Report.Records...)
		if n < 1000 {
			break
		}
		offset += 1000
		time.Sleep(250 * time.Millisecond)
	}
	return all, pages, scanned
}

func findCall(client *http.Client, base, token, callID string, cols []string) map[string]interface{} {
	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-7 * 24 * time.Hour)
	recs, _, _ := scanCalllogs(client, base, token, start, end, cols, 8)
	for _, r := range recs {
		if id, _ := r["inboundCallId"].(string); id == callID {
			return r
		}
	}
	return nil
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

func isConvertedYes(r map[string]interface{}, convID string) bool {
	if !hasKey(r, convID) {
		return false
	}
	return isYes(r[convID])
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
		if s, ok := v.(string); ok && looksLikePhone(s) {
			out[k] = "REDACTED_PII"
			continue
		}
		out[k] = v
	}
	return out
}

func looksLikePhone(s string) bool {
	s = strings.TrimSpace(s)
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits < 10 || digits > 15 {
		return false
	}
	if strings.HasPrefix(s, "+") {
		return true
	}
	for _, r := range s {
		if r >= '0' && r <= '9' || r == ' ' || r == '-' || r == '(' || r == ')' {
			continue
		}
		return false
	}
	return true
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func asStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, it := range t {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func keysOf(m map[string]json.RawMessage) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ",")
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
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
