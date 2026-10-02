package main

// verify-retreaver-call fetches Retreaver calls using encrypted credentials from
// Postgres. Prints only non-sensitive fields. Never prints api_key, company_id
// (except last 4), caller numbers, or request URLs (the key travels in the query).
//
// Usage:
//   go run ./cmd/verify-retreaver-call -integration <uuid> -call <call-uuid>
//   go run ./cmd/verify-retreaver-call -integration <uuid> -calls uuid,uuid,...
//   go run ./cmd/verify-retreaver-call -integration <uuid> -list 10
//   go run ./cmd/verify-retreaver-call -integration <uuid> -window-start RFC3339 -window-end RFC3339

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	integrationID := flag.String("integration", "", "integration UUID with Retreaver credentials")
	callID := flag.String("call", "", "Retreaver call UUID to fetch")
	callsCSV := flag.String("calls", "", "comma-separated call UUIDs")
	listN := flag.Int("list", 0, "if >0, list N call uuids from calls.json (no secrets)")
	windowStart := flag.String("window-start", "", "RFC3339 updated_at_start for window recon")
	windowEnd := flag.String("window-end", "", "RFC3339 updated_at_end for window recon")
	createdStart := flag.String("created-start", "", "RFC3339 created_at_start (Total probe only)")
	createdEnd := flag.String("created-end", "", "RFC3339 created_at_end (Total probe only)")
	dumpIDs := flag.String("dump-ids", "", "write collected UUIDs, one per line")
	dumpFields := flag.String("dump-fields", "", "write TSV of non-sensitive fields")
	perDay := flag.Bool("per-day", false, "also print Total per UTC day inside the window")
	flag.Parse()

	if *integrationID == "" {
		fmt.Fprintln(os.Stderr, "usage: -integration <id> (-call <uuid> | -calls csv | -list N | -window-start -window-end)")
		os.Exit(1)
	}

	dbURL := envOr("DATABASE_URL", "postgres://callloom_sync:sync_dev@localhost:5432/callloom?sslmode=disable")
	credKey := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	if credKey == "" {
		credKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fatal("postgres", err)
	}
	defer pool.Close()

	// get_api_sync_integration_any does not require is_active — needed while the
	// sync is paused. Never prints the blob.
	var blob []byte
	err = pool.QueryRow(ctx, `
		SELECT api_credentials_blob
		FROM get_api_sync_integration_any($1::uuid, $2::uuid)`,
		"11111111-1111-1111-1111-111111111111", *integrationID).Scan(&blob)
	if err != nil {
		fatal("load integration", err)
	}
	if len(blob) == 0 {
		fatal("credentials", fmt.Errorf("empty credentials blob"))
	}

	dec, err := crypto.NewCredentialDecryptor(credKey)
	if err != nil {
		fatal("decryptor", err)
	}
	creds, err := dec.DecryptMap(blob)
	if err != nil {
		fatal("decrypt", err)
	}
	apiKey := creds["api_key"]
	companyID := creds["company_id"]
	if apiKey == "" || companyID == "" {
		fatal("credentials", fmt.Errorf("api_key/company_id missing after decrypt"))
	}
	if n := len(companyID); n >= 4 {
		fmt.Printf("company_id_last4=%s\n", companyID[n-4:])
	}

	client := &http.Client{Timeout: 60 * time.Second}

	if *listN > 0 {
		listCalls(ctx, client, apiKey, companyID, *listN)
		return
	}
	if *callsCSV != "" {
		ids := splitCSV(*callsCSV)
		fmt.Printf("calls_requested=%d\n", len(ids))
		fmt.Println("n\tuuid\thttp\tstatus\tcreated_at\tupdated_at\tstart_time\ttotal_duration\tdialed_call_duration\trevenue\tpayout")
		for i, id := range ids {
			c := fetchOne(ctx, client, apiKey, companyID, id)
			fmt.Printf("%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				i+1, c.UUID, c.HTTPStatus, c.Status, c.CreatedAt, c.UpdatedAt, c.StartTime,
				fmtInt(c.TotalDuration), fmtInt(c.DialedCallDuration), fmtMoney(c.Revenue), fmtMoney(c.Payout))
			time.Sleep(400 * time.Millisecond)
		}
		return
	}
	if *createdStart != "" && *createdEnd != "" {
		_, h, st := fetchPage(ctx, client, apiKey, companyID, url.Values{
			"created_at_start": {*createdStart},
			"created_at_end":   {*createdEnd},
			"sort_by":          {"created_at"},
			"per_page":         {"1"},
			"page":             {"1"},
		})
		fmt.Printf("created_start=%s\n", *createdStart)
		fmt.Printf("created_end=%s\n", *createdEnd)
		fmt.Printf("http=%d\n", st)
		fmt.Printf("total_header=%d\n", atoi(h.Get("Total")))
		return
	}
	if *windowStart != "" && *windowEnd != "" {
		walkWindow(ctx, client, apiKey, companyID, *windowStart, *windowEnd, *dumpIDs, *dumpFields, *perDay)
		return
	}
	if *callID == "" {
		fmt.Fprintln(os.Stderr, "need -call, -calls, -list or -window-start/-window-end")
		os.Exit(1)
	}
	c := fetchOne(ctx, client, apiKey, companyID, *callID)
	fmt.Printf("uuid=%s\n", c.UUID)
	fmt.Printf("http_status=%d\n", c.HTTPStatus)
	if c.HTTPStatus != 200 {
		os.Exit(1)
	}
	fmt.Printf("state=%s\n", c.State)
	fmt.Printf("status=%s\n", c.Status)
	fmt.Printf("created_at=%s\n", c.CreatedAt)
	fmt.Printf("updated_at=%s\n", c.UpdatedAt)
	fmt.Printf("start_time=%s\n", c.StartTime)
	fmt.Printf("total_duration=%s\n", fmtInt(c.TotalDuration))
	fmt.Printf("dialed_call_duration=%s\n", fmtInt(c.DialedCallDuration))
	fmt.Printf("revenue=%s\n", fmtMoney(c.Revenue))
	fmt.Printf("payout=%s\n", fmtMoney(c.Payout))
	fmt.Printf("converted=%s\n", fmtBool(c.Converted))
	fmt.Printf("caller_present=%t\n", c.Caller != "")
}

