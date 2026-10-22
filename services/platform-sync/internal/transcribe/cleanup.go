package transcribe

import (
	"context"
	"log/slog"
	"time"
)

// Cleanup enforces the archive retention (RECORDING_RETENTION_MONTHS, 24 by owner decision) next to the
// bucket's own lifecycle rule: it deletes each object past retention, then marks the row expired. The
// transcript (and later QA) stays. A delete that fails leaves the row archived, so the next run retries.
type Cleanup struct {
	Store           Store
	Archive         ObjectStore
	RetentionMonths int
	Logger          *slog.Logger
	Now             func() time.Time
}

func (c *Cleanup) RunOnce(ctx context.Context) (expired int, err error) {
	now := time.Now().UTC()
	if c.Now != nil {
		now = c.Now().UTC()
	}
	before := now.AddDate(0, -c.RetentionMonths, 0)
	for {
		rows, err := c.Store.ListToExpire(ctx, before, 200)
		if err != nil {
			return expired, err
		}
		if len(rows) == 0 {
			return expired, nil
		}
		progress := 0
		for _, r := range rows {
			if r.ObjectKey != "" {
				if c.Archive == nil {
					return expired, nil // nothing can be deleted until the archive is configured
				}
				if err := c.Archive.Delete(ctx, r.ObjectKey); err != nil {
					c.Logger.Warn("expired recording not deleted yet", "recording_id", r.ID, "error", safeErr(err))
					continue
				}
			}
			if err := c.Store.MarkExpired(ctx, r.ID); err != nil {
				return expired, err
			}
			expired++
			progress++
		}
		if progress == 0 {
			return expired, nil
		}
	}
}

// Run once a day.
func (c *Cleanup) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if n, err := c.RunOnce(ctx); err != nil {
			c.Logger.Error("recording cleanup", "error", safeErr(err))
		} else if n > 0 {
			c.Logger.Info("recording cleanup", "expired", n, "retention_months", c.RetentionMonths)
		}
		select {
		case <-ctx.Done():
		case <-time.After(24 * time.Hour):
		}
	}
}
