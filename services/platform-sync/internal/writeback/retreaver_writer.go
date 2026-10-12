package writeback

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
)

// RetreaverAPI is the transport the Retreaver writer needs (adapter.RetreaverCallClient).
type RetreaverAPI interface {
	GetCall(ctx context.Context, creds map[string]string, uuid string) (adapter.RetreaverCallReadBack, error)
	SetRevenue(ctx context.Context, creds map[string]string, uuid string, revenue float64) error
}

// retreaverWriter is the Retreaver write-back path (Stage 4b).
//
//	Convert(amount) → PUT /api/v5/calls/{uuid}.json {"call":{"revenue":amount}}
//	Void()          → PUT /api/v5/calls/{uuid}.json {"call":{"revenue":0}}
//	ReadBack()      → GET /api/v2/calls/{uuid}.json
//
// payout and session_notes are never sent (owner decision). The provisional settle rules
// below are Phase 1's; Phase 3 replaces them with what the sandbox proves:
//
//	convert succeeded = revenue == amount, and converted == true if the field is present
//	void succeeded    = revenue is 0 or null
type retreaverWriter struct {
	api    RetreaverAPI
	settle time.Duration
}

// NewRetreaverWriter returns the Retreaver Writer. settle is WRITEBACK_SETTLE_SECONDS_RETREAVER
// (ParseSettleWindow; default 20s until Phase 3 measures the real value).
func NewRetreaverWriter(api RetreaverAPI, settle time.Duration) Writer {
	if settle <= 0 {
		settle = SettleWindow
	}
	return &retreaverWriter{api: api, settle: settle}
}

func (w *retreaverWriter) Platform() string { return "retreaver" }

func (w *retreaverWriter) CheckCredentials(creds map[string]string) error {
	if strings.TrimSpace(creds["api_key"]) == "" || strings.TrimSpace(creds["company_id"]) == "" {
		return fmt.Errorf("writeback: Retreaver api_key and company_id missing on this tenant's credentials")
	}
	return nil
}

// roundCents is the amount actually written. Retreaver revenue is money; sending float noise
// such as 18.400000000000002 would make the read-back comparison flaky.
func roundCents(v float64) float64 { return math.Round(v*100) / 100 }

// ValidateRequest refuses, before the lock and before any request, a call id that is not a
// Retreaver UUID and a convert amount that would round to $0.00 — which Retreaver would treat
// as a void ("Passing 0 clears the revenue").
func (w *retreaverWriter) ValidateRequest(action, callID string, amount float64) error {
	if !adapter.IsRetreaverCallUUID(callID) {
		return ErrInvalidCallID
	}
	if action == "convert" && roundCents(amount) < 0.01 {
		return ErrInvalidAmount
	}
	return nil
}

func (w *retreaverWriter) state(rb adapter.RetreaverCallReadBack) CallState {
	return CallState{
		Call: rb.Call,
		ReadBack: &ReadBackFields{
			Converted:               rb.Converted,
			Revenue:                 rb.Revenue,
			Payout:                  rb.Payout,
			Receivable:              rb.Receivable,
			Payable:                 rb.Payable,
			ConversionsDeterminedAt: rb.ConversionsDeterminedAt,
			FiredPixelsCount:        rb.FiredPixelsCount,
			PostbackValue:           rb.PostbackValue,
			UpdatedAt:               rb.UpdatedAt,
		},
	}
}

// LookupCall reads the call with this tenant's key and company_id. A call that the API
// returns for a different company is reported with an empty ExternalCallID, which the
// Service audits and refuses as an account mismatch.
func (w *retreaverWriter) LookupCall(ctx context.Context, creds map[string]string, callID string) (CallState, error) {
	rb, err := w.api.GetCall(ctx, creds, callID)
	if err != nil {
		return CallState{}, err
	}
	st := w.state(rb)
	want := strings.TrimSpace(creds["company_id"])
	if rb.CompanyID != "" && want != "" && rb.CompanyID != want {
		st.Call.ExternalCallID = ""
	}
	return st, nil
}

func (w *retreaverWriter) Convert(ctx context.Context, creds map[string]string, callID string, amount float64) (WriteInfo, error) {
	return WriteInfo{}, w.api.SetRevenue(ctx, creds, callID, roundCents(amount))
}

func (w *retreaverWriter) Void(ctx context.Context, creds map[string]string, callID string, _ CallState) (WriteInfo, error) {
	return WriteInfo{}, w.api.SetRevenue(ctx, creds, callID, 0)
}

func (w *retreaverWriter) ReadBack(ctx context.Context, creds map[string]string, callID string) (CallState, error) {
	rb, err := w.api.GetCall(ctx, creds, callID)
	if err != nil {
		return CallState{}, err
	}
	return w.state(rb), nil
}

func revenueOf(c CallState) (float64, bool) {
	if c.ReadBack == nil || c.ReadBack.Revenue == nil {
		return 0, false
	}
	return *c.ReadBack.Revenue, true
}

func convertedFlagOK(c CallState) bool {
	return c.ReadBack == nil || c.ReadBack.Converted == nil || *c.ReadBack.Converted
}

// IsConverted mirrors the API's `converted` state: revenue > 0 and the converted flag true
// (or absent).
func (w *retreaverWriter) IsConverted(c CallState) bool {
	rev, ok := revenueOf(c)
	return ok && rev > 0 && convertedFlagOK(c)
}

func (w *retreaverWriter) ConvertSettled(after CallState, amount float64) bool {
	rev, ok := revenueOf(after)
	return ok && math.Abs(rev-roundCents(amount)) < 0.005 && convertedFlagOK(after)
}

func (w *retreaverWriter) VoidSettled(after CallState) bool {
	if after.ReadBack == nil {
		return false
	}
	rev, ok := revenueOf(after)
	return !ok || math.Abs(rev) < 0.005
}

func (w *retreaverWriter) SettleWindow() time.Duration { return w.settle }

func (w *retreaverWriter) FailOnUnsettled() bool { return true }
