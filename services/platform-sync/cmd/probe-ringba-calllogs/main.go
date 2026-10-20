// Probe Ringba calllogs endpoint: print column ids (geo/state), first-record keys
// (no sensitive values), and size acceptance. Never prints api_token / ANI.
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

func main() {
	integrationID := flagOrEnv("INTEGRATION_ID", "ae17d28e-3f98-4288-abf9-ef1bfa978776")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://callloom:callloom_dev@postgres:5432/callloom?sslmode=disable"
	}
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	if key == "" {
		fatal("CREDENTIAL_ENCRYPTION_KEY required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fatal("db", err)
	}
	defer pool.Close()

	var blob []byte
	err = pool.QueryRow(ctx, `
		SELECT api_credentials_blob FROM integrations
		WHERE id = $1::uuid AND platform = 'ringba' AND connection_mode = 'api_sync'`,
		integrationID).Scan(&blob)
	if err != nil {
		fatal("load integration", err)
	}
	dec, err := crypto.NewCredentialDecryptor(key)
	if err != nil {
		fatal("decryptor", err)
	}
	creds, err := dec.DecryptMap(blob)
	if err != nil {
		fatal("decrypt", err)
	}
	token := creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	accountID := creds["account_id"]
	if token == "" || accountID == "" {
		fatal("missing api_token/account_id after decrypt")
	}

	base := os.Getenv("RINGBA_BASE_URL")
	if base == "" {
		base = "https://api.ringba.com/v2"
	}
	base = strings.TrimRight(base, "/")
	client := &http.Client{Timeout: 30 * time.Second}

	fmt.Println("=== GET calllogs/columns (ids matching state|geo|region|pay|rev|user|tag) ===")
	colBody, code := do(client, http.MethodGet, fmt.Sprintf("%s/%s/calllogs/columns", base, accountID), token, nil)
	fmt.Printf("http_status=%d\n", code)
	printMatchingColumns(colBody, []string{"state", "geo", "region", "pay", "rev", "user", "tag", "city", "zip"})

	fmt.Println("=== GET calllogs/tags ===")
	tagBody, code := do(client, http.MethodGet, fmt.Sprintf("%s/%s/calllogs/tags", base, accountID), token, nil)
	fmt.Printf("http_status=%d\n", code)
	printTagTypes(tagBody)

	fmt.Println("=== POST calllogs size=1 (record keys + redacted first record) ===")
	end := time.Now().UTC()
	start := end.Add(-7 * 24 * time.Hour)
	body := map[string]interface{}{
		"reportStart": start.Format(time.RFC3339),
		"reportEnd":   end.Format(time.RFC3339),
		"filters":     []interface{}{},
		"size":        1,
		"offset":      0,
		"valueColumns": []map[string]string{
			{"column": "inboundCallId"},
			{"column": "callDt"},
			{"column": "inboundPhoneNumber"},
			{"column": "conversionAmount"},
			{"column": "payoutAmount"},
			{"column": "campaignId"},
			{"column": "publisherId"},
			{"column": "targetId"},
			{"column": "tag:InboundNumber:State"},
			{"column": "tag:InboundNumber:Region"},
		},
	}
	raw, code := do(client, http.MethodPost, fmt.Sprintf("%s/%s/calllogs", base, accountID), token, body)
	fmt.Printf("http_status=%d\n", code)
	printRecordKeys(raw)
	printRedactedFirstRecord(raw)

	if os.Getenv("SKIP_SIZE_PROBE") == "1" {
		fmt.Println("=== size probe skipped (SKIP_SIZE_PROBE=1) ===")
		return
	}
	fmt.Println("=== size probe (1, 50, 100, 500, 1000, 1001) ===")
	for _, size := range []int{1, 50, 100, 500, 1000, 1001} {
		b := map[string]interface{}{
			"reportStart": start.Format(time.RFC3339),
			"reportEnd":   end.Format(time.RFC3339),
			"filters":     []interface{}{},
			"size":        size,
			"offset":      0,
		}
		respBody, st := do(client, http.MethodPost, fmt.Sprintf("%s/%s/calllogs", base, accountID), token, b)
		n := recordCount(respBody)
		snippet := strings.TrimSpace(string(respBody))
		if len(snippet) > 120 {
			snippet = snippet[:120] + "…"
		}
		fmt.Printf("size=%d http=%d records=%d body=%s\n", size, st, n, snippet)
		time.Sleep(250 * time.Millisecond)
	}
}

func flagOrEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func do(client *http.Client, method, url, token string, body map[string]interface{}) ([]byte, int) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		fatal("request", err)
	}
	req.Header.Set("Authorization", "Token "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		fatal("http", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return b, resp.StatusCode
}

func printMatchingColumns(body []byte, needles []string) {
	var payload struct {
		Columns []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Printf("parse_columns_err=%v\n", err)
		return
	}
	n := 0
	for _, c := range payload.Columns {
		blob := strings.ToLower(c.ID + " " + c.Title)
		for _, needle := range needles {
			if strings.Contains(blob, needle) {
				fmt.Printf("column id=%q title=%q\n", c.ID, c.Title)
				n++
				break
			}
		}
	}
	fmt.Printf("matched_columns=%d total_columns=%d\n", n, len(payload.Columns))
}

func printTagTypes(body []byte) {
	// Response shape varies; print compact JSON keys + any name-like strings.
	var any interface{}
	if err := json.Unmarshal(body, &any); err != nil {
		fmt.Printf("parse_tags_err=%v\n", err)
		return
	}
	b, _ := json.Marshal(any)
	s := string(b)
	if len(s) > 2000 {
		s = s[:2000] + "…"
	}
	fmt.Printf("tags_json=%s\n", s)
}

func printRecordKeys(body []byte) {
	var payload struct {
		Report struct {
			Records []map[string]json.RawMessage `json:"records"`
		} `json:"report"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Printf("parse_records_err=%v\n", err)
		return
	}
	if len(payload.Report.Records) == 0 {
		fmt.Println("records=0")
		return
	}
	keys := make([]string, 0, len(payload.Report.Records[0]))
	for k := range payload.Report.Records[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("record_key_count=%d\n", len(keys))
	for _, k := range keys {
		fmt.Printf("key=%s\n", k)
	}
	// Non-sensitive presence flags only.
	for _, k := range []string{
		"state", "geo:state", "user:state", "caller:state", "tags",
		"inboundCallId", "callDt", "dtStamp", "conversionAmount",
		"tag:InboundNumber:State", "tag:InboundNumber:Region",
		"InboundNumber:State", "InboundNumber:Region",
	} {
		_, ok := payload.Report.Records[0][k]
		fmt.Printf("has_%s=%v\n", strings.ReplaceAll(k, ":", "_"), ok)
	}
}

func printRedactedFirstRecord(body []byte) {
	var payload struct {
		IsSuccessful  bool   `json:"isSuccessful"`
		TransactionID string `json:"transactionId"`
		Report        struct {
			PartialResult bool                     `json:"partialResult"`
			Records       []map[string]interface{} `json:"records"`
		} `json:"report"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Printf("parse_redact_err=%v\n", err)
		return
	}
	fmt.Printf("isSuccessful=%v transactionId=%q partialResult=%v record_count=%d\n",
		payload.IsSuccessful, payload.TransactionID, payload.Report.PartialResult, len(payload.Report.Records))
	if len(payload.Report.Records) == 0 {
		fmt.Println("redacted_first_record=(none)")
		return
	}
	rec := payload.Report.Records[0]
	for k, v := range rec {
		lk := strings.ToLower(k)
		if k == "inboundPhoneNumber" || strings.Contains(lk, "phonenumber") ||
			strings.HasSuffix(lk, "ani") || strings.Contains(lk, ":ani") {
			rec[k] = "REDACTED_ANI"
			continue
		}
		if s, ok := v.(string); ok && looksLikePhone(s) {
			rec[k] = "REDACTED_ANI"
		}
	}
	out, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		fmt.Printf("marshal_redact_err=%v\n", err)
		return
	}
	fmt.Println("redacted_first_record=")
	fmt.Println(string(out))
}

func looksLikePhone(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "+") {
		digits := 0
		for _, r := range s {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		return digits >= 10 && digits <= 15
	}
	// Bare digit strings: phone-length only (avoid Ringba hex/UUID ids).
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		} else if r != ' ' && r != '-' && r != '(' && r != ')' {
			return false
		}
	}
	return digits == 10 || digits == 11
}

func recordCount(body []byte) int {
	var payload struct {
		Report struct {
			Records []json.RawMessage `json:"records"`
		} `json:"report"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return -1
	}
	return len(payload.Report.Records)
}

func fatal(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