func walkWindow(ctx context.Context, client *http.Client, apiKey, companyID, start, end, dumpIDs, dumpFields string, perDay bool) {
	fmt.Printf("window_start=%s\n", start)
	fmt.Printf("window_end=%s\n", end)
	_, hdr0, status0 := fetchPage(ctx, client, apiKey, companyID, url.Values{
		"updated_at_start": {start},
		"updated_at_end":   {end},
		"sort_by":          {"updated_at"},
		"per_page":         {"1"},
		"page":             {"1"},
	})
	total := atoi(hdr0.Get("Total"))
	fmt.Printf("total_probe_http=%d\n", status0)
	fmt.Printf("total_header=%d\n", total)
	time.Sleep(400 * time.Millisecond)

	page1, hdr, status := fetchPage(ctx, client, apiKey, companyID, url.Values{
		"updated_at_start": {start},
		"updated_at_end":   {end},
		"sort_by":          {"updated_at"},
		"per_page":         {"100"},
		"page":             {"1"},
	})
	perPage := atoi(hdr.Get("Per-Page"))
	fmt.Printf("page1_http=%d\n", status)
	fmt.Printf("per_page_header=%d\n", perPage)
	fmt.Printf("page1_rows=%d\n", len(page1))

	all := append([]callFields{}, page1...)
	page := 1
	for len(all) < total {
		page++
		time.Sleep(400 * time.Millisecond)
		rows, _, st := fetchPage(ctx, client, apiKey, companyID, url.Values{
			"updated_at_start": {start},
			"updated_at_end":   {end},
			"sort_by":          {"updated_at"},
			"per_page":         {"100"},
			"page":             {strconv.Itoa(page)},
		})
		if st != 200 {
			fmt.Printf("page_%d_http=%d collected=%d\n", page, st, len(all))
			break
		}
		if len(rows) == 0 {
			break
		}
		all = append(all, rows...)
		if page > 500 {
			fmt.Println("abort_page_cap=500")
			break
		}
	}
	fmt.Printf("pages_fetched=%d\n", page)
	fmt.Printf("collected=%d\n", len(all))
	fmt.Printf("collected_vs_total=%d\n", len(all)-total)

	if dumpIDs != "" {
		f, err := os.Create(dumpIDs)
		if err != nil {
			fatal("dump-ids", err)
		}
		w := bufio.NewWriter(f)
		for _, c := range all {
			if c.UUID != "" {
				fmt.Fprintln(w, c.UUID)
			}
		}
		_ = w.Flush()
		_ = f.Close()
		fmt.Printf("dump_ids=%s\n", dumpIDs)
	}
	if dumpFields != "" {
		f, err := os.Create(dumpFields)
		if err != nil {
			fatal("dump-fields", err)
		}
		w := bufio.NewWriter(f)
		fmt.Fprintln(w, "uuid\tstatus\tcreated_at\tupdated_at\tstart_time\ttotal_duration\tdialed_call_duration\trevenue\tpayout\tconverted\tcampaign_id\ttarget_id\tcaller_state")
		for _, c := range all {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				c.UUID, c.Status, c.CreatedAt, c.UpdatedAt, c.StartTime,
				fmtInt(c.TotalDuration), fmtInt(c.DialedCallDuration),
				fmtMoney(c.Revenue), fmtMoney(c.Payout), fmtBool(c.Converted),
				c.CampaignID, c.TargetID, c.State)
		}
		_ = w.Flush()
		_ = f.Close()
		fmt.Printf("dump_fields=%s\n", dumpFields)
	}

	if !perDay {
		return
	}
	startT, err1 := time.Parse(time.RFC3339, start)
	endT, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil || err2 != nil {
		fatal("window parse", fmt.Errorf("start=%v end=%v", err1, err2))
	}
	fmt.Println("day\tfilter\ttotal_header\thttp")
	day := time.Date(startT.UTC().Year(), startT.UTC().Month(), startT.UTC().Day(), 0, 0, 0, 0, time.UTC)
	last := time.Date(endT.UTC().Year(), endT.UTC().Month(), endT.UTC().Day(), 0, 0, 0, 0, time.UTC)
	for !day.After(last) {
		dStart := day
		dEnd := day.Add(24 * time.Hour)
		if dStart.Before(startT) {
			dStart = startT
		}
		if dEnd.After(endT) {
			dEnd = endT
		}
		if !dEnd.After(dStart) {
			day = day.Add(24 * time.Hour)
			continue
		}
		time.Sleep(400 * time.Millisecond)
		_, h, st := fetchPage(ctx, client, apiKey, companyID, url.Values{
			"updated_at_start": {dStart.Format(time.RFC3339)},
			"updated_at_end":   {dEnd.Format(time.RFC3339)},
			"sort_by":          {"updated_at"},
			"per_page":         {"1"},
			"page":             {"1"},
		})
		fmt.Printf("%s\tupdated_at\t%d\t%d\n", day.Format("2006-01-02"), atoi(h.Get("Total")), st)

		time.Sleep(400 * time.Millisecond)
		_, h2, st2 := fetchPage(ctx, client, apiKey, companyID, url.Values{
			"created_at_start": {day.Format(time.RFC3339)},
			"created_at_end":   {day.Add(24 * time.Hour).Format(time.RFC3339)},
			"updated_at_start": {start},
			"updated_at_end":   {end},
			"sort_by":          {"created_at"},
			"per_page":         {"1"},
			"page":             {"1"},
		})
		fmt.Printf("%s\tcreated_at+window\t%d\t%d\n", day.Format("2006-01-02"), atoi(h2.Get("Total")), st2)
		day = day.Add(24 * time.Hour)
	}
}

