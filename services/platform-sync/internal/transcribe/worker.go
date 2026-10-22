package transcribe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"time"
)

// Job states (transcription_jobs.state).
const (
	StateQueued      = "queued"
	StateDone        = "done"
	StateNoRecording = "no_recording"
	StateFailed      = "failed"
	StateCapped      = "capped"
)

type WorkerConfig struct {
	DailyCapMinutes float64        // hard stop per tenant per Eastern day
	MaxAttempts     int            // retries for transient failures, then failed
	SpanishPattern  *regexp.Regexp // campaign name → language=multi + numbers redaction
	AllowMock       bool           // Test-mode integrations (never in production)
}

// Decrypt opens an encrypted credentials blob (crypto.CredentialDecryptor.DecryptMap).
type Decrypt func(blob []byte) (map[string]string, error)

// Worker runs one job at a time: fetch once → SHA-256 → archive → Deepgram (same bytes) → scrub → store.
type Worker struct {
	Store   Store
	Decrypt Decrypt
	Sources map[string]Source // "ringba", "retreaver"
	Mock    Source
	DG      Transcriber
	Archive ObjectStore // nil when the archive is not configured: recordings are then "not_archived"
	Cfg     WorkerConfig
	Logger  *slog.Logger
	Now     func() time.Time

	cappedUntil map[string]time.Time // tenant → next Eastern midnight
}

var eastern = func() *time.Location {
	if l, err := time.LoadLocation("America/New_York"); err == nil {
		return l
	}
	return time.FixedZone("ET", -5*3600)
}()

// nextEasternMidnight: when the daily cap resets.
func nextEasternMidnight(now time.Time) time.Time {
	t := now.In(eastern)
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 30, 0, eastern).UTC()
}

// backoff: 2, 4, 8 … minutes, at most 6 hours.
func backoff(attempts int) time.Duration {
	d := time.Duration(math.Pow(2, float64(attempts))) * time.Minute
	if d > 6*time.Hour {
		d = 6 * time.Hour
	}
	return d
}

func (w *Worker) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}

// Run processes jobs until ctx ends: one after another while there is work, every 15 s otherwise.
func (w *Worker) Run(ctx context.Context) {
	w.Logger.Info("transcription worker started", "daily_cap_minutes", w.Cfg.DailyCapMinutes,
		"archive_configured", w.Archive != nil, "mip_opt_out", true)
	lastRequeue := time.Time{}
	for ctx.Err() == nil {
		if time.Since(lastRequeue) > 10*time.Minute {
			if n, err := w.Store.RequeueStale(ctx, 30); err == nil && n > 0 {
				w.Logger.Warn("transcription jobs requeued after a stop", "count", n)
			}
			lastRequeue = time.Now()
		}
		worked, err := w.RunOnce(ctx)
		if err != nil {
			w.Logger.Error("transcription worker", "error", safeErr(err))
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
		case <-time.After(15 * time.Second):
		}
	}
}

// RunOnce claims and processes one job. worked=false when nothing was due.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	job, err := w.Store.ClaimJob(ctx)
	if err != nil || job == nil {
		return false, err
	}
	state, reason, errText, next := w.process(ctx, job)
	if err := w.Store.FinishJob(context.Background(), job.ID, state, reason, errText, next); err != nil {
		return true, fmt.Errorf("finish job: %w", err)
	}
	w.Logger.Info("transcription job finished", "job_id", job.ID, "platform", job.Platform,
		"external_call_id", job.ExternalCallID, "reason", job.Reason, "state", state,
		"no_recording_reason", reason, "attempts", job.Attempts, "error", errText)
	return true, nil
}

func (w *Worker) retryOrFail(job *Job, err error) (string, string, string, *time.Time) {
	text := safeErr(err)
	if job.Attempts >= w.Cfg.MaxAttempts {
		return StateFailed, "", text, nil
	}
	next := w.now().Add(backoff(job.Attempts))
	return StateQueued, "", text, &next
}

func (w *Worker) classify(job *Job, err error) (string, string, string, *time.Time) {
	var nr *NoRecordingError
	var fe *FinalError
	switch {
	case errors.As(err, &nr):
		return StateNoRecording, nr.Reason, nr.Detail, nil
	case errors.As(err, &fe):
		return StateFailed, "", safeErr(err), nil
	default: // TransientError and anything unexpected: retry with backoff
		return w.retryOrFail(job, err)
	}
}

