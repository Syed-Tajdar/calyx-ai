package main

// probe-retreaver-writeback — Stage 4b Phase 3. Not built into any image.
//
// Runs the owner-approved sandbox plan (docs/retreaver-writeback-report.md §5.5) one step per
// invocation, through the real write-back transport (adapter.RetreaverCallClient: PUT v5
// revenue, GET v2 read-back, shared per-company limiter, retry rules, redaction).
//
// Hard limits, enforced in code:
//   - only the sandbox call below; any other -call is refused
//   - only revenue 1 or 0; the body type has no other field
//   - only GET and PUT, only to api.retreaver.com
//   - refuses a Live integration
//   - stops (exit 2) when fired_pixels_count leaves its baseline, payout or payable appears,
//     the company check fails, a PUT errors, or the read-back never settles
//
// Credentials come only from the stored integration, decrypted in-process. stdin carries
// CREDENTIAL_ENCRYPTION_KEY then DATABASE_URL, so neither is on a command line. The api_key and
// company_id are never printed.
//
// Usage: go run ./cmd/probe-retreaver-writeback -integration <uuid> -step a|b|d|f|g|h|i

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	tenantID       = "11111111-1111-1111-1111-111111111111"
	sandboxCall    = "a82a7a80-f1fd-46a5-a018-379dd7893ea7"
	apiHost        = "api.retreaver.com"
	baselinePixels = 1
	pollEvery      = 3 * time.Second
	pollMax        = 60 * time.Second
)

// step is one row of the approved plan. Convert steps write 1, void steps write 0.
type step struct {
	write     *float64
	needState string // "unconverted", "converted" or "" (any)
	desc      string
}

func f(v float64) *float64 { return &v }

var plan = map[string]step{
	"a": {nil, "", "read (baseline)"},
	"b": {f(1), "unconverted", "convert $1, then read until settled (c)"},
	"d": {f(0), "converted", "void, then read until settled (e)"},
	"f": {f(1), "unconverted", "convert $1 again, then read until settled"},
	"g": {f(1), "converted", "convert $1 once more (proves absolute: expect 1, not 2)"},
	"h": {f(0), "converted", "void, then read until settled"},
	"i": {nil, "", "final read (end state)"},
}

type restrictedTransport struct{ next http.RoundTripper }

func (t restrictedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		return nil, fmt.Errorf("probe refuses method %s", r.Method)
	}
	if r.URL.Scheme != "https" || r.URL.Host != apiHost {
		return nil, fmt.Errorf("probe refuses host %s", r.URL.Host)
	}
	if !strings.Contains(r.URL.Path, sandboxCall) {
		return nil, errors.New("probe refuses a path that is not the sandbox call")
	}
	return t.next.RoundTrip(r)
}

type readOut struct {
	At                      string   `json:"at"`
	SecondsSinceWrite       *float64 `json:"seconds_since_write,omitempty"`
	CompanyMatch            bool     `json:"company_match"`
	Converted               *bool    `json:"converted"`
	Revenue                 *float64 `json:"revenue"`
	Payout                  *float64 `json:"payout"`
	Receivable              *bool    `json:"receivable"`
	Payable                 *bool    `json:"payable"`
	FiredPixelsCount        *int     `json:"fired_pixels_count"`
	ConversionsDeterminedAt *string  `json:"conversions_determined_at"`
	PostbackValue           *string  `json:"postback_value"`
	UpdatedAt               *string  `json:"updated_at"`
}

