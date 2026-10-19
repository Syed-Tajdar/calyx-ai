package main

// probe-v2-recording-head — v2 Phase 0 item 3 (Retreaver). Not built into any image.
//
// Reads one Retreaver call (GET v2 and GET v5), then checks its recording WITHOUT any auth:
// one HEAD, and at most two tiny Range GETs (10 bytes, then 4 bytes) to read the container and
// the MP3 channel mode. Nothing is stored. The recording URL is never printed: only its host,
// whether it carries a query string, and HTTP facts.
//
// Credentials come only from the stored integration, decrypted in-process; stdin carries
// CREDENTIAL_ENCRYPTION_KEY then DATABASE_URL.
//
// Usage: go run ./cmd/probe-v2-recording-head -integration <uuid> -call <uuid>

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
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/jackc/pgx/v5/pgxpool"
)

const tenantID = "11111111-1111-1111-1111-111111111111"

// readOnly refuses everything except GET and HEAD.
type readOnly struct{ next http.RoundTripper }

func (t readOnly) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return nil, fmt.Errorf("probe refuses method %s", r.Method)
	}
	if r.URL.Scheme != "https" {
		return nil, errors.New("probe refuses non-https")
	}
	return t.next.RoundTrip(r)
}

var hc = &http.Client{
	Timeout:   20 * time.Second,
	Transport: readOnly{http.DefaultTransport},
	// Never follow redirects: report them instead.
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

func main() {
	integrationID := flag.String("integration", "", "stored Retreaver integration UUID")
	callID := flag.String("call", "", "Retreaver call UUID")
	flag.Parse()
	if !adapter.IsRetreaverCallUUID(*integrationID) || !adapter.IsRetreaverCallUUID(*callID) {
		fatal(errors.New("need -integration <uuid> and -call <uuid>"))
	}
	in := bufio.NewScanner(os.Stdin)
	credKey, dbURL := readLine(in), readLine(in)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	defer pool.Close()
	var (
		platform, syncMode string
		blob               []byte
	)
	if err := pool.QueryRow(ctx, `SELECT platform, sync_mode, api_credentials_blob
		FROM get_api_sync_integration_any($1::uuid, $2::uuid)`, tenantID, *integrationID).Scan(&platform, &syncMode, &blob); err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	if platform != "retreaver" || syncMode == "live" {
		fatal(errors.New("integration must be a Retreaver Test (mock) integration"))
	}
	dec, err := crypto.NewCredentialDecryptor(credKey)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	creds, err := dec.DecryptMap(blob)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}

	v2 := getCall(ctx, creds, "v2", *callID)
	v5 := getCall(ctx, creds, "v5", *callID)
	recURL, _ := v2["recording_url"].(string)
	inbound, _ := v5["inbound_leg_recording_url"].(string)
	fmt.Printf("v2 recording_url present: %v %s\n", recURL != "", describe(recURL))
	fmt.Printf("v5 recording_url present: %v\n", str(v5["recording_url"]) != "")
	fmt.Printf("v5 inbound_leg_recording_url present: %v %s\n", inbound != "", describe(inbound))
	if recURL == "" {
		return
	}

	// 1. HEAD, no auth.
	resp, err := do(ctx, http.MethodHead, recURL, "")
	if err != nil {
		fatal(err)
	}
	resp.Body.Close()
	report("HEAD (no auth)", resp)

	// 2. 10 bytes, no auth: container / ID3 header.
	head, resp2, err := rangeGet(ctx, recURL, 0, 9)
	if err != nil {
		fatal(err)
	}
	report("GET Range 0-9 (no auth)", resp2)
	if len(head) < 10 {
		return
	}
	offset := int64(0)
	if string(head[:3]) == "ID3" {
		size := int64(head[6]&0x7f)<<21 | int64(head[7]&0x7f)<<14 | int64(head[8]&0x7f)<<7 | int64(head[9]&0x7f)
		offset = 10 + size
		fmt.Printf("container: ID3v2.%d tag, %d bytes, MP3 frames start at %d\n", head[3], size, offset)
	} else {
		fmt.Printf("container magic: % x\n", head[:4])
	}

	// 3. 4 bytes at the first MP3 frame: channel mode.
	fr, _, err := rangeGet(ctx, recURL, offset, offset+3)
	if err != nil || len(fr) < 4 {
		fmt.Println("frame header: not read")
		return
	}
	if fr[0] != 0xFF || fr[1]&0xE0 != 0xE0 {
		fmt.Printf("frame header: no MPEG sync at offset (% x)\n", fr)
		return
	}
	version := map[byte]string{0: "MPEG 2.5", 2: "MPEG 2", 3: "MPEG 1"}[(fr[1]>>3)&3]
	layer := map[byte]string{1: "Layer III", 2: "Layer II", 3: "Layer I"}[(fr[1]>>1)&3]
	mode := map[byte]string{0: "stereo", 1: "joint stereo", 2: "dual channel", 3: "mono"}[(fr[3]>>6)&3]
	rates := map[string][]int{"MPEG 1": {44100, 48000, 32000}, "MPEG 2": {22050, 24000, 16000}, "MPEG 2.5": {11025, 12000, 8000}}
	sr := "?"
	if i := (fr[2] >> 2) & 3; i < 3 && rates[version] != nil {
		sr = fmt.Sprint(rates[version][i])
	}
	fmt.Printf("frame header: %s %s, %s Hz, channel mode %s\n", version, layer, sr, mode)
}