func fetchPage(ctx context.Context, client *http.Client, apiKey, companyID string, extra url.Values) ([]callFields, http.Header, int) {
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	for k, vs := range extra {
		for _, v := range vs {
			q.Set(k, v)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.retreaver.com/api/v2/calls.json?"+q.Encode(), nil)
	if err != nil {
		fatal("request", err)
	}
	var resp *http.Response
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err = client.Do(req)
		if err == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "retry page fetch attempt=%d class=%v\n", attempt, redactNet(err))
		time.Sleep(time.Duration(attempt) * time.Second)
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://api.retreaver.com/api/v2/calls.json?"+q.Encode(), nil)
		if err != nil {
			fatal("request", err)
		}
	}
	if err != nil {
		fatal("http", redactNet(err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != 200 {
		return nil, resp.Header, resp.StatusCode
	}
	return parseCallList(body), resp.Header, resp.StatusCode
}

func listCalls(ctx context.Context, client *http.Client, apiKey, companyID string, n int) {
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	q.Set("per_page", strconv.Itoa(n))
	q.Set("page", "1")
	q.Set("sort_by", "created_at")
	q.Set("order", "desc")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.retreaver.com/api/v2/calls.json?"+q.Encode(), nil)
	if err != nil {
		fatal("request", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		fatal("http", redactNet(err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	fmt.Printf("list_http_status=%d\n", resp.StatusCode)
	if resp.StatusCode != 200 {
		fmt.Printf("list_error_body_len=%d\n", len(body))
		os.Exit(1)
	}
	calls := parseCallList(body)
	fmt.Printf("list_count=%d\n", len(calls))
	nonzeroRev := 0
	for i, c := range calls {
		if c.Revenue != nil && *c.Revenue != 0 {
			nonzeroRev++
		}
		revType := "null"
		if c.RevenueRaw != nil {
			revType = fmt.Sprintf("%T", c.RevenueRaw)
		}
		fmt.Printf("call[%d] uuid=%s state=%s status=%s created_at=%s revenue=%s revenue_json_type=%s payout=%s\n",
			i, c.UUID, c.State, c.Status, c.CreatedAt, fmtMoney(c.Revenue), revType, fmtMoney(c.Payout))
	}
	fmt.Printf("nonzero_revenue_in_page=%d/%d\n", nonzeroRev, len(calls))
	ids := make([]string, 0, len(calls))
	for _, c := range calls {
		if c.UUID != "" {
			ids = append(ids, c.UUID)
		}
	}
	fmt.Printf("uuids=%s\n", strings.Join(ids, ","))
}

func fetchOne(ctx context.Context, client *http.Client, apiKey, companyID, callID string) callFields {
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	u := fmt.Sprintf("https://api.retreaver.com/api/v2/calls/%s.json?%s", url.PathEscape(callID), q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		fatal("request", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		fatal("http", redactNet(err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c := callFields{UUID: callID, HTTPStatus: resp.StatusCode}
	if resp.StatusCode != 200 {
		return c
	}
	parsed := parseOneCall(body)
	parsed.HTTPStatus = resp.StatusCode
	if parsed.UUID == "" {
		parsed.UUID = callID
	}
	return parsed
}

type callFields struct {
	UUID               string
	HTTPStatus         int
	State              string
	Status             string
	CreatedAt          string
	UpdatedAt          string
	StartTime          string
	Caller             string
	Revenue            *float64
	Payout             *float64
	RevenueRaw         any
	Converted          *bool
	TotalDuration      *uint32
	DialedCallDuration *uint32
	CampaignID         string
	TargetID           string
}

func parseCallList(body []byte) []callFields {
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil {
		var wrapped struct {
			Calls []json.RawMessage `json:"calls"`
		}
		if err2 := json.Unmarshal(body, &wrapped); err2 != nil {
			fatal("parse list", err)
		}
		arr = wrapped.Calls
	}
	out := make([]callFields, 0, len(arr))
	for _, raw := range arr {
		out = append(out, parseOneCall(raw))
	}
	return out
}

func parseOneCall(raw []byte) callFields {
	var wrap struct {
		Call json.RawMessage `json:"call"`
	}
	payload := raw
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Call) > 0 {
		payload = wrap.Call
	}
	var row struct {
		UUID               string          `json:"uuid"`
		CallerState        string          `json:"caller_state"`
		Status             string          `json:"status"`
		CreatedAt          string          `json:"created_at"`
		UpdatedAt          string          `json:"updated_at"`
		StartTime          string          `json:"start_time"`
		Caller             string          `json:"caller"`
		Revenue            json.RawMessage `json:"revenue"`
		Payout             json.RawMessage `json:"payout"`
		Converted          *bool           `json:"converted"`
		TotalDuration      *uint32         `json:"total_duration"`
		DialedCallDuration *uint32         `json:"dialed_call_duration"`
		CampaignID         int             `json:"campaign_id"`
		SystemCampaignID   int             `json:"system_campaign_id"`
		TargetID           int             `json:"target_id"`
		SystemTargetID     int             `json:"system_target_id"`
	}
	_ = json.Unmarshal(payload, &row)
	rev, revRaw := parseMoney(row.Revenue)
	pay, _ := parseMoney(row.Payout)
	campaign := row.SystemCampaignID
	if campaign == 0 {
		campaign = row.CampaignID
	}
	target := row.SystemTargetID
	if target == 0 {
		target = row.TargetID
	}
	return callFields{
		UUID:               row.UUID,
		State:              row.CallerState,
		Status:             row.Status,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
		StartTime:          row.StartTime,
		Caller:             row.Caller,
		Revenue:            rev,
		Payout:             pay,
		RevenueRaw:         revRaw,
		Converted:          row.Converted,
		TotalDuration:      row.TotalDuration,
		DialedCallDuration: row.DialedCallDuration,
		CampaignID:         strconv.Itoa(campaign),
		TargetID:           strconv.Itoa(target),
	}
}

func parseMoney(raw json.RawMessage) (*float64, any) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return &f, f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, _ := strconv.ParseFloat(s, 64)
		return &v, s
	}
	return nil, string(raw)
}

func fmtMoney(v *float64) string {
	if v == nil {
		return "null"
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func fmtInt(v *uint32) string {
	if v == nil {
		return "null"
	}
	return strconv.FormatUint(uint64(*v), 10)
}

func fmtBool(v *bool) string {
	if v == nil {
		return "null"
	}
	if *v {
		return "true"
	}
	return "false"
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func redactNet(err error) error {
	if err == nil {
		return nil
	}
	// url.Error.URL carries the api_key. Report only the type and a short class.
	return fmt.Errorf("network error (%T)", err)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func fatal(step string, err error) {
	fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", step, err)
	os.Exit(1)
}