func (w *Worker) process(ctx context.Context, job *Job) (state, noRecReason, errText string, next *time.Time) {
	transcribe := job.Reason != "archive_only"

	// Hard daily cap: the job waits for the next Eastern day, nothing is fetched or sent.
	if transcribe {
		if until, ok := w.cappedUntil[job.TenantID]; ok && w.now().Before(until) {
			return StateCapped, "", "daily minutes cap reached", &until
		}
		used, err := w.Store.MinutesToday(ctx, job.TenantID)
		if err != nil {
			return w.retryOrFail(job, err)
		}
		estimate := float64(job.DurationSeconds) / 60
		if estimate < 1 {
			estimate = 1
		}
		if used+estimate > w.Cfg.DailyCapMinutes {
			until := nextEasternMidnight(w.now())
			if w.cappedUntil == nil {
				w.cappedUntil = map[string]time.Time{}
			}
			w.cappedUntil[job.TenantID] = until
			w.Logger.Warn("transcription daily cap reached — queue stopped until the next Eastern day",
				"tenant_id", job.TenantID, "used_minutes", used, "cap_minutes", w.Cfg.DailyCapMinutes)
			return StateCapped, "", "daily minutes cap reached", &until
		}
	}

	src, err := w.source(job)
	if err != nil {
		return StateFailed, "", err.Error(), nil
	}
	call := Call{Platform: job.Platform, ExternalCallID: job.ExternalCallID, OccurredAt: job.OccurredAt, SyncMode: job.SyncMode}
	if len(job.APICredentialsBlob) > 0 && job.SyncMode == "real" {
		if call.Credentials, err = w.Decrypt(job.APICredentialsBlob); err != nil {
			return StateFailed, "", "re-enter the integration credentials", nil
		}
	}
	if len(job.RecordingAccessBlob) > 0 {
		if call.RecordingAccess, err = w.Decrypt(job.RecordingAccessBlob); err != nil {
			return StateFailed, "", "re-enter the Recording Access credentials", nil
		}
	}

	// 1. Fetch ONCE. The same bytes go to the archive and to Deepgram.
	audio, err := src.Fetch(ctx, call)
	if err != nil {
		return w.classify(job, err)
	}
	sha := audio.SHA256()
	downloaded := w.now()

	// 2. Archive. A failure does not block transcription: the row says not_archived and why.
	rec := RecordingRow{
		TenantID: job.TenantID, Platform: job.Platform, ExternalCallID: job.ExternalCallID, JobID: job.ID,
		SHA256: sha, Bytes: int64(len(audio.Bytes)), DurationSeconds: float64(job.DurationSeconds),
		ContentType: audio.ContentType, DownloadedAt: downloaded, ArchiveState: "not_archived",
	}
	if w.Archive == nil {
		rec.ArchiveError = "archive not configured"
	} else {
		key := ObjectKey(job.TenantID, job.Platform, downloaded, sha, audio.ContentType)
		exists, aerr := w.Archive.Exists(ctx, key)
		if aerr == nil && !exists {
			aerr = w.Archive.Put(ctx, key, audio.Bytes, audio.ContentType)
		}
		if aerr != nil {
			rec.ArchiveError = safeErr(aerr)
			w.Logger.Warn("recording not archived (transcription continues)", "job_id", job.ID, "error", rec.ArchiveError)
		} else {
			rec.ArchiveState, rec.ObjectKey = "archived", key
		}
	}
	recID, err := w.Store.UpsertRecording(ctx, rec)
	if err != nil {
		return w.retryOrFail(job, err)
	}
	if !transcribe {
		return StateDone, "", "", nil
	}

	// 3. Deepgram, with the bytes already in memory (no URL leaves Calyx).
	opts := TranscribeOptions{Spanish: IsSpanishCampaign(job.CampaignName, w.Cfg.SpanishPattern)}
	res, err := w.DG.Transcribe(ctx, audio, opts)
	if err != nil {
		return w.classify(job, err)
	}

	// 4. Calyx scrub on every utterance, whatever Deepgram redacted.
	scrubbed := ScrubUtterances(res.Utterances)
	if res.Duration > 0 {
		rec.DurationSeconds = res.Duration
		_, _ = w.Store.UpsertRecording(ctx, rec)
	}
	if _, err := w.Store.InsertTranscript(ctx, TranscriptRow{
		TenantID: job.TenantID, Platform: job.Platform, ExternalCallID: job.ExternalCallID,
		RecordingID: recID, RecordingSHA256: sha, Language: res.Language, Model: res.Model, RequestID: res.RequestID,
		DurationSeconds: res.Duration, Redaction: res.Redaction + "+calyx", Scrubbed: scrubbed, Utterances: res.Utterances,
	}); err != nil {
		return w.retryOrFail(job, err)
	}
	minutes := res.Duration / 60
	if minutes <= 0 {
		minutes = float64(job.DurationSeconds) / 60
	}
	if _, err := w.Store.AddUsage(ctx, job.TenantID, math.Round(minutes*100)/100); err != nil {
		w.Logger.Error("transcription usage not recorded", "job_id", job.ID, "error", safeErr(err))
	}
	return StateDone, "", "", nil
}

func (w *Worker) source(job *Job) (Source, error) {
	if job.SyncMode != "real" {
		if !w.Cfg.AllowMock || w.Mock == nil {
			return nil, fmt.Errorf("Test-mode integrations are not transcribed here")
		}
		return w.Mock, nil
	}
	s, ok := w.Sources[job.Platform]
	if !ok {
		return nil, fmt.Errorf("no recording source for %s", job.Platform)
	}
	return s, nil
}
