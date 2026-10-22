// Phase 2A read-only lookback count and omission check.
// Never prints tokens, account ids, or phone numbers. Never writes to Ringba.
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
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationID = "ce6fd338-af48-47d8-850a-bd10b855e2f4"

func main() {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	if key == "" {
		key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}
	dsn := envOr("DATABASE_URL", "postgres://callloom:callloom_dev@127.0.0.1:5432/callloom?sslmode=disable")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	must(err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `SELECT set_config('app.current_tenant', '11111111-1111-1111-1111-111111111111', false)`)
	must(err)
	var blob []byte
	var lookback int
	var lastSync *time.Time
	must(pool.QueryRow(ctx, `SELECT api_credentials_blob, initial_lookback_days, last_sync_at FROM integrations WHERE id=$1::uuid`, integrationID).Scan(&blob, &lookback, &lastSync))
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
		fatal("missing token or account")
	}
	if lookback < 1 {
		lookback = 30
	}
	end := time.Now().UTC()
	start := end.Add(-time.Duration(lookback) * 24 * time.Hour)
	if v := os.Getenv("WINDOW_START"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		must(err)
		start = t
	}
	if v := os.Getenv("WINDOW_END"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		must(err)
		end = t
	}
	fmt.Printf("WINDOW start=%s end=%s lookback_days=%d TOKEN=REDACTED\n", start.Format(time.RFC3339), end.Format(time.RFC3339), lookback)
	if lastSync != nil {
		fmt.Printf("STORED_LAST_SYNC=%s overlap_start=%s\n", lastSync.UTC().Format(time.RFC3339), lastSync.UTC().Add(-2*time.Hour).Format(time.RFC3339))
	}

	client := &http.Client{Timeout: 120 * time.Second}
	cols := []string{
		"inboundCallId", "hasConnected", "wasBlocked", "blockReason",
		"wasConversionAdjusted", "hasConverted", "conversionAmount",
		"connectedCallLengthInSeconds",
	}
	vc := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		vc = append(vc, map[string]string{"column": c})
	}

	type tally struct {
		rows, distinct                                int
		hcAbsent, hcTrue, hcFalse                     int
		hcAbsentDur0, hcAbsentBlocked, hcAbsentDurPos int
		wbAbsent, wbTrue, wbFalse                     int
		wbAbsentWithReason, wbAbsentConnected         int
		adjTrue, convTrueAmt                          int
	}
	var t tally
	ids := map[string]struct{}{}
	reasons := map[string]int{}
	offset := 0
	const size = 1000
	pages := 0
	started := time.Now()
	for {
		body := map[string]interface{}{
			"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
			"filters": []interface{}{}, "size": size, "offset": offset, "valueColumns": vc,
		}
		rawBody, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.ringba.com/v2/"+accountID+"/calllogs", bytes.NewReader(rawBody))
		must(err)
		req.Header.Set("Authorization", "Token "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		must(err)
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			fatal(fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
		var parsed struct {
			Report struct {
				Records []map[string]interface{} `json:"records"`
			} `json:"report"`
		}
		must(json.Unmarshal(respBody, &parsed))
		recs := parsed.Report.Records
		pages++
		t.rows += len(recs)
		for _, rec := range recs {
			id := str(rec["inboundCallId"])
			if id != "" {
				ids[id] = struct{}{}
			}
			dur := num(rec["connectedCallLengthInSeconds"])
			_, hcOK := rec["hasConnected"]
			_, wbOK := rec["wasBlocked"]
			reason := str(rec["blockReason"])
			if reason != "" {
				reasons[reason]++
			}
			switch {
			case !hcOK:
				t.hcAbsent++
				if dur > 0 {
					t.hcAbsentDurPos++
				} else {
					t.hcAbsentDur0++
				}
				if isYes(rec["wasBlocked"]) {
					t.hcAbsentBlocked++
				}
			case isYes(rec["hasConnected"]):
				t.hcTrue++
			default:
				t.hcFalse++
			}
			switch {
			case !wbOK:
				t.wbAbsent++
				if reason != "" {
					t.wbAbsentWithReason++
				}
				if isYes(rec["hasConnected"]) {
					t.wbAbsentConnected++
				}
			case isYes(rec["wasBlocked"]):
				t.wbTrue++
			default:
				t.wbFalse++
			}
			if isYes(rec["wasConversionAdjusted"]) {
				t.adjTrue++
			}
			if isYes(rec["hasConverted"]) && num(rec["conversionAmount"]) > 0 {
				t.convTrueAmt++
			}
		}
		fmt.Printf("PAGE offset=%d n=%d\n", offset, len(recs))
		if len(recs) < size {
			break
		}
		offset += size
		time.Sleep(800 * time.Millisecond)
	}
	t.distinct = len(ids)
	elapsed := time.Since(started).Round(time.Second)
	fmt.Printf("ROWS=%d DISTINCT=%d PAGES=%d ELAPSED=%s\n", t.rows, t.distinct, pages, elapsed)
	fmt.Printf("HAS_CONNECTED absent=%d true=%d explicit_false=%d absent_dur0=%d absent_blocked=%d absent_dur_positive=%d\n",
		t.hcAbsent, t.hcTrue, t.hcFalse, t.hcAbsentDur0, t.hcAbsentBlocked, t.hcAbsentDurPos)
	fmt.Printf("WAS_BLOCKED absent=%d true=%d explicit_false=%d absent_with_reason=%d absent_and_connected=%d\n",
		t.wbAbsent, t.wbTrue, t.wbFalse, t.wbAbsentWithReason, t.wbAbsentConnected)
	fmt.Printf("ADJUSTED_TRUE=%d CONVERTED_AND_AMOUNT=%d\n", t.adjTrue, t.convTrueAmt)
	keys := make([]string, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("REASON %q %d\n", k, reasons[k])
	}
	if t.hcAbsentDurPos > 0 || t.wbAbsentWithReason > 0 {
		fmt.Println("OMISSION_RULE=FAIL")
		os.Exit(2)
	}
	fmt.Println("OMISSION_RULE=PASS")
	if path := os.Getenv("IDS_OUT"); path != "" {
		f, err := os.Create(path)
		must(err)
		defer f.Close()
		list := make([]string, 0, len(ids))
		for id := range ids {
			list = append(list, id)
		}
		sort.Strings(list)
		for _, id := range list {
			fmt.Fprintln(f, id)
		}
		fmt.Printf("IDS_WRITTEN=%d\n", len(list))
	}
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func num(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func isYes(v interface{}) bool {
	switch n := v.(type) {
	case bool:
		return n
	case string:
		return n == "true" || n == "1" || n == "yes"
	case float64:
		return n != 0
	default:
		return false
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func must(err error) {
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
