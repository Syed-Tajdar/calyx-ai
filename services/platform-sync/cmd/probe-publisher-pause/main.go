package main

// probe-publisher-pause — v2 Phase 2D sandbox proof, step 1: GET ONLY. Not built into any image
// (the Dockerfile builds only ./cmd/server).
//
// It finds the sandbox publisher (Ringba) or affiliate (Retreaver) by exact name — default
// "CALYX-TEST-PUB-DO-NOT-USE" — prints its id and paused/enabled state when visible, and tries to
// prove it has no numbers and no campaigns. Anything it cannot prove is printed as UNKNOWN.
// Finally it prints the planned write steps (read → pause → read → unpause → read) as a table.
// Those steps are NEVER executed: there is no flag to execute them in this phase, and the HTTP
// transport refuses every method except GET and every host except the two platform APIs.
//
// Credentials come only from the stored integration, decrypted in-process (same pattern as
// cmd/probe-retreaver-writeback-get). Stdin carries two lines: CREDENTIAL_ENCRYPTION_KEY, then
// DATABASE_URL, so neither appears on a command line. It reads a Test or a Live integration (GET
// only either way). No key, token, account id or URL is printed: requests are shown as host +
// path with the account id replaced by {accountId} and without the query string.
//
// Usage:
//   go run ./cmd/probe-publisher-pause -integration <uuid> [-tenant <uuid>] [-name CALYX-TEST-PUB-DO-NOT-USE]

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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultTenant = "11111111-1111-1111-1111-111111111111"
	defaultName   = "CALYX-TEST-PUB-DO-NOT-USE"
	ringbaHost    = "api.ringba.com"
	retreaverHost = "api.retreaver.com"
	maxPages      = 50
	perPage       = 100
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var safeIDRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// getOnly refuses every request that is not an https GET to one of the two platform APIs.
type getOnly struct{ next http.RoundTripper }

func (g getOnly) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet {
		return nil, fmt.Errorf("probe refuses method %s (GET only in this phase)", r.Method)
	}
	if r.URL.Scheme != "https" || (r.URL.Host != ringbaHost && r.URL.Host != retreaverHost) {
		return nil, fmt.Errorf("probe refuses host %s", r.URL.Host)
	}
	return g.next.RoundTrip(r)
}

type client struct {
	http     *http.Client
	spacing  time.Duration
	last     time.Time
	requests int
	// secrets are removed from anything printed, on top of internal/redact.
	secrets []string
}

func (c *client) scrub(s string) string {
	s = redact.RedactMessage(s)
	for _, v := range c.secrets {
		if len(v) >= 3 {
			s = strings.ReplaceAll(s, v, "{redacted}")
		}
	}
	return s
}

// get sends one GET. shown is what is printed for the request (host + templated path).
func (c *client) get(ctx context.Context, rawURL string, hdr map[string]string, shown string) (any, int, error) {
	if wait := c.spacing - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
	c.requests++
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, errors.New(c.scrub(err.Error()))
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, errors.New(c.scrub(err.Error()))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	fmt.Printf("  GET %s -> %d\n", shown, resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("http %d: %s", resp.StatusCode, c.scrub(truncate(string(body), 200)))
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("parse: %s", c.scrub(err.Error()))
	}
	return out, resp.StatusCode, nil
}

