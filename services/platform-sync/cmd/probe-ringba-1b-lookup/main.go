// Read-only: LookupCall + raw key presence for one inboundCallId. Never prints tokens or ANI.
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

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	wantID := envOr("INBOUND_CALL_ID", "RGB512060522F815F149F10AD1B143CCF4BD5E5F709V3LIN01")
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

	rb := adapter.NewRingbaAdapter(adapter.NewHTTPClient())
	call, err := rb.LookupCall(ctx, creds, wantID)
	if err != nil {
		fmt.Printf("LOOKUP_ERR=%s\n", err.Error())
	} else {
		fmt.Printf("LOOKUP_OK id=%s campaignId=%s campaignName=%q hasConverted=%v hasConvertedPresent=%v revenue=%v payout=%v hasConnected=%v wasBlocked=%v wasConversionAdjusted=%v wasPayoutAdjusted=%v durationSec=%d callLengthSec=%d offlinePresent=%v offline=%v\n",
			call.ExternalCallID, call.CampaignID, call.CampaignName,
			call.HasConverted, call.HasConvertedPresent, call.Revenue, call.Payout,
			call.HasConnected, call.WasBlocked, call.WasConversionAdjusted, call.WasPayoutAdjusted,
			call.DurationSeconds, call.CallLengthSeconds,
			call.OfflineConversionUploadedPresent, call.OfflineConversionUploaded)
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
			{"column": "conversionAmount"},
			{"column": "payoutAmount"},
			{"column": "hasConverted"},
			{"column": "hasConnected"},
			{"column": "wasConversionAdjusted"},
			{"column": "wasPayoutAdjusted"},
			{"column": "connectedCallLengthInSeconds"},
			{"column": "callLengthInSeconds"},
			{"column": "offlineConversionUploaded"},
		},
	}
	client := &http.Client{Timeout: 60 * time.Second}
	found := false
	for page := 0; page < 8; page++ {
		body["offset"] = page * 1000
		raw, st := do(client, "https://api.ringba.com/v2/"+accountID+"/calllogs", token, body)
		fmt.Printf("RAW_PAGE HTTP=%d page=%d token=REDACTED\n", st, page)
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
		for _, rec := range p.Report.Records {
			id, _ := rec["inboundCallId"].(string)
			if id != wantID {
				continue
			}
			found = true
			keys := make([]string, 0, len(rec))
			for k := range rec {
				keys = append(keys, k)
			}
			fmt.Printf("RAW_MATCH id=%s keys=%s campaignName=%q campaignId=%v hasConverted_present=%v hasConverted=%v conversionAmount_present=%v conversionAmount=%v payoutAmount_present=%v payoutAmount=%v wasConversionAdjusted_present=%v wasConversionAdjusted=%v wasPayoutAdjusted_present=%v wasPayoutAdjusted=%v hasConnected_present=%v hasConnected=%v connectedCallLengthInSeconds_present=%v connectedCallLengthInSeconds=%v\n",
				id, strings.Join(keys, ","), rec["campaignName"], rec["campaignId"],
				hasKey(rec, "hasConverted"), rec["hasConverted"],
				hasKey(rec, "conversionAmount"), rec["conversionAmount"],
				hasKey(rec, "payoutAmount"), rec["payoutAmount"],
				hasKey(rec, "wasConversionAdjusted"), rec["wasConversionAdjusted"],
				hasKey(rec, "wasPayoutAdjusted"), rec["wasPayoutAdjusted"],
				hasKey(rec, "hasConnected"), rec["hasConnected"],
				hasKey(rec, "connectedCallLengthInSeconds"), rec["connectedCallLengthInSeconds"],
			)
		}
		if found || len(p.Report.Records) < 1000 {
			break
		}
	}
	if !found {
		fmt.Printf("RAW_NOT_FOUND id=%s in 7d/%d pages\n", wantID, 8)
		os.Exit(1)
	}
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
