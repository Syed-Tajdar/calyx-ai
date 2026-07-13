package main

// probe-retreaver-updated-at — read-only A2 probe. Never prints api_key.
// Usage: go run ./cmd/probe-retreaver-updated-at -integration <uuid> [-call <uuid>]

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
	integrationID := flag.String("integration", "", "integration UUID")
	callID := flag.String("call", "", "optional call UUID; if empty pick first from list")
	flag.Parse()
	if *integrationID == "" {
		fmt.Fprintln(os.Stderr, "need -integration")
		os.Exit(1)
	}

	dbURL := envOr("DATABASE_URL", "postgres://callloom_sync:sync_dev@localhost:5432/callloom?sslmode=disable")
	credKey := envOr("CREDENTIAL_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	must(err)
	defer pool.Close()

	var blob []byte
	must(pool.QueryRow(ctx, `
		SELECT api_credentials_blob FROM get_api_sync_integration($1::uuid, $2::uuid)`,
		"11111111-1111-1111-1111-111111111111", *integrationID).Scan(&blob))
	dec, err := crypto.NewCredentialDecryptor(credKey)
	must(err)
	creds, err := dec.DecryptMap(blob)
	must(err)
	apiKey, companyID := creds["api_key"], creds["company_id"]
	if apiKey == "" || companyID == "" {
		fatal(fmt.Errorf("missing api_key/company_id"))
	}

	client := &http.Client{Timeout: 25 * time.Second}
	id := *callID
	if id == "" {
		calls := list(ctx, client, apiKey, companyID, 5)
		if len(calls) == 0 {
			fatal(fmt.Errorf("no calls"))
		}
		id = calls[0].UUID
		fmt.Printf("picked_uuid=%s\n", id)
	}

	detail := fetchDetail(ctx, client, apiKey, companyID, id)
	fmt.Printf("created_at=%s\n", detail.CreatedAt)
	fmt.Printf("updated_at=%s\n", detail.UpdatedAt)
	fmt.Printf("revenue=%v\n", detail.Revenue)

	upd, err := time.Parse(time.RFC3339Nano, normalizeTS(detail.UpdatedAt))
	if err != nil {
		upd, err = time.Parse(time.RFC3339, normalizeTS(detail.UpdatedAt))
	}
	must(err)

	before := upd.Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	after := upd.Add(2 * time.Minute).UTC().Format(time.RFC3339)
	// Bound the window: open-ended updated_at_start alone can return thousands of
	// newer rows (desc), burying the target past maxPages. Use a tight end bound.
	boundEnd := upd.Add(30 * time.Minute).UTC().Format(time.RFC3339)

	inBefore := listContains(ctx, client, apiKey, companyID, before, boundEnd, id)
	inAfter := listContains(ctx, client, apiKey, companyID, after, boundEnd, id)
	fmt.Printf("probe_updated_at_start_before=%s end_bound=%s appears=%v\n", before, boundEnd, inBefore)
	fmt.Printf("probe_updated_at_start_after=%s end_bound=%s appears=%v\n", after, boundEnd, inAfter)

	// updated_at_end: window that includes call vs end before update
	endOk := listContains(ctx, client, apiKey, companyID, before, after, id)
	endTight := listContains(ctx, client, apiKey, companyID, before, before, id)
	fmt.Printf("probe_updated_at_end_includes appears=%v\n", endOk)
	fmt.Printf("probe_updated_at_end_before_update appears=%v (expect false)\n", endTight)

	page1 := listSorted(ctx, client, apiKey, companyID, 1)
	page2 := listSorted(ctx, client, apiKey, companyID, 1)
	stable := len(page1) > 0 && len(page2) > 0 && page1[0] == page2[0]
	fmt.Printf("sort_by_updated_at_page1_first=%s stable_repeat=%v\n", safeFirst(page1), stable)

	viable := inBefore && !inAfter
	fmt.Printf("updated_at_cursor_VIABLE=%v\n", viable)
	fmt.Printf("docs_note=revenue_bump_of_updated_at UNVERIFIED_in_docs (samples show conversions_determined_at near updated_at)\n")
}

