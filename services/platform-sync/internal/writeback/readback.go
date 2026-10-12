package writeback

import (
	"fmt"
	"math"
	"strings"
)

// ReadBackSummary is the platform read-back recorded in the audit row and returned to the
// caller: the call before the write, the call after it, how long the read-back took to settle,
// and the two things the owner watches — whether payout moved on its own and whether any pixel
// fired. Contains no credentials or caller data.
type ReadBackSummary struct {
	Before           *ReadBackFields `json:"before,omitempty"`
	After            *ReadBackFields `json:"after,omitempty"`
	SettleSeconds    float64         `json:"settle_seconds"`
	PayoutChanged    bool            `json:"payout_changed"`
	FiredPixelsDelta *int            `json:"fired_pixels_delta,omitempty"`
}

// summarize returns nil when neither side carries platform read-back fields (Ringba), so
// Ringba audit rows and responses keep their pre-registry shape.
func summarize(before, after *CallState, settleSeconds float64) *ReadBackSummary {
	var b, a *ReadBackFields
	if before != nil {
		b = before.ReadBack
	}
	if after != nil {
		a = after.ReadBack
	}
	if b == nil && a == nil {
		return nil
	}
	out := &ReadBackSummary{Before: b, After: a, SettleSeconds: math.Round(settleSeconds*1000) / 1000}
	if b != nil && a != nil {
		out.PayoutChanged = !sameMoney(b.Payout, a.Payout)
		if b.FiredPixelsCount != nil && a.FiredPixelsCount != nil {
			d := *a.FiredPixelsCount - *b.FiredPixelsCount
			out.FiredPixelsDelta = &d
		}
	}
	return out
}

// sameMoney treats null and 0 as different only when one side has a real amount.
func sameMoney(x, y *float64) bool {
	xv, yv := 0.0, 0.0
	if x != nil {
		xv = *x
	}
	if y != nil {
		yv = *y
	}
	return math.Abs(xv-yv) < 0.005
}

func describeReadBack(c CallState) string {
	rb := c.ReadBack
	if rb == nil {
		return fmt.Sprintf("revenue=%.2f converted=%t", c.Call.Revenue, c.Call.HasConverted)
	}
	parts := []string{
		"revenue=" + fmtFloatPtr(rb.Revenue),
		"converted=" + fmtBoolPtr(rb.Converted),
		"payout=" + fmtFloatPtr(rb.Payout),
	}
	return strings.Join(parts, " ")
}

func fmtFloatPtr(v *float64) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%.2f", *v)
}

func fmtBoolPtr(v *bool) string {
	if v == nil {
		return "absent"
	}
	return fmt.Sprintf("%t", *v)
}
