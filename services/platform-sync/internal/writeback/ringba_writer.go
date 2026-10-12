package writeback

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ringbaWriter is the Ringba write-back path, moved behind the Writer interface without any
// behaviour change: convert fires the account's conversion webhook ("My Webhook" preferred),
// void calls Ringba's void with the call's current revenue and payout, read-back is the same
// call lookup, and an unsettled read-back is pending.
type ringbaWriter struct {
	api RingbaAPI
}

// NewRingbaWriter wraps the Ringba adapter as a Writer.
func NewRingbaWriter(api RingbaAPI) Writer {
	return &ringbaWriter{api: api}
}

func (w *ringbaWriter) Platform() string { return "ringba" }

func (w *ringbaWriter) CheckCredentials(creds map[string]string) error {
	if strings.TrimSpace(creds["account_id"]) == "" {
		return fmt.Errorf("writeback: Ringba account_id missing on this tenant's credentials")
	}
	return nil
}

func (w *ringbaWriter) LookupCall(ctx context.Context, creds map[string]string, callID string) (CallState, error) {
	c, err := w.api.LookupCall(ctx, creds, callID)
	if err != nil {
		return CallState{}, err
	}
	return CallState{Call: c}, nil
}

func (w *ringbaWriter) Convert(ctx context.Context, creds map[string]string, callID string, amount float64) (WriteInfo, error) {
	hooks, err := w.api.ListConversionWebhooks(ctx, creds)
	if err != nil {
		return WriteInfo{}, PreWrite(err)
	}
	hook, err := pickConversionWebhook(hooks)
	if err != nil {
		return WriteInfo{}, PreWrite(err)
	}
	info := WriteInfo{WebhookName: hook.Name}
	if err := w.api.FireInboundConversion(ctx, hook.ID, callID, amount); err != nil {
		return info, err
	}
	return info, nil
}

func (w *ringbaWriter) Void(ctx context.Context, creds map[string]string, callID string, before CallState) (WriteInfo, error) {
	ref, err := w.api.VoidCall(ctx, creds, callID, before.Call.Revenue, before.Call.Payout)
	if err != nil {
		return WriteInfo{}, err
	}
	return WriteInfo{Ref: ref}, nil
}

func (w *ringbaWriter) ReadBack(ctx context.Context, creds map[string]string, callID string) (CallState, error) {
	return w.LookupCall(ctx, creds, callID)
}

func (w *ringbaWriter) IsConverted(c CallState) bool { return isConverted(c.Call) }

// ConvertSettled keeps the pre-registry rule: any converted state with revenue > 0 settles.
func (w *ringbaWriter) ConvertSettled(after CallState, _ float64) bool {
	return after.Call.HasConverted && after.Call.Revenue > 0
}

func (w *ringbaWriter) VoidSettled(after CallState) bool { return !isConverted(after.Call) }

func (w *ringbaWriter) SettleWindow() time.Duration { return 0 }

func (w *ringbaWriter) FailOnUnsettled() bool { return false }