type callRow struct {
	UUID      string
	CreatedAt string
	UpdatedAt string
	Revenue   float64
}

func list(ctx context.Context, client *http.Client, apiKey, companyID string, n int) []callRow {
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	q.Set("per_page", strconv.Itoa(n))
	q.Set("page", "1")
	q.Set("sort_by", "updated_at")
	q.Set("order", "desc")
	body, code := doGET(ctx, client, "https://api.retreaver.com/api/v2/calls.json?"+q.Encode())
	fmt.Printf("list_http=%d\n", code)
	return parseList(body)
}

func listSorted(ctx context.Context, client *http.Client, apiKey, companyID string, page int) []string {
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	q.Set("per_page", "5")
	q.Set("page", strconv.Itoa(page))
	q.Set("sort_by", "updated_at")
	q.Set("order", "desc")
	body, _ := doGET(ctx, client, "https://api.retreaver.com/api/v2/calls.json?"+q.Encode())
	calls := parseList(body)
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.UUID)
	}
	return out
}

func listContains(ctx context.Context, client *http.Client, apiKey, companyID, start, end, want string) bool {
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	q.Set("per_page", "100")
	q.Set("page", "1")
	q.Set("sort_by", "updated_at")
	q.Set("order", "desc")
	q.Set("updated_at_start", start)
	if end != "" {
		q.Set("updated_at_end", end)
	}
	body, code := doGET(ctx, client, "https://api.retreaver.com/api/v2/calls.json?"+q.Encode())
	if code != 200 {
		fmt.Printf("filter_http=%d start=%s end=%s\n", code, start, end)
		return false
	}
	for _, c := range parseList(body) {
		if c.UUID == want {
			return true
		}
	}
	// scan a few more pages
	for page := 2; page <= 5; page++ {
		q.Set("page", strconv.Itoa(page))
		body, code = doGET(ctx, client, "https://api.retreaver.com/api/v2/calls.json?"+q.Encode())
		if code != 200 {
			break
		}
		calls := parseList(body)
		if len(calls) == 0 {
			break
		}
		for _, c := range calls {
			if c.UUID == want {
				return true
			}
		}
	}
	return false
}

func fetchDetail(ctx context.Context, client *http.Client, apiKey, companyID, id string) callRow {
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	u := fmt.Sprintf("https://api.retreaver.com/api/v2/calls/%s.json?%s", url.PathEscape(id), q.Encode())
	body, code := doGET(ctx, client, u)
	fmt.Printf("detail_http=%d\n", code)
	if code != 200 {
		fatal(fmt.Errorf("detail status %d", code))
	}
	return parseOne(body)
}

func doGET(ctx context.Context, client *http.Client, u string) ([]byte, int) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	must(err)
	resp, err := client.Do(req)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return body, resp.StatusCode
}

func parseList(body []byte) []callRow {
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil {
		var wrapped struct {
			Calls []json.RawMessage `json:"calls"`
		}
		if err2 := json.Unmarshal(body, &wrapped); err2 != nil {
			return nil
		}
		arr = wrapped.Calls
	}
	out := make([]callRow, 0, len(arr))
	for _, raw := range arr {
		out = append(out, parseOne(raw))
	}
	return out
}

func parseOne(raw []byte) callRow {
	var wrap struct {
		Call json.RawMessage `json:"call"`
	}
	payload := raw
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Call) > 0 {
		payload = wrap.Call
	}
	var row struct {
		UUID      string          `json:"uuid"`
		CreatedAt string          `json:"created_at"`
		UpdatedAt string          `json:"updated_at"`
		Revenue   json.RawMessage `json:"revenue"`
	}
	_ = json.Unmarshal(payload, &row)
	var rev float64
	_ = json.Unmarshal(row.Revenue, &rev)
	return callRow{UUID: row.UUID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revenue: rev}
}

func normalizeTS(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, " ") && !strings.Contains(s, "T") {
		s = strings.Replace(s, " ", "T", 1)
	}
	return s
}

func safeFirst(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func must(err error) {
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
	os.Exit(1)
}
