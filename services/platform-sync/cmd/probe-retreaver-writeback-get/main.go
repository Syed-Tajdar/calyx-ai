package main

// probe-retreaver-writeback-get — Stage 4b Phase 2, GET only. Not built into any image.
//
// Reads one Retreaver call plus its campaign, target and affiliate, so the owner can see
// whether pixels/postbacks or non-$0 terms are configured before any write is approved.
//
// Credentials come only from the stored integration, decrypted in-process. The encryption
// key and DATABASE_URL are read from stdin (two lines, in that order) so they never appear
// on a command line. The api_key and company_id are never printed: every request URL and
// error goes through internal/redact, and the printed objects are masked (phones, SIP
// credentials, object/show keys, postback key uuids, URL paths and queries).
//
// The HTTP transport refuses any method other than GET and any host other than
// api.retreaver.com, so this binary cannot write.
//
// Usage (stdin: CREDENTIAL_ENCRYPTION_KEY, then DATABASE_URL):
//   go run ./cmd/probe-retreaver-writeback-get -integration <uuid> -call <uuid>

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	tenantID   = "11111111-1111-1111-1111-111111111111"
	apiOrigin  = "https://api.retreaver.com"
	apiHost    = "api.retreaver.com"
	reqSpacing = 700 * time.Millisecond // stays under the shared 2/s limit
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// phoneRE finds phone numbers inside free text such as target names.
var phoneRE = regexp.MustCompile(`\+\d{7,15}`)

// getOnly refuses every request that is not a GET to api.retreaver.com.
type getOnly struct{ next http.RoundTripper }

func (g getOnly) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet {
		return nil, fmt.Errorf("probe refuses method %s", r.Method)
	}
	if r.URL.Scheme != "https" || r.URL.Host != apiHost {
		return nil, fmt.Errorf("probe refuses host %s", r.URL.Host)
	}
	return g.next.RoundTrip(r)
}

type client struct {
	http      *http.Client
	apiKey    string
	companyID string
	last      time.Time
	requests  int
}

