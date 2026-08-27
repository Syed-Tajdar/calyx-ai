package main

// Probe each candidate valueColumn alone; print accept/reject + sample (no ANI/token).
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

func main() {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := os.Getenv("DATABASE_URL")
	integrationID := envOr("INTEGRATION_ID", "ae17d28e-3f98-4288-abf9-ef1bfa978776")
	tenantID := envOr("TENANT_ID", "11111111-1111-1111-1111-111111111111")
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
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
	client := &http.Client{Timeout: 45 * time.Second}
	url := "https://api.ringba.com/v2/" + accountID + "/calllogs"
	end := time.Now().UTC()
	start := end.Add(-7 * 24 * time.Hour)

	// Always include a known-good id column so we get records.
	anchor := "inboundCallId"

	candidates := []string{
		"campaignName", "publisherName", "targetName",
		"buyer", "buyerName", "buyerId",
		"callLengthInSeconds", "connectedCallLengthInSeconds", "connectedCallLength",
		"timeToConnectInSeconds", "timeToConnect",
		"hasRecording", "recordingUrl", "recording",
		"converted", "isConverted", "duplicate", "isDuplicate",
		"profit", "profitGross", "profitNetTelco", "totalCost", "telcoCost",
		"callConnectedDt", "callCompleteDt", "connectedDt", "completeDt",
		"targetNumber", "targetGroup", "targetGroupId",
		"publisherSubId", "targetSubId", "buyerSubId",
		"paidOut", "noPayoutReason",
	}

	accepted := []string{}
	rejected := []string{}

	for _, col := range candidates {
		status, msg, keys, sample, n, ms := probe(client, url, token, start, end, []string{anchor, col})
		// Accept if HTTP 200 and column appears as a key (even empty value)
		present := false
		for k := range keys {
			if strings.EqualFold(k, col) {
				present = true
				sample = keys[k]
				break
			}
		}
		if status == 200 && present {
			accepted = append(accepted, col)
			fmt.Printf("ACCEPT col=%s http=%d ms=%d records=%d sample=%s\n", col, status, ms, n, redact(sample))
		} else if status == 200 && !present {
			fmt.Printf("SILENT_DROP col=%s http=%d ms=%d keys=%v msg=%s\n", col, status, ms, keyList(keys), snip([]byte(msg), 120))
			rejected = append(rejected, col+" (silent)")
		} else {
			rejected = append(rejected, col)
			fmt.Printf("REJECT col=%s http=%d msg=%s\n", col, status, snip([]byte(msg), 160))
		}
		time.Sleep(220 * time.Millisecond) // stay under 5/sec
	}

	fmt.Println("\n=== SUMMARY accepted ===")
	for _, c := range accepted {
		fmt.Println(c)
	}
	fmt.Println("=== SUMMARY rejected/silent ===")
	for _, c := range rejected {
		fmt.Println(c)
	}

	// Final: all accepted + baseline, size before/after
	baseline := []string{
		"inboundCallId", "callDt", "inboundPhoneNumber",
		"conversionAmount", "payoutAmount",
		"campaignId", "publisherId", "targetId",
		"tag:InboundNumber:State",
	}
	fmt.Println("\n=== SIZE before (baseline) size=1000 ===")
	st, _, _, _, n, ms := probe(client, url, token, start, end, baseline)
	fmt.Printf("http=%d records=%d duration_ms=%d\n", st, n, ms)
	// measure bytes via separate call
	b1, ms1 := probeBytes(client, url, token, start, end, baseline, 1000)
	fmt.Printf("bytes=%d duration_ms=%d\n", b1, ms1)

	extended := append(append([]string{}, baseline...), accepted...)
	fmt.Println("=== SIZE after (baseline+accepted) size=1000 ===")
	b2, ms2 := probeBytes(client, url, token, start, end, extended, 1000)
	fmt.Printf("bytes=%d duration_ms=%d cols=%d\n", b2, ms2, len(extended))

	// Dump one record keys for accepted set (redacted)
	fmt.Println("\n=== SAMPLE RECORD KEYS (extended, first record) ===")
	_, _, keys, _, _, _ := probe(client, url, token, start, end, extended)
	for k, v := range keys {
		fmt.Printf("  %s = %s\n", k, redact(v))
	}
}

func probe(client *http.Client, url, token string, start, end time.Time, cols []string) (status int, msg string, keys map[string]string, sample string, n int, ms int64) {
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}
	body := map[string]interface{}{
		"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
		"filters": []interface{}{}, "size": 5, "offset": 0, "valueColumns": vc,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")
	t0 := time.Now()
	resp, err := client.Do(req)
	must(err)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	resp.Body.Close()
	ms = time.Since(t0).Milliseconds()
	status = resp.StatusCode
	msg = string(raw)
	keys = map[string]string{}
	if status != 200 {
		return
	}
	var payload struct {
		Report struct {
			Records []map[string]interface{} `json:"records"`
		} `json:"report"`
	}
	_ = json.Unmarshal(raw, &payload)
	n = len(payload.Report.Records)
	if n > 0 {
		for k, v := range payload.Report.Records[0] {
			keys[k] = fmt.Sprintf("%v", v)
		}
	}
	return
}

func probeBytes(client *http.Client, url, token string, start, end time.Time, cols []string, size int) (int, int64) {
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}
	body := map[string]interface{}{
		"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
		"filters": []interface{}{}, "size": size, "offset": 0, "valueColumns": vc,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")
	t0 := time.Now()
	resp, err := client.Do(req)
	must(err)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	resp.Body.Close()
	return len(raw), time.Since(t0).Milliseconds()
}

func keyList(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
func redact(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "<nil>" {
		return "(empty)"
	}
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits >= 10 {
		return "(redacted)"
	}
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}
func snip(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
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