func main() {
	integrationID := flag.String("integration", "", "stored Retreaver integration UUID")
	stepName := flag.String("step", "", "plan step: a, b, d, f, g, h or i")
	callID := flag.String("call", sandboxCall, "must be the sandbox call")
	flag.Parse()
	st, ok := plan[*stepName]
	if !ok {
		fatal(errors.New("-step must be one of a b d f g h i"))
	}
	if *callID != sandboxCall {
		fatal(errors.New("only the sandbox call may be used"))
	}
	if !adapter.IsRetreaverCallUUID(*integrationID) {
		fatal(errors.New("need -integration <uuid>"))
	}
	if st.write != nil && *st.write != 0 && *st.write != 1 {
		fatal(errors.New("plan amount must be 0 or 1"))
	}

	in := bufio.NewScanner(os.Stdin)
	credKey, dbURL := readLine(in), readLine(in)
	if credKey == "" || dbURL == "" {
		fatal(errors.New("stdin must carry CREDENTIAL_ENCRYPTION_KEY then DATABASE_URL"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fatal(errors.New(redact.SanitizeErr(err)))
	}
	defer pool.Close()
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
	fmt.Printf("integration platform=%s name=%q sync_mode=%s writeback_enabled=%v\n", platform, name, syncMode, writeback)
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
	stored := strings.TrimSpace(creds["company_id"])

	client := adapter.NewRetreaverCallClientWith(adapter.RetreaverCallClientOptions{
		HTTPClient: &http.Client{
			Timeout:   25 * time.Second,
			Transport: restrictedTransport{http.DefaultTransport},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	})

	gets, puts := 0, 0
	read := func(since *time.Time) readOut {
		gets++
		rb, err := client.GetCall(ctx, creds, sandboxCall)
		if err != nil {
			stop("read-back GET failed: " + err.Error())
		}
		o := readOut{
			At: time.Now().UTC().Format(time.RFC3339), CompanyMatch: rb.CompanyID == stored,
			Converted: rb.Converted, Revenue: rb.Revenue, Payout: rb.Payout, Receivable: rb.Receivable,
			Payable: rb.Payable, FiredPixelsCount: rb.FiredPixelsCount,
			ConversionsDeterminedAt: rb.ConversionsDeterminedAt, PostbackValue: rb.PostbackValue, UpdatedAt: rb.UpdatedAt,
		}
		if since != nil {
			s := math.Round(time.Since(*since).Seconds()*10) / 10
			o.SecondsSinceWrite = &s
		}
		b, _ := json.Marshal(o)
		fmt.Println("READ " + redact.RedactMessage(string(b)))
		guard(o)
		return o
	}

	fmt.Printf("STEP %s — %s\n", *stepName, st.desc)
	before := read(nil)
	if st.needState == "unconverted" && revenue(before) > 0 {
		stop("precondition: call must be unconverted before this step")
	}
	if st.needState == "converted" && revenue(before) <= 0 {
		stop("precondition: call must be converted before this step")
	}
	if st.write == nil {
		fmt.Printf("DONE step=%s gets=%d puts=0\n", *stepName, gets)
		return
	}

	amount := *st.write
	writeAt := time.Now()
	puts++
	if err := client.SetRevenue(ctx, creds, sandboxCall, amount); err != nil {
		stop("PUT failed: " + err.Error())
	}
	fmt.Printf("PUT revenue=%v accepted at %s\n", amount, writeAt.UTC().Format(time.RFC3339))

	// Step g writes the value the call already has, so "revenue == amount" is true at once.
	// Watch the whole window instead: revenue must never become 2, and updated_at may move.
	sameValue := math.Abs(revenue(before)-amount) < 0.005
	for {
		time.Sleep(pollEvery)
		o := read(&writeAt)
		if sameValue {
			if math.Abs(revenue(o)-amount) >= 0.005 {
				stop(fmt.Sprintf("revenue moved to %v after re-writing %v (not absolute?)", revenue(o), amount))
			}
			if str(o.UpdatedAt) != str(before.UpdatedAt) || time.Since(writeAt) > 30*time.Second {
				fmt.Printf("SETTLED step=%s revenue stayed %v for %.1fs gets=%d puts=%d updated_at_moved=%v\n",
					*stepName, amount, time.Since(writeAt).Seconds(), gets, puts, str(o.UpdatedAt) != str(before.UpdatedAt))
				return
			}
			continue
		}
		if math.Abs(revenue(o)-amount) < 0.005 {
			fmt.Printf("SETTLED step=%s revenue=%v after %.1fs gets=%d puts=%d updated_at_moved=%v\n",
				*stepName, amount, time.Since(writeAt).Seconds(), gets, puts, str(o.UpdatedAt) != str(before.UpdatedAt))
			return
		}
		if revenue(o) > 0 && math.Abs(revenue(o)-amount) >= 0.005 && math.Abs(revenue(o)-revenue(before)) >= 0.005 {
			stop(fmt.Sprintf("read-back contradicts the write: revenue %v", revenue(o)))
		}
		if time.Since(writeAt) > pollMax {
			stop(fmt.Sprintf("not settled within %s", pollMax))
		}
	}
}

// guard enforces the plan's stop rules on every read.
func guard(o readOut) {
	if !o.CompanyMatch {
		stop("company check is not true")
	}
	if o.FiredPixelsCount == nil || *o.FiredPixelsCount != baselinePixels {
		stop("fired_pixels_count left its baseline of 1")
	}
	if o.Payout != nil && *o.Payout != 0 {
		stop("payout changed")
	}
	if o.Payable != nil && *o.Payable {
		stop("payable became true")
	}
}

func revenue(o readOut) float64 {
	if o.Revenue == nil {
		return 0
	}
	return *o.Revenue
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func readLine(s *bufio.Scanner) string {
	if s.Scan() {
		return strings.TrimSpace(s.Text())
	}
	return ""
}

func stop(msg string) {
	fmt.Fprintf(os.Stderr, "STOP: %s\n", redact.RedactMessage(msg))
	os.Exit(2)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "FAIL: %s\n", redact.RedactMessage(err.Error()))
	os.Exit(1)
}
