// Read-only: list recent Ringba sandbox-campaign calls. Never prints tokens or ANI.
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

func main() {
	integrationID := envOr("INTEGRATION_ID", "ce6fd338-af48-47d8-850a-bd10b855e2f4")
	dsn := envOr("DATABASE_URL", "postgres://callloom:callloom_dev@127.0.0.1:5432/callloom?sslmode=disable")
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	if key == "" {
		fatal("CREDENTIAL_ENCRYPTION_KEY required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	must(err)
	defer pool.Close()

	var blob []byte
	must(pool.QueryRow(ctx, `
		SELECT api_credentials_blob FROM integrations
		WHERE id = $1::uuid AND platform = 'ringba'`, integrationID).Scan(&blob))
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

	end := time.Now().UTC().Add(2 * time.Hour)
	start := end.Add(-7 * 24 * time.Hour)
	body := map[string]interface{}{
		"reportStart": start.Format(time.RFC3339),
		"reportEnd":   end.Format(time.RFC3339),
		"filters":     []interface{}{},
		"size":        1000,
		"offset":      0,
		"valueColumns": []map[string]string{
			{"column": "inboundCallId"},
			{"column": "callDt"},
			{"column": "campaignName"},
			{"column": "campaignId"},
			{"column": "publisherName"},
			{"column": "targetName"},
			{"column": "targetId"},
			{"column": "conversionAmount"},
			{"column": "payoutAmount"},
			{"column": "hasConverted"},
			{"column": "hasPayout"},
			{"column": "wasConversionAdjusted"},
			{"column": "hasRecording"},
			{"column": "hasAnnotations"},
		},
	}
	client := &http.Client{Timeout: 60 * time.Second}
	raw, st := do(client, "https://api.ringba.com/v2/"+accountID+"/calllogs", token, body)
	fmt.Printf("HTTP=%d window_hours=168 token=REDACTED\n", st)
	if st != 200 {
		fmt.Printf("ABORT snip=%s\n", snip(raw, 300))
		os.Exit(1)
	}
	var p struct {
		Report struct {
			Records []map[string]interface{} `json:"records"`
		} `json:"report"`
	}
	_ = json.Unmarshal(raw, &p)
	fmt.Printf("page_records=%d\n", len(p.Report.Records))
	n := 0
	for _, rec := range p.Report.Records {
		camp := fmt.Sprintf("%v", rec["campaignName"])
		campID := fmt.Sprintf("%v", rec["campaignId"])
		if !strings.Contains(strings.ToUpper(camp), "SANDBOX") &&
			!strings.EqualFold(campID, "CA98abf9d92be446e786f298680a3bbee6") {
			continue
		}
		n++
		id, _ := rec["inboundCallId"].(string)
		dirty := hasKey(rec, "wasConversionAdjusted") || asFloat(rec["conversionAmount"]) != 0 || asFloat(rec["payoutAmount"]) != 0
		fmt.Printf("SANDBOX_CALL id=%s callDt=%v campaign=%q target=%q targetId=%v conv=%v payout=%v hasConverted=%v hasRecording=%v dirty_money_or_adjust=%v\n",
			id, rec["callDt"], camp, rec["targetName"], rec["targetId"],
			rec["conversionAmount"], rec["payoutAmount"], rec["hasConverted"], rec["hasRecording"], dirty)
	}
	fmt.Printf("SANDBOX_MATCHES=%d old_dirty_id_still_listed_if_in_window=RGBC281C0CEFCB973E29DDA985A8F7C5EE522897406V3GHG01\n", n)
}

func do(client *http.Client, url, token string, body map[string]interface{}) ([]byte, int) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	must(err)
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	must(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return raw, resp.StatusCode
}

func hasKey(m map[string]interface{}, k string) bool {
	_, ok := m[k]
	return ok
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

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func snip(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func must(err error) {
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "fatal:", msg)
	os.Exit(1)
}