func getCall(ctx context.Context, creds map[string]string, version, uuid string) map[string]any {
	q := url.Values{}
	q.Set("api_key", creds["api_key"])
	q.Set("company_id", creds["company_id"])
	u := "https://api.retreaver.com/api/" + version + "/calls/" + uuid + ".json?" + q.Encode()
	resp, err := do(ctx, http.MethodGet, u, "")
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	fmt.Printf("GET %s -> %d\n", redact.RedactURL(u), resp.StatusCode)
	var doc map[string]any
	if json.Unmarshal(body, &doc) != nil {
		return map[string]any{}
	}
	if c, ok := doc["call"].(map[string]any); ok {
		return c
	}
	return doc
}

func do(ctx context.Context, method, u, rng string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, errors.New("bad request")
	}
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	resp, err := hc.Do(req)
	if err != nil {
		// The error text can contain the URL; print only the error kind.
		return nil, fmt.Errorf("%s failed: %s", method, redact.RedactMessage(errKind(err)))
	}
	return resp, nil
}

func rangeGet(ctx context.Context, u string, from, to int64) ([]byte, *http.Response, error) {
	resp, err := do(ctx, http.MethodGet, u, fmt.Sprintf("bytes=%d-%d", from, to))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, to-from+1))
	return b, resp, nil
}

func report(label string, r *http.Response) {
	loc := ""
	if l := r.Header.Get("Location"); l != "" {
		loc = " redirect-to " + describe(l)
	}
	fmt.Printf("%s -> %d content-type=%q content-length=%q accept-ranges=%q content-range=%q www-authenticate=%v%s\n",
		label, r.StatusCode, r.Header.Get("Content-Type"), r.Header.Get("Content-Length"),
		r.Header.Get("Accept-Ranges"), r.Header.Get("Content-Range"), r.Header.Get("WWW-Authenticate") != "", loc)
}

// describe prints only non-sensitive facts about a URL.
func describe(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "(unparseable)"
	}
	ext := ""
	if i := strings.LastIndex(u.Path, "."); i >= 0 && len(u.Path)-i <= 5 {
		ext = u.Path[i:]
	}
	return fmt.Sprintf("(host=%s ext=%s has_query=%v query_has_api_key=%v)", u.Host, ext, u.RawQuery != "", u.Query().Get("api_key") != "")
}

func errKind(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i >= 0 {
		return s[i+2:]
	}
	return "transport error"
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func readLine(s *bufio.Scanner) string {
	if s.Scan() {
		return strings.TrimSpace(s.Text())
	}
	return ""
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "FAIL: %s\n", redact.RedactMessage(err.Error()))
	os.Exit(1)
}