func (c *client) get(ctx context.Context, path string) (map[string]any, int, error) {
	if wait := reqSpacing - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
	c.requests++

	q := url.Values{}
	q.Set("api_key", c.apiKey)
	q.Set("company_id", c.companyID)
	u := apiOrigin + path + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, errors.New(redact.SanitizeErr(err))
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, errors.New(redact.SanitizeErr(err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	fmt.Printf("GET %s -> %d\n", redact.RedactURL(u), resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("http %d: %s", resp.StatusCode, redact.RedactMessage(truncate(string(body), 300)))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		var arr []any
		if err2 := json.Unmarshal(body, &arr); err2 == nil {
			return map[string]any{"_array": arr}, resp.StatusCode, nil
		}
		return nil, resp.StatusCode, fmt.Errorf("parse: %s", redact.SanitizeErr(err))
	}
	return out, resp.StatusCode, nil
}

func main() {
	integrationID := flag.String("integration", "", "stored Retreaver integration UUID")
	callID := flag.String("call", "", "sandbox call UUID")
	flag.Parse()
	if !uuidRE.MatchString(*integrationID) || !uuidRE.MatchString(*callID) {
		fatal(errors.New("need -integration <uuid> and -call <uuid>"))
	}

	in := bufio.NewScanner(os.Stdin)
	credKey, dbURL := readLine(in), readLine(in)
	if credKey == "" || dbURL == "" {
		fatal(errors.New("stdin must carry CREDENTIAL_ENCRYPTION_KEY then DATABASE_URL"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	defer pool.Close()

	// Same lookup as the write-back path: it also returns paused integrations.
	var (
		platform, name, syncMode string
		writeback                bool
		blob                     []byte
	)
	err = pool.QueryRow(ctx, `
		SELECT platform, name, sync_mode, writeback_enabled, api_credentials_blob
		  FROM get_api_sync_integration_any($1::uuid, $2::uuid)`,
		tenantID, *integrationID).Scan(&platform, &name, &syncMode, &writeback, &blob)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	fmt.Printf("integration platform=%s name=%q sync_mode=%s writeback_enabled=%v\n",
		platform, name, syncMode, writeback)
	if platform != "retreaver" || syncMode == "live" {
		fatal(errors.New("integration must be a Retreaver Test (mock) integration; refusing"))
	}

	dec, err := crypto.NewCredentialDecryptor(credKey)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	creds, err := dec.DecryptMap(blob)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	c := &client{
		http:      &http.Client{Timeout: 25 * time.Second, Transport: getOnly{http.DefaultTransport}},
		apiKey:    creds["api_key"],
		companyID: creds["company_id"],
	}
	if c.apiKey == "" || c.companyID == "" {
		fatal(errors.New("stored credentials lack api_key or company_id"))
	}
	fmt.Println("credentials: api_key present, company_id present (values not printed)")

	// 1. The call (read-back endpoint).
	callDoc, _, err := c.get(ctx, "/api/v2/calls/"+*callID+".json")
	if err != nil {
		fatal(err)
	}
	call := unwrap(callDoc, "call")
	// Same trimmed compare as the writer's LookupCall; neither value is printed.
	stored := strings.TrimSpace(c.companyID)
	fmt.Printf("call.company_id == stored company_id (trimmed, as the writer compares): %v; stored value has surrounding whitespace: %v\n",
		str(call["company_id"]) == stored, stored != c.companyID)
	section("CALL read-back fields")
	for _, k := range []string{"uuid", "status", "converted", "revenue", "payout", "receivable", "payable",
		"fired_pixels_count", "conversions_determined_at", "postback_value", "updated_at", "created_at",
		"campaign_id", "campaign_name", "cid", "target_id", "target_name", "tid", "system_target_id",
		"afid", "system_affiliate_id", "affiliate_name", "sid", "duplicate", "payable_duplicate",
		"receivable_duplicate", "network_sale_timer_fired", "affiliate_sale_timer_fired",
		"target_sale_timer_fired", "conversion_seconds", "connected", "total_duration", "via"} {
		fmt.Printf("  %s = %s\n", k, show(call[k]))
	}
	section("CALL fired_pixels")
	printJSON(mask(call["fired_pixels"], ""))
	section("CALL full (masked)")
	printJSON(mask(call, ""))

	// 2. Campaign.
	section("CAMPAIGN")
	if cid := str(call["cid"]); cid != "" {
		doc, _, err := c.get(ctx, "/campaigns/cid/"+url.PathEscape(cid)+".json")
		report(doc, "campaign", err)
	} else if id := str(call["campaign_id"]); id != "" {
		fmt.Println("  call has no cid; listing campaigns and matching campaign_id")
		doc, _, err := c.get(ctx, "/campaigns.json")
		if err != nil {
			fmt.Printf("  ERROR %v\n", err)
		} else {
			found := false
			for _, item := range asList(doc) {
				camp := unwrap(asMap(item), "campaign")
				if str(camp["id"]) == id {
					printJSON(mask(camp, ""))
					found = true
				}
			}
			if !found {
				fmt.Printf("  campaign id %s not found in list (%d campaigns)\n", id, len(asList(doc)))
			}
		}
	} else {
		fmt.Println("  call has no campaign")
	}

	// 3. Target.
	section("TARGET")
	if id := str(call["target_id"]); id != "" {
		doc, _, err := c.get(ctx, "/targets/"+url.PathEscape(id)+".json")
		report(doc, "target", err)
	} else {
		fmt.Println("  call has no target_id")
	}

	// 4. Affiliate (publisher).
	section("AFFILIATE")
	// The legacy afid lookup first, then the id endpoint from learn.retreaver.com/api/affiliates.
	done := false
	if afid := str(call["afid"]); afid != "" {
		doc, _, err := c.get(ctx, "/api/v1/affiliates/afid/"+url.PathEscape(afid)+".json")
		report(doc, "affiliate", err)
		done = err == nil
	}
	if id := str(call["system_affiliate_id"]); !done && id != "" {
		doc, _, err := c.get(ctx, "/api/v1/affiliates/"+url.PathEscape(id)+".json")
		report(doc, "affiliate", err)
		done = err == nil
	}
	if !done {
		fmt.Println("  affiliate not readable")
	}

	fmt.Printf("\nrequests sent: %d (all GET)\n", c.requests)
}

func report(doc map[string]any, key string, err error) {
	if err != nil {
		fmt.Printf("  ERROR %v\n", err)
		return
	}
	printJSON(mask(unwrap(doc, key), ""))
}

// mask hides values that could identify people or carry secrets. Structure, numbers,
// booleans and names stay visible so the terms and pixel settings can be checked.
func mask(v any, key string) any {
	k := strings.ToLower(key)
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for kk, vv := range t {
			out[kk] = mask(vv, kk)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, vv := range t {
			out[i] = mask(vv, key)
		}
		return out
	case string:
		if t == "" {
			return t
		}
		switch {
		case k == "uuid" || k == "batch_uuid":
			return t
		case k == "company_id":
			return "***"
		case strings.Contains(k, "password") || strings.Contains(k, "secret") || strings.Contains(k, "token") ||
			strings.HasSuffix(k, "key") || strings.Contains(k, "uuid") || k == "sip_username":
			return "***set***"
		case strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://"):
			if u, err := url.Parse(t); err == nil {
				return "<url host=" + u.Host + ">"
			}
			return "<url>"
		case strings.HasPrefix(t, "+") || k == "caller" || strings.Contains(k, "number") || k == "caller_zip" || k == "caller_city":
			return "***"
		}
		return phoneRE.ReplaceAllString(redact.RedactMessage(t), "***")
	}
	if k == "company_id" && v != nil {
		return "***"
	}
	return v
}

func unwrap(doc map[string]any, key string) map[string]any {
	if inner, ok := doc[key].(map[string]any); ok {
		return inner
	}
	return doc
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asList(doc map[string]any) []any {
	if arr, ok := doc["_array"].([]any); ok {
		return arr
	}
	for _, v := range doc {
		if arr, ok := v.([]any); ok {
			return arr
		}
	}
	return nil
}

func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	}
	return fmt.Sprint(v)
}

func show(v any) string {
	if v == nil {
		return "null"
	}
	b, _ := json.Marshal(mask(v, ""))
	return string(b)
}

func printJSON(v any) {
	b, err := json.MarshalIndent(v, "  ", "  ")
	if err != nil {
		fmt.Printf("  <unprintable: %v>\n", err)
		return
	}
	fmt.Println("  " + redact.RedactMessage(string(b)))
}

func section(title string) { fmt.Printf("\n=== %s ===\n", title) }

func readLine(s *bufio.Scanner) string {
	if s.Scan() {
		return strings.TrimSpace(s.Text())
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "FAIL: %s\n", redact.RedactMessage(err.Error()))
	os.Exit(1)
}
