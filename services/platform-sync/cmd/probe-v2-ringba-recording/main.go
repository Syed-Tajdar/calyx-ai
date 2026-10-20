package main

// probe-v2-ringba-recording — v2 Phase 0 item 3 follow-up (Ringba, owner Q1). Not in any image.
//
// Reads RINGBA_RECORDING_USER, RINGBA_RECORDING_PASSWORD and RINGBA_RECORDING_TEST_URL from a
// local git-ignored env file (path given with -env). None of the three is ever printed.
//
// Three GET requests, redirects never followed:
//  1. the recording link without auth            → status only
//  2. the recording link with Basic Auth          → status + Location host only
//  3. 4 bytes (Range 0-3) of where (2) points     → format magic only
//
// Nothing is saved. Response bodies are discarded unread except the 4 bytes of step 3.
//
// Usage: go run ./cmd/probe-v2-ringba-recording -env /path/to/.env

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type getOnly struct{ next http.RoundTripper }

func (g getOnly) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet || r.URL.Scheme != "https" {
		return nil, errors.New("probe allows only https GET")
	}
	return g.next.RoundTrip(r)
}

var client = &http.Client{
	Timeout:       20 * time.Second,
	Transport:     getOnly{http.DefaultTransport},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func readEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("cannot open env file")
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, ok := strings.Cut(line, "=")
		if !ok || !strings.HasPrefix(k, "RINGBA_RECORDING_") {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out, nil
}

// host returns only the host of a URL (never path or query).
func host(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "(unparseable)"
	}
	return u.Host
}

func get(rawURL, user, pass, rng string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, errors.New("bad url")
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	resp, err := client.Do(req)
	if err != nil {
		// The error text contains the URL: report only the kind.
		return nil, errors.New("request failed (network/TLS)")
	}
	return resp, nil
}

func format(b []byte) string {
	switch {
	case len(b) >= 3 && string(b[:3]) == "ID3":
		return "MP3 (ID3 tag)"
	case len(b) >= 2 && b[0] == 0xFF && b[1]&0xE0 == 0xE0:
		return "MP3 (MPEG frame)"
	case len(b) >= 4 && string(b[:4]) == "RIFF":
		return "WAV (RIFF)"
	case len(b) >= 4 && string(b[:4]) == "OggS":
		return "Ogg"
	case len(b) >= 4 && string(b[:4]) == "fLaC":
		return "FLAC"
	}
	return fmt.Sprintf("unknown (magic % x)", b)
}

func main() {
	envPath := flag.String("env", "", "git-ignored env file with the three RINGBA_RECORDING_* values")
	flag.Parse()
	env, err := readEnv(*envPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	user, pass, link := env["RINGBA_RECORDING_USER"], env["RINGBA_RECORDING_PASSWORD"], env["RINGBA_RECORDING_TEST_URL"]
	if user == "" || pass == "" || link == "" {
		fmt.Fprintln(os.Stderr, "FAIL: one of the three RINGBA_RECORDING_* values is empty")
		os.Exit(1)
	}
	fmt.Printf("recording link: host=%s (path and query not printed)\n", host(link))

	// 1. Without auth: status only.
	r1, err := get(link, "", "", "bytes=0-0")
	if err != nil {
		fmt.Println("1. no auth:", err)
	} else {
		r1.Body.Close()
		fmt.Printf("1. no auth (no redirect followed): HTTP %d\n", r1.StatusCode)
	}

	// 2. With Basic Auth: status + Location host.
	r2, err := get(link, user, pass, "bytes=0-0")
	if err != nil {
		fmt.Println("2. basic auth:", err)
		os.Exit(1)
	}
	r2.Body.Close()
	loc := r2.Header.Get("Location")
	if loc != "" {
		if u, perr := url.Parse(loc); perr == nil && !u.IsAbs() {
			base, _ := url.Parse(link)
			loc = base.ResolveReference(u).String()
		}
		fmt.Printf("2. basic auth (no redirect followed): HTTP %d, Location host=%s\n", r2.StatusCode, host(loc))
	} else {
		fmt.Printf("2. basic auth (no redirect followed): HTTP %d, no Location header\n", r2.StatusCode)
	}

	// 3. Four bytes of the audio, from the redirect target (or the link itself if it answered 2xx).
	target, user3, pass3 := loc, "", ""
	if target == "" && r2.StatusCode/100 == 2 {
		target, user3, pass3 = link, user, pass
	}
	if target == "" {
		fmt.Println("3. format: not read (no redirect and no 2xx)")
		return
	}
	r3, err := get(target, user3, pass3, "bytes=0-3")
	if err != nil {
		fmt.Println("3. format:", err)
		return
	}
	b, _ := io.ReadAll(io.LimitReader(r3.Body, 4))
	r3.Body.Close()
	fmt.Printf("3. 4 bytes (Range 0-3): HTTP %d, content-type=%q, total size=%q, format=%s\n",
		r3.StatusCode, r3.Header.Get("Content-Type"), r3.Header.Get("Content-Range"), format(b))
}
