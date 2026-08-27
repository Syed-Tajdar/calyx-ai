package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	key := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	dsn := os.Getenv("DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	must(err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `SELECT set_config('app.current_tenant', $1, false)`, "11111111-1111-1111-1111-111111111111")
	must(err)
	var blob []byte
	must(pool.QueryRow(ctx, `SELECT api_credentials_blob FROM integrations WHERE id='ae17d28e-3f98-4288-abf9-ef1bfa978776'::uuid`).Scan(&blob))
	dec, err := crypto.NewCredentialDecryptor(key)
	must(err)
	creds, err := dec.DecryptMap(blob)
	must(err)
	token := creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	base := "https://api.ringba.com/v2/" + creds["account_id"]
	client := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	fake := "RGB0000000000000000000000000000000000000000FAKEPROBE01"

	for _, path := range []string{"/calllogs/detail", "/calllogs/details", "/CallLogs/detail"} {
		body, _ := json.Marshal(map[string]interface{}{"InboundCallIds": []string{fake}})
		req, _ := http.NewRequest("POST", base+path, bytes.NewReader(body))
		req.Header.Set("Authorization", "Token "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(path, err)
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		snip := string(raw)
		if len(snip) > 300 {
			snip = snip[:300] + "…"
		}
		fmt.Printf("DETAIL %s status=%d body=%s\n", path, resp.StatusCode, snip)
	}

	end := time.Now().UTC()
	start := end.Add(-14 * 24 * time.Hour)
	body, _ := json.Marshal(map[string]interface{}{
		"reportStart": start.Format(time.RFC3339), "reportEnd": end.Format(time.RFC3339),
		"filters": []interface{}{}, "size": 200, "offset": 0,
		"valueColumns": []map[string]string{
			{"column": "inboundCallId"}, {"column": "conversionAmount"}, {"column": "hasConverted"},
			{"column": "hasPayout"}, {"column": "noPayoutReason"}, {"column": "hasAnnotations"},
			{"column": "wasConversionAdjusted"}, {"column": "wasPayoutAdjusted"},
			{"column": "noConversionReason"},
		},
	})
	req, _ := http.NewRequest("POST", base+"/calllogs", bytes.NewReader(body))
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	must(err)
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	resp.Body.Close()
	fmt.Printf("CALLLOGS status=%d bytes=%d\n", resp.StatusCode, len(raw))
	var p struct {
		Report struct {
			Records []map[string]interface{} `json:"records"`
		} `json:"report"`
	}
	_ = json.Unmarshal(raw, &p)

	type st struct{ n, hasKey, trueN, falseN int }
	stats := map[string]*st{}
	for _, f := range []string{"hasConverted", "hasPayout", "noPayoutReason", "hasAnnotations", "wasConversionAdjusted", "wasPayoutAdjusted", "noConversionReason"} {
		stats[f] = &st{}
	}
	revPos, revZero := 0, 0
	convTrueRev, convAbsentRev, convFalseRev := 0, 0, 0
	convTrueZero, convAbsentZero, convFalseZero := 0, 0, 0
	for _, r := range p.Report.Records {
		amt := asFloat(r["conversionAmount"])
		if amt > 0 {
			revPos++
		} else {
			revZero++
		}
		_, present := r["hasConverted"]
		val := fmt.Sprint(r["hasConverted"])
		if amt > 0 {
			if !present {
				convAbsentRev++
			} else if val == "true" {
				convTrueRev++
			} else {
				convFalseRev++
			}
		} else {
			if !present {
				convAbsentZero++
			} else if val == "true" {
				convTrueZero++
			} else {
				convFalseZero++
			}
		}
		for f, s := range stats {
			s.n++
			if _, ok := r[f]; ok {
				s.hasKey++
				v := fmt.Sprint(r[f])
				if v == "true" || v == "1" || v == "Yes" {
					s.trueN++
				} else {
					s.falseN++
				}
			}
		}
	}
	fmt.Printf("XTAB records=%d revPos=%d revZero=%d\n", len(p.Report.Records), revPos, revZero)
	fmt.Printf("hasConverted|rev>0: true=%d false=%d absent=%d\n", convTrueRev, convFalseRev, convAbsentRev)
	fmt.Printf("hasConverted|rev=0: true=%d false=%d absent=%d\n", convTrueZero, convFalseZero, convAbsentZero)
	for f, s := range stats {
		fmt.Printf("FIELD %s present=%d/%d true=%d other=%d\n", f, s.hasKey, s.n, s.trueN, s.falseN)
	}
}

func asFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return 0
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
