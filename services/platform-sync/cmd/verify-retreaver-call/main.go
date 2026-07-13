package main

// verify-retreaver-call fetches a call by UUID from Retreaver using encrypted
// credentials from Postgres. Prints only non-sensitive fields. Never prints api_key.
//
// Usage:
//   go run ./cmd/verify-retreaver-call -integration <uuid> -call <call-uuid>
//   go run ./cmd/verify-retreaver-call -integration <uuid> -list 10

import (
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
	listN := flag.Int("list", 0, "if >0, list N call uuids from calls.json (no secrets)")
	flag.Parse()

	if *integrationID == "" {
		fmt.Fprintln(os.Stderr, "usage: -integration <id> (-call <uuid> | -list N)")
		os.Exit(1)
	}

	dbURL := envOr("DATABASE_URL", "postgres://callloom_sync:sync_dev@localhost:5432/callloom?sslmode=disable")
	credKey := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	if credKey == "" {
		credKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fatal("postgres", err)
	}
	defer pool.Close()

	var blob []byte
	err = pool.QueryRow(ctx, `
		SELECT api_credentials_blob
		FROM get_api_sync_integration($1::uuid, $2::uuid)`,
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

	client := &http.Client{Timeout: 25 * time.Second}

	if *listN > 0 {
		listCalls(ctx, client, apiKey, companyID, *listN)
		return
	}
	if *callID == "" {
		fmt.Fprintln(os.Stderr, "need -call or -list")
		os.Exit(1)
	}
	fetchOne(ctx, client, apiKey, companyID, *callID)
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
		fatal("http", err)
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
		if c.Revenue != 0 {
			nonzeroRev++
		}
		revType := "null"
		if c.RevenueRaw != nil {
			revType = fmt.Sprintf("%T", c.RevenueRaw)
		}
		fmt.Printf("call[%d] uuid=%s state=%s status=%s created_at=%s revenue=%v revenue_json_type=%s payout=%v\n",
			i, c.UUID, c.State, c.Status, c.CreatedAt, c.Revenue, revType, c.Payout)
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

func fetchOne(ctx context.Context, client *http.Client, apiKey, companyID, callID string) {
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
		fatal("http", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	fmt.Printf("uuid=%s\n", callID)
	fmt.Printf("http_status=%d\n", resp.StatusCode)
	if resp.StatusCode != 200 {
		fmt.Printf("body_len=%d\n", len(body))
		os.Exit(1)
	}
	c := parseOneCall(body)
	fmt.Printf("state=%s\n", c.State)
	fmt.Printf("status=%s\n", c.Status)
	fmt.Printf("created_at=%s\n", c.CreatedAt)
	fmt.Printf("updated_at=%s\n", c.UpdatedAt)
	fmt.Printf("revenue=%v\n", c.Revenue)
	fmt.Printf("payout=%v\n", c.Payout)
	fmt.Printf("caller_present=%t\n", c.Caller != "")
}

type callFields struct {
	UUID       string
	State      string
	Status     string
	CreatedAt  string
	UpdatedAt  string
	Caller     string
	Revenue    float64
	Payout     float64
	RevenueRaw any
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
		UUID        string          `json:"uuid"`
		CallerState string          `json:"caller_state"`
		Status      string          `json:"status"`
		CreatedAt   string          `json:"created_at"`
		UpdatedAt   string          `json:"updated_at"`
		Caller      string          `json:"caller"`
		Revenue     json.RawMessage `json:"revenue"`
		Payout      json.RawMessage `json:"payout"`
	}
	_ = json.Unmarshal(payload, &row)
	rev, revRaw := parseMoney(row.Revenue)
	pay, _ := parseMoney(row.Payout)
	return callFields{
		UUID:       row.UUID,
		State:      row.CallerState,
		Status:     row.Status,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		Caller:     row.Caller,
		Revenue:    rev,
		Payout:     pay,
		RevenueRaw: revRaw,
	}
}

func parseMoney(raw json.RawMessage) (float64, any) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, _ := strconv.ParseFloat(s, 64)
		return v, s
	}
	return 0, string(raw)
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
