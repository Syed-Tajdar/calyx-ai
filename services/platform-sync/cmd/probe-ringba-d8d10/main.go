package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	callID := os.Getenv("CALL_ID")
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := os.Getenv("DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, _ := pgxpool.New(ctx, dsn)
	defer pool.Close()
	var blob []byte
	_ = pool.QueryRow(ctx, `SELECT api_credentials_blob FROM integrations WHERE id='ae17d28e-3f98-4288-abf9-ef1bfa978776'::uuid`).Scan(&blob)
	dec, _ := crypto.NewCredentialDecryptor(key)
	creds, _ := dec.DecryptMap(blob)
	token := creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	accountID := creds["account_id"]
	client := &http.Client{Timeout: 45 * time.Second}
	// Window: last 24h UTC aligned to wall clock used in CH query
	end := time.Now().UTC()
	start := end.Add(-24 * time.Hour)
	type rec map[string]interface{}
	var all []rec
	offset := 0
	pages := 0
	for {
		body := map[string]interface{}{
			"reportStart": start.Format(time.RFC3339),
			"reportEnd":   end.Format(time.RFC3339),
			"filters":     []interface{}{},
			"size":        1000, "offset": offset,
			"valueColumns": []map[string]string{
				{"column": "inboundCallId"}, {"column": "callDt"}, {"column": "conversionAmount"},
				{"column": "tag:InboundNumber:State"}, {"column": "campaignId"}, {"column": "publisherId"}, {"column": "targetId"},
			},
		}
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, "https://api.ringba.com/v2/"+accountID+"/calllogs", bytes.NewReader(b))
		req.Header.Set("Authorization", "Token "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("err", err)
			os.Exit(1)
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		pages++
		if resp.StatusCode != 200 {
			fmt.Printf("page_http=%d snip=%s\n", resp.StatusCode, string(raw)[:min(120, len(raw))])
			os.Exit(2)
		}
		var payload struct {
			IsSuccessful bool `json:"isSuccessful"`
			Report       struct {
				Records []rec `json:"records"`
				Partial bool  `json:"partialResult"`
			} `json:"report"`
		}
		_ = json.Unmarshal(raw, &payload)
		all = append(all, payload.Report.Records...)
		fmt.Printf("page=%d offset=%d got=%d partial=%v\n", pages, offset, len(payload.Report.Records), payload.Report.Partial)
		if len(payload.Report.Records) < 1000 {
			break
		}
		offset += 1000
		if pages > 20 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	ids := map[string]rec{}
	for _, r := range all {
		id, _ := r["inboundCallId"].(string)
		if id != "" {
			ids[id] = r
		}
	}
	fmt.Printf("D10_ringba_api_distinct_24h=%d pages=%d windowStart=%s windowEnd=%s\n", len(ids), pages, start.Format(time.RFC3339), end.Format(time.RFC3339))
	r, ok := ids[callID]
	fmt.Printf("D8_exact_record_found=%v\n", ok)
	if ok {
		// print non-PII fields only
		fmt.Printf("D8_callDt=%v conversionAmount=%v state=%v campaignId=%v publisherId=%v targetId=%v\n",
			r["callDt"], r["conversionAmount"], r["tag:InboundNumber:State"], r["campaignId"], r["publisherId"], r["targetId"])
	}
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