func main() {
	integrationID := flag.String("integration", "", "stored integration UUID (Ringba or Retreaver; Test or Live)")
	tenant := flag.String("tenant", defaultTenant, "tenant UUID")
	name := flag.String("name", defaultName, "exact publisher / affiliate name")
	flag.Parse()
	if !uuidRE.MatchString(*integrationID) || !uuidRE.MatchString(*tenant) {
		fatal(errors.New("need -integration <uuid> (and -tenant <uuid> if not the default)"))
	}
	if strings.TrimSpace(*name) == "" {
		fatal(errors.New("-name must not be empty"))
	}

	in := bufio.NewScanner(os.Stdin)
	credKey, dbURL := readLine(in), readLine(in)
	if credKey == "" || dbURL == "" {
		fatal(errors.New("stdin must carry CREDENTIAL_ENCRYPTION_KEY then DATABASE_URL"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	defer pool.Close()

	var (
		platform, intName, syncMode string
		isActive, allowPause        bool
		blob                        []byte
	)
	err = pool.QueryRow(ctx, `
		SELECT platform, COALESCE(name, ''), COALESCE(sync_mode, ''), is_active, allow_publisher_pause, api_credentials_blob
		  FROM get_publisher_pause_integration($1::uuid, $2::uuid)`,
		*tenant, *integrationID).Scan(&platform, &intName, &syncMode, &isActive, &allowPause, &blob)
	if err != nil {
		fatal(errors.New("integration not found via get_publisher_pause_integration: " + redact.SanitizeErr(err)))
	}
	mode := "Test (mock)"
	if syncMode == "real" {
		mode = "Live (real)"
	}
	fmt.Printf("integration platform=%s name=%q mode=%s is_active=%v allow_publisher_pause=%v\n",
		platform, intName, mode, isActive, allowPause)

	dec, err := crypto.NewCredentialDecryptor(credKey)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	creds, err := dec.DecryptMap(blob)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	c := &client{http: &http.Client{
		Timeout:   25 * time.Second,
		Transport: getOnly{http.DefaultTransport},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
	for _, v := range creds {
		c.secrets = append(c.secrets, strings.TrimSpace(v))
	}

	var r report
	switch platform {
	case "ringba":
		c.spacing = 300 * time.Millisecond
		r = probeRingba(ctx, c, creds, *name)
	case "retreaver":
		c.spacing = 700 * time.Millisecond // under the shared 2/s limit
		r = probeRetreaver(ctx, c, creds, *name)
	default:
		fatal(fmt.Errorf("platform %q has no publisher pause", platform))
	}
	r.print()
	printPlan(platform, r.id, r.pathID)
	fmt.Printf("\nrequests sent: %d (all GET). No write was sent; none can be sent by this binary.\n", c.requests)
}

// report is the probe's result. Every check is PROVEN, INFERRED or UNKNOWN.
type report struct {
	platform string
	name     string
	id       string // Ringba publisher id / Retreaver afid (what Calyx's API sends)
	pathID   string // id used in the write path (Ringba id / Retreaver system id)
	state    string
	checks   [][2]string
}

func (r *report) add(label, verdict string) { r.checks = append(r.checks, [2]string{label, verdict}) }

func (r report) print() {
	fmt.Printf("\n=== RESULT (%s) ===\n", r.platform)
	fmt.Printf("  name           %q\n", r.name)
	if r.id == "" {
		fmt.Println("  id             NOT FOUND")
	} else {
		fmt.Printf("  id             %s\n", r.id)
		if r.pathID != "" && r.pathID != r.id {
			fmt.Printf("  system id      %s\n", r.pathID)
		}
	}
	fmt.Printf("  state          %s\n", r.state)
	for _, ch := range r.checks {
		fmt.Printf("  %-34s %s\n", ch[0], ch[1])
	}
}

// ---------------------------------------------------------------------------------------------
// Ringba

func probeRingba(ctx context.Context, c *client, creds map[string]string, name string) report {
	r := report{platform: "ringba", name: name, state: "UNKNOWN"}
	token := strings.TrimSpace(creds["api_token"])
	if token == "" {
		token = strings.TrimSpace(creds["api_key"])
	}
	account := strings.TrimSpace(creds["account_id"])
	if token == "" || !safeIDRE.MatchString(account) {
		fatal(errors.New("stored Ringba credentials lack api_token or a valid account_id"))
	}
	hdr := map[string]string{"Authorization": "Token " + token}
	base := "https://" + ringbaHost + "/v2/" + url.PathEscape(account)
	show := func(p string) string { return ringbaHost + " /v2/{accountId}" + p }

	fmt.Println("\n=== Ringba: list publishers ===")
	doc, _, err := c.get(ctx, base+"/Publishers", hdr, show("/Publishers"))
	if err != nil {
		fmt.Printf("  ERROR %v\n", err)
		r.add("found by name", "UNKNOWN (publisher list not readable)")
		return r
	}
	var matches []map[string]any
	list := asList(doc)
	for _, it := range list {
		m := unwrap(asMap(it), "publisher")
		if str(m["name"]) == name {
			matches = append(matches, m)
		}
	}
	fmt.Printf("  publishers listed: %d, exact name matches: %d\n", len(list), len(matches))
	if len(matches) != 1 {
		if len(matches) > 1 {
			for _, m := range matches {
				fmt.Printf("  match id=%s\n", str(m["id"]))
			}
			r.add("found by name", "AMBIGUOUS (more than one publisher has this name) — stop")
		} else {
			r.add("found by name", "NOT FOUND")
		}
		return r
	}
	id := str(matches[0]["id"])
	if !safeIDRE.MatchString(id) {
		r.add("found by name", "FOUND but the id is not a safe token — stop")
		return r
	}
	r.id, r.pathID = id, id
	r.add("found by name", "PROVEN (exactly one)")

	fmt.Println("\n=== Ringba: publisher detail ===")
	det, _, err := c.get(ctx, base+"/Publishers/"+url.PathEscape(id), hdr, show("/Publishers/"+id))
	var detail map[string]any
	if err != nil {
		fmt.Printf("  ERROR %v\n", err)
		r.add("detail GET", "UNKNOWN ("+err.Error()+")")
	} else {
		detail = unwrap(asMap(det), "publisher")
		fmt.Printf("  keys: %s\n", strings.Join(keys(detail), ", "))
		if en, ok := detail["enabled"].(bool); ok {
			r.state = fmt.Sprintf("enabled=%v (paused=%v)", en, !en)
			r.add("\"enabled\" in GET (read-back)", "PROVEN present")
		} else {
			r.add("\"enabled\" in GET (read-back)", "UNKNOWN (key absent)")
		}
		for _, k := range keys(detail) {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "number") || strings.Contains(lk, "campaign") {
				fmt.Printf("  detail.%s = %s\n", k, summarize(detail[k]))
			}
		}
	}

	fmt.Println("\n=== Ringba: numbers ===")
	ndoc, _, err := c.get(ctx, base+"/numbers", hdr, show("/numbers"))
	if err != nil {
		fmt.Printf("  ERROR %v\n", err)
		r.add("numbers assigned", "UNKNOWN (number list not readable)")
	} else {
		nums := asList(ndoc)
		withRef, hits := 0, 0
		for _, it := range nums {
			m := asMap(it)
			if hasPublisherRef(m) {
				withRef++
			}
			if containsValue(m, id) {
				hits++
			}
		}
		fmt.Printf("  numbers listed: %d, carrying a publisher/affiliate field: %d, referencing this id: %d\n", len(nums), withRef, hits)
		switch {
		case hits > 0:
			r.add("numbers assigned", fmt.Sprintf("NOT EMPTY (%d numbers reference this publisher) — stop", hits))
		case len(nums) == 0:
			r.add("numbers assigned", "PROVEN none (account lists no numbers)")
		case withRef == 0:
			r.add("numbers assigned", "UNKNOWN (number objects carry no publisher field to check)")
		default:
			r.add("numbers assigned", "PROVEN none (no number references this id)")
		}
	}

	fmt.Println("\n=== Ringba: campaigns ===")
	cdoc, _, err := c.get(ctx, base+"/campaigns", hdr, show("/campaigns"))
	if err != nil {
		fmt.Printf("  ERROR %v\n", err)
		r.add("campaign membership", "UNKNOWN (campaign list not readable)")
	} else {
		camps := asList(cdoc)
		withRef, hits := 0, 0
		for _, it := range camps {
			m := asMap(it)
			if hasPublisherRef(m) {
				withRef++
			}
			if containsValue(m, id) {
				hits++
			}
		}
		fmt.Printf("  campaigns listed: %d, carrying a publisher/affiliate field: %d, referencing this id: %d\n", len(camps), withRef, hits)
		switch {
		case hits > 0:
			r.add("campaign membership", fmt.Sprintf("NOT EMPTY (%d campaigns reference this publisher) — stop", hits))
		case len(camps) == 0:
			r.add("campaign membership", "PROVEN none (account lists no campaigns)")
		case withRef == 0:
			r.add("campaign membership", "UNKNOWN (the campaign list does not show publishers; check in the Ringba UI)")
		default:
			r.add("campaign membership", "PROVEN none (no listed campaign references this id)")
		}
	}
	return r
}

// ---------------------------------------------------------------------------------------------
// Retreaver

func probeRetreaver(ctx context.Context, c *client, creds map[string]string, name string) report {
	r := report{platform: "retreaver", name: name, state: "UNKNOWN"}
	apiKey := strings.TrimSpace(creds["api_key"])
	company := strings.TrimSpace(creds["company_id"])
	if apiKey == "" || company == "" {
		fatal(errors.New("stored Retreaver credentials lack api_key or company_id"))
	}
	mk := func(path string, extra url.Values) string {
		q := url.Values{}
		for k, v := range extra {
			q[k] = v
		}
		q.Set("api_key", apiKey)
		q.Set("company_id", company)
		return "https://" + retreaverHost + path + "?" + q.Encode()
	}

	fmt.Println("\n=== Retreaver: list affiliates ===")
	var matches []map[string]any
	total := 0
	listed := true
	for page := 1; page <= maxPages; page++ {
		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(perPage)}}
		doc, _, err := c.get(ctx, mk("/api/v1/affiliates.json", q), nil,
			fmt.Sprintf("%s /api/v1/affiliates.json page=%d", retreaverHost, page))
		if err != nil {
			fmt.Printf("  ERROR %v\n", err)
			listed = false
			break
		}
		items := asList(doc)
		total += len(items)
		for _, it := range items {
			m := unwrap(asMap(it), "affiliate")
			full := strings.TrimSpace(str(m["first_name"]) + " " + str(m["last_name"]))
			if str(m["company_name"]) == name || full == name || str(m["first_name"]) == name || str(m["name"]) == name {
				matches = append(matches, m)
			}
		}
		if len(items) < perPage {
			break
		}
	}
	fmt.Printf("  affiliates listed: %d, exact name matches: %d\n", total, len(matches))
	if !listed && len(matches) == 0 {
		r.add("found by name", "UNKNOWN (affiliate list not readable)")
		return r
	}
	if len(matches) != 1 {
		if len(matches) > 1 {
			for _, m := range matches {
				fmt.Printf("  match id=%s afid=%s\n", str(m["id"]), str(m["afid"]))
			}
			r.add("found by name", "AMBIGUOUS (more than one affiliate has this name) — stop")
		} else {
			r.add("found by name", "NOT FOUND")
		}
		return r
	}
	sysID, afid := str(matches[0]["id"]), str(matches[0]["afid"])
	if !safeIDRE.MatchString(sysID) {
		r.add("found by name", "FOUND but the system id is not a safe token — stop")
		return r
	}
	r.id, r.pathID = afid, sysID
	if afid == "" {
		r.id = "(no afid)"
	}
	r.add("found by name", "PROVEN (exactly one)")

	fmt.Println("\n=== Retreaver: affiliate detail ===")
	if afid != "" && safeIDRE.MatchString(afid) {
		_, status, err := c.get(ctx, mk("/api/v1/affiliates/afid/"+url.PathEscape(afid)+".json", nil), nil,
			retreaverHost+" /api/v1/affiliates/afid/"+afid+".json")
		switch {
		case err == nil:
			r.add("afid lookup form", "PROVEN works (200)")
		case status == http.StatusNotFound:
			r.add("afid lookup form", "404 → service will use the list fallback")
		default:
			r.add("afid lookup form", "UNKNOWN ("+err.Error()+")")
		}
	}
	det, _, err := c.get(ctx, mk("/api/v1/affiliates/"+url.PathEscape(sysID)+".json", nil), nil,
		retreaverHost+" /api/v1/affiliates/"+sysID+".json")
	if err != nil {
		fmt.Printf("  ERROR %v\n", err)
		r.add("detail GET", "UNKNOWN ("+err.Error()+")")
	} else {
		d := unwrap(asMap(det), "affiliate")
		fmt.Printf("  keys: %s\n", strings.Join(keys(d), ", "))
		if p, ok := d["paused"].(bool); ok {
			r.state = fmt.Sprintf("paused=%v", p)
			r.add("\"paused\" in GET (read-back)", "PROVEN present → read_back ok")
		} else {
			r.state = "not visible (GET has no \"paused\")"
			r.add("\"paused\" in GET (read-back)", "ABSENT → confirmed_by_response / read_back none")
		}
	}

	fmt.Println("\n=== Retreaver: numbers ===")
	// No documented afid filter for /api/v1/numbers.json, so list and filter locally.
	nTotal, withRef, hits := 0, 0, 0
	nListed := true
	for page := 1; page <= maxPages; page++ {
		q := url.Values{"page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(perPage)}}
		doc, _, err := c.get(ctx, mk("/api/v1/numbers.json", q), nil,
			fmt.Sprintf("%s /api/v1/numbers.json page=%d", retreaverHost, page))
		if err != nil {
			fmt.Printf("  ERROR %v\n", err)
			nListed = false
			break
		}
		items := asList(doc)
		nTotal += len(items)
		for _, it := range items {
			m := unwrap(asMap(it), "number")
			_, hasAfid := m["afid"]
			_, hasAffID := m["affiliate_id"]
			if hasAfid || hasAffID {
				withRef++
			}
			if (afid != "" && str(m["afid"]) == afid) || str(m["affiliate_id"]) == sysID {
				hits++
			}
		}
		if len(items) < perPage {
			break
		}
	}
	fmt.Printf("  numbers listed: %d, carrying afid/affiliate_id: %d, belonging to this affiliate: %d\n", nTotal, withRef, hits)
	switch {
	case !nListed && nTotal == 0:
		r.add("numbers assigned", "UNKNOWN (number list not readable)")
	case hits > 0:
		r.add("numbers assigned", fmt.Sprintf("NOT EMPTY (%d numbers) — stop", hits))
	case !nListed:
		r.add("numbers assigned", "UNKNOWN (number list only partly readable)")
	case nTotal == 0:
		r.add("numbers assigned", "PROVEN none (company lists no numbers)")
	case withRef == 0:
		r.add("numbers assigned", "UNKNOWN (number objects carry no afid/affiliate_id)")
	default:
		r.add("numbers assigned", "PROVEN none")
	}
	// Retreaver has no campaign↔affiliate membership endpoint (report §5.2); an affiliate reaches
	// a campaign only through a number, so no numbers implies no campaign traffic.
	if hits == 0 && nListed && withRef > 0 {
		r.add("campaign membership", "INFERRED none (no numbers; no documented membership endpoint)")
	} else {
		r.add("campaign membership", "UNKNOWN (no documented campaign↔affiliate endpoint)")
	}
	return r
}

// ---------------------------------------------------------------------------------------------
// Plan (printed, never executed)

func printPlan(platform, id, pathID string) {
	fmt.Println("\n=== PLANNED WRITE STEPS — NOT EXECUTED (GET-only phase; needs written owner approval) ===")
	if id == "" || pathID == "" {
		fmt.Println("  (no plan: the sandbox publisher was not found uniquely)")
		return
	}
	type step struct{ n, method, path, body, expect string }
	var steps []step
	if platform == "ringba" {
		p := "/v2/{accountId}/Publishers/" + pathID
		steps = []step{
			{"1", "GET", p, "", "enabled=true (record)"},
			{"2", "PATCH", p, `{"enabled":false}`, "2xx"},
			{"3", "GET", p, "", "enabled=false; publisher list in the UI shows Paused"},
			{"4", "PATCH", p, `{"enabled":true}`, "2xx"},
			{"5", "GET", p, "", "enabled=true; UI shows Active"},
		}
	} else {
		p := "/api/v1/affiliates/" + pathID + ".json"
		steps = []step{
			{"1", "GET", p, "", "record keys; paused present?"},
			{"2", "PUT", p, `{"affiliate":{"paused":true}}`, "2xx; record response keys"},
			{"3", "GET", p, "", "paused=true if the key exists, else none"},
			{"4", "PUT", p, `{"affiliate":{"paused":false}}`, "2xx"},
			{"5", "GET", p, "", "paused=false if the key exists, else none"},
		}
	}
	fmt.Printf("  %-2s  %-6s  %-44s  %-32s  %s\n", "#", "METHOD", "PATH (no query)", "BODY", "EXPECT")
	for _, s := range steps {
		fmt.Printf("  %-2s  %-6s  %-44s  %-32s  %s\n", s.n, s.method, s.path, s.body, s.expect)
	}
}

// ---------------------------------------------------------------------------------------------
// helpers

func hasPublisherRef(m map[string]any) bool {
	for k, v := range m {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "publisher") || strings.Contains(lk, "affiliate") {
			return true
		}
		if sub, ok := v.(map[string]any); ok && hasPublisherRef(sub) {
			return true
		}
		if arr, ok := v.([]any); ok {
			for _, it := range arr {
				if sub, ok := it.(map[string]any); ok && hasPublisherRef(sub) {
					return true
				}
			}
		}
	}
	return false
}

// containsValue reports whether any string value anywhere in v equals want.
func containsValue(v any, want string) bool {
	switch t := v.(type) {
	case string:
		return t == want
	case map[string]any:
		for _, vv := range t {
			if containsValue(vv, want) {
				return true
			}
		}
	case []any:
		for _, vv := range t {
			if containsValue(vv, want) {
				return true
			}
		}
	}
	return false
}

func summarize(v any) string {
	switch t := v.(type) {
	case []any:
		return fmt.Sprintf("array(len=%d)", len(t))
	case map[string]any:
		return fmt.Sprintf("object(keys=%d)", len(t))
	case nil:
		return "null"
	case string:
		return fmt.Sprintf("string(len=%d)", len(t))
	}
	return fmt.Sprint(v)
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func unwrap(m map[string]any, key string) map[string]any {
	if inner, ok := m[key].(map[string]any); ok {
		return inner
	}
	return m
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// asList returns a top-level array, or the first array value of a top-level object.
func asList(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	if m, ok := v.(map[string]any); ok {
		for _, k := range keys(m) {
			if arr, ok := m[k].([]any); ok {
				return arr
			}
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
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

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
