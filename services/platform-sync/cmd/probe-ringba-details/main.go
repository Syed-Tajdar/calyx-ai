// Probe Ringba calllogs/details for one inboundCallId (D8 A1).
// Never prints api_token / ANI / account_id.
package main

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

func main() {
	integrationID := env("INTEGRATION_ID", "ae17d28e-3f98-4288-abf9-ef1bfa978776")
	callID := os.Getenv("CALL_ID")
	if callID == "" {
		fatal("CALL_ID required")
	}
	dsn := env("DATABASE_URL", "postgres://callloom:callloom_dev@localhost:5432/callloom?sslmode=disable")
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
		fatal("load", err)
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
		fatal("missing creds")
	}

	base := strings.TrimRight(env("RINGBA_BASE_URL", "https://api.ringba.com/v2"), "/")
	client := &http.Client{Timeout: 30 * time.Second}
	root := fmt.Sprintf("%s/%s", base, accountID)

	type attempt struct {
		label  string
		method string
		url    string
		body   map[string]interface{}
	}
	attempts := []attempt{
		// Official July 2023 release notes: GET …/calllogs/details
		{"GET query InboundCallIds", http.MethodGet, root + "/calllogs/details?" + url.Values{"InboundCallIds": {callID}}.Encode(), nil},
		{"GET query inboundCallIds", http.MethodGet, root + "/calllogs/details?" + url.Values{"inboundCallIds": {callID}}.Encode(), nil},
		{"GET query id", http.MethodGet, root + "/calllogs/details?" + url.Values{"id": {callID}}.Encode(), nil},
		{"POST InboundCallIds", http.MethodPost, root + "/calllogs/details", map[string]interface{}{"InboundCallIds": []string{callID}}},
		{"POST inboundCallIds", http.MethodPost, root + "/calllogs/details", map[string]interface{}{"inboundCallIds": []string{callID}}},
	}

	for _, a := range attempts {
		code, raw := do(client, a.method, a.url, token, a.body)
		fmt.Printf("attempt=%q http=%d contains_id=%v snip=%s\n",
			a.label, code, strings.Contains(string(raw), callID), redact(string(raw), accountID))
		if code == 200 && strings.Contains(string(raw), callID) {
			printFlags(raw)
			os.Exit(0)
		}
	}

	// Fallback A1: filtered Get Call Log for the same id (still proves id round-trip).
	end := time.Now().UTC()
	start := end.Add(-72 * time.Hour)
	code, raw := do(client, http.MethodPost, root+"/calllogs", token, map[string]interface{}{
		"reportStart": start.Format(time.RFC3339),
		"reportEnd":   end.Format(time.RFC3339),
		"filters":     []interface{}{},
		"size":        1000,
		"offset":      0,
		"valueColumns": []map[string]string{
			{"column": "inboundCallId"},
			{"column": "callDt"},
			{"column": "conversionAmount"},
			{"column": "tag:InboundNumber:State"},
		},
	})
	found := strings.Contains(string(raw), callID)
	fmt.Printf("fallback_calllogs_window http=%d contains_id=%v snip=%s\n",
		code, found, redact(string(raw), accountID))
	if code == 200 && found {
		printFlags(raw)
		fmt.Println("D8_MODE=calllogs_window_contains_id")
		os.Exit(0)
	}
	os.Exit(2)
}

func do(client *http.Client, method, rawURL, token string, body map[string]interface{}) (int, []byte) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, rawURL, rdr)
	if err != nil {
		fatal(err)
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
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return resp.StatusCode, b
}

func printFlags(raw []byte) {
	s := string(raw)
	for _, k := range []string{"inboundCallId", "callDt", "conversionAmount", "campaignId", "tag:InboundNumber:State"} {
		fmt.Printf("raw_has_%s=%v\n", strings.ReplaceAll(k, ":", "_"), strings.Contains(s, `"`+k+`"`))
	}
	fmt.Printf("isSuccessful_true=%v\n", strings.Contains(s, `"isSuccessful":true`) || strings.Contains(s, `"isSuccessful": true`))
}

func redact(s, accountID string) string {
	s = strings.ReplaceAll(s, accountID, "{accountId}")
	if len(s) > 220 {
		s = s[:220] + "…"
	}
	return s
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fatal(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
