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
	"strings"
	"time"
)

func main() {
	want := map[string]bool{}
	for _, id := range strings.Split(os.Getenv("CALL_IDS"), ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = true
		}
	}
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := os.Getenv("DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	pool, _ := pgxpool.New(ctx, dsn)
	defer pool.Close()
	_, _ = pool.Exec(ctx, `SELECT set_config('app.current_tenant',$1,false)`, "11111111-1111-1111-1111-111111111111")
	var blob []byte
	_ = pool.QueryRow(ctx, `SELECT api_credentials_blob FROM integrations WHERE id='ae17d28e-3f98-4288-abf9-ef1bfa978776'::uuid`).Scan(&blob)
	dec, _ := crypto.NewCredentialDecryptor(key)
	creds, _ := dec.DecryptMap(blob)
	token := creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	url := "https://api.ringba.com/v2/" + creds["account_id"] + "/calllogs"
	start := time.Now().UTC().Add(-30 * 24 * time.Hour)
	end := time.Now().UTC()
	client := &http.Client{Timeout: 60 * time.Second}
	found := map[string]map[string]interface{}{}
	offset := 0
	for {
		body, _ := json.Marshal(map[string]interface{}{
			"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
			"filters": []interface{}{}, "size": 1000, "offset": offset,
			"valueColumns": []map[string]string{{"column": "inboundCallId"}, {"column": "conversionAmount"}, {"column": "payoutAmount"}},
		})
		req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		req.Header.Set("Authorization", "Token "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, _ := client.Do(req)
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		var p struct {
			Report struct {
				Records []map[string]interface{} `json:"records"`
			} `json:"report"`
		}
		_ = json.Unmarshal(raw, &p)
		for _, r := range p.Report.Records {
			id, _ := r["inboundCallId"].(string)
			if want[id] {
				found[id] = r
			}
		}
		if len(p.Report.Records) < 1000 {
			break
		}
		offset += 1000
		time.Sleep(220 * time.Millisecond)
	}
	for id := range want {
		r := found[id]
		if r == nil {
			fmt.Printf("MISS %s\n", id)
			continue
		}
		fmt.Printf("HIT %s conversionAmount=%v payoutAmount=%v\n", id, r["conversionAmount"], r["payoutAmount"])
	}
}
