package transcribe

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- fakes ----------------------------------------------------------------------------------------

type fakeStore struct {
	mu          sync.Mutex
	jobs        []*Job
	finished    map[string][]string // job id → [state, reason, err]
	nextTry     map[string]*time.Time
	minutes     map[string]float64
	recordings  map[string]RecordingRow // tenant|platform|call
	recIDs      map[string]string
	transcripts []TranscriptRow
	expired     map[string]bool
}

func newFakeStore(jobs ...*Job) *fakeStore {
	return &fakeStore{jobs: jobs, finished: map[string][]string{}, nextTry: map[string]*time.Time{}, minutes: map[string]float64{},
		recordings: map[string]RecordingRow{}, recIDs: map[string]string{}, expired: map[string]bool{}}
}

func rkey(t, p, c string) string { return t + "|" + p + "|" + c }

func (f *fakeStore) ClaimJob(context.Context) (*Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.jobs) == 0 {
		return nil, nil
	}
	j := f.jobs[0]
	f.jobs = f.jobs[1:]
	j.Attempts++
	return j, nil
}
func (f *fakeStore) FinishJob(_ context.Context, id, state, reason, errText string, next *time.Time) error {
	f.finished[id] = []string{state, reason, errText}
	f.nextTry[id] = next
	return nil
}
func (f *fakeStore) RequeueStale(context.Context, int) (int, error) { return 0, nil }
func (f *fakeStore) MinutesToday(_ context.Context, t string) (float64, error) {
	return f.minutes[t], nil
}
func (f *fakeStore) AddUsage(_ context.Context, t string, m float64) (float64, error) {
	f.minutes[t] += m
	return f.minutes[t], nil
}
func (f *fakeStore) UpsertRecording(_ context.Context, r RecordingRow) (string, error) {
	k := rkey(r.TenantID, r.Platform, r.ExternalCallID)
	f.recordings[k] = r
	if f.recIDs[k] == "" {
		f.recIDs[k] = "rec-" + r.ExternalCallID
	}
	return f.recIDs[k], nil
}
func (f *fakeStore) InsertTranscript(_ context.Context, t TranscriptRow) (string, error) {
	f.transcripts = append(f.transcripts, t)
	return "tr-1", nil
}
func (f *fakeStore) GetRecordingObject(_ context.Context, t, p, c string) (*RecordingObject, error) {
	r, ok := f.recordings[rkey(t, p, c)]
	if !ok {
		return nil, nil
	}
	state := r.ArchiveState
	if f.expired[f.recIDs[rkey(t, p, c)]] {
		state = "expired"
	}
	return &RecordingObject{ID: f.recIDs[rkey(t, p, c)], SHA256: r.SHA256, Bytes: r.Bytes, ContentType: r.ContentType,
		ObjectKey: r.ObjectKey, ArchiveState: state, DownloadedAt: r.DownloadedAt, DurationSeconds: r.DurationSeconds}, nil
}
func (f *fakeStore) LatestTranscript(_ context.Context, t, p, c string) (*TranscriptDoc, error) {
	for _, tr := range f.transcripts {
		if tr.TenantID == t && tr.Platform == p && tr.ExternalCallID == c {
			us, _ := json.Marshal(tr.Utterances)
			return &TranscriptDoc{ID: "tr-1", Version: 1, Model: tr.Model, RequestID: tr.RequestID, RecordingSHA256: tr.RecordingSHA256, Utterances: us}, nil
		}
	}
	return nil, nil
}
func (f *fakeStore) ListToExpire(_ context.Context, before time.Time, _ int) ([]ExpireRow, error) {
	var out []ExpireRow
	for k, r := range f.recordings {
		id := f.recIDs[k]
		if r.ArchiveState == "archived" && !f.expired[id] && r.DownloadedAt.Before(before) {
			out = append(out, ExpireRow{ID: id, ObjectKey: r.ObjectKey})
		}
	}
	return out, nil
}
func (f *fakeStore) MarkExpired(_ context.Context, id string) error { f.expired[id] = true; return nil }

// countingSource is a fake recording source that remembers how often it was asked.
type countingSource struct {
	audio Audio
	err   error
	calls int
}

func (s *countingSource) Fetch(context.Context, Call) (Audio, error) {
	s.calls++
	return s.audio, s.err
}

type fakeDG struct {
	got   []Audio
	opts  []TranscribeOptions
	res   TranscriptResult
	err   error
	calls int
}

func (d *fakeDG) Transcribe(_ context.Context, a Audio, o TranscribeOptions) (TranscriptResult, error) {
	d.calls++
	d.got = append(d.got, a)
	d.opts = append(d.opts, o)
	return d.res, d.err
}

const tenant = "11111111-1111-1111-1111-111111111111"

func job(callID string) *Job {
	return &Job{ID: "job-" + callID, TenantID: tenant, Platform: "retreaver", ExternalCallID: callID, Reason: "converted",
		DurationSeconds: 120, SyncMode: "real", APICredentialsBlob: []byte("blob"), CampaignName: "Final Expense"}
}

func testWorker(store Store, src Source, dg Transcriber, archive ObjectStore, logs *bytes.Buffer) *Worker {
	return &Worker{
		Store: store, Decrypt: func([]byte) (map[string]string, error) { return map[string]string{"api_key": "k"}, nil },
		Sources: map[string]Source{"retreaver": src, "ringba": src}, DG: dg, Archive: archive,
		Cfg:    WorkerConfig{DailyCapMinutes: 800, MaxAttempts: 3},
		Logger: slog.New(slog.NewJSONHandler(logs, nil)),
		Now:    func() time.Time { return time.Date(2026, 10, 22, 15, 0, 0, 0, time.UTC) },
	}
}

func goodDG() *fakeDG {
	return &fakeDG{res: TranscriptResult{RequestID: "req-1", Model: "nova-3 general", Language: "en", Duration: 90,
		Redaction: "deepgram:pci,ssn", Utterances: []Utterance{
			{Index: 0, Speaker: 0, Text: "thank you for calling"},
			{Index: 1, Speaker: 1, Text: "my card is 4111 1111 1111 1111"},
		}}}
}

// ---- tests ----------------------------------------------------------------------------------------

func TestSingleFetchForArchiveAndDeepgram(t *testing.T) {
	st := newFakeStore(job("c1"))
	src := &countingSource{audio: Audio{Bytes: []byte("mp3-bytes-once"), ContentType: "audio/mpeg"}}
	dg, arc, logs := goodDG(), NewMemoryStore(), &bytes.Buffer{}
	if _, err := testWorker(st, src, dg, arc, logs).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if src.calls != 1 {
		t.Fatalf("the recording must be fetched once, got %d", src.calls)
	}
	sum := sha256.Sum256([]byte("mp3-bytes-once"))
	want := hex.EncodeToString(sum[:])
	rec := st.recordings[rkey(tenant, "retreaver", "c1")]
	if rec.SHA256 != want || rec.ArchiveState != "archived" || !strings.HasSuffix(rec.ObjectKey, "/2026/10/"+want+".mp3") ||
		!strings.HasPrefix(rec.ObjectKey, "recordings/"+tenant+"/retreaver/") {
		t.Fatalf("recording row %+v", rec)
	}
	if string(arc.Objects[rec.ObjectKey]) != "mp3-bytes-once" || string(dg.got[0].Bytes) != "mp3-bytes-once" {
		t.Fatal("the archive and Deepgram must get the same bytes")
	}
	if st.finished["job-c1"][0] != StateDone || len(st.transcripts) != 1 || st.transcripts[0].RecordingSHA256 != want {
		t.Fatalf("job %v, transcripts %+v", st.finished["job-c1"], st.transcripts)
	}
	if st.minutes[tenant] != 1.5 {
		t.Fatalf("usage = %v minutes, want 1.5 (90 s)", st.minutes[tenant])
	}
}

func TestStoredTranscriptIsScrubbed(t *testing.T) {
	st := newFakeStore(job("c2"))
	_, _ = testWorker(st, &countingSource{audio: Audio{Bytes: []byte("a"), ContentType: "audio/mpeg"}}, goodDG(), NewMemoryStore(), &bytes.Buffer{}).RunOnce(context.Background())
	tr := st.transcripts[0]
	if strings.Contains(tr.Utterances[1].Text, "4111") || tr.Scrubbed != 1 || tr.Redaction != "deepgram:pci,ssn+calyx" {
		t.Fatalf("transcript not scrubbed: %+v", tr)
	}
}

func TestArchiveFailureDoesNotBlockTranscription(t *testing.T) {
	st := newFakeStore(job("c3"))
	arc := NewMemoryStore()
	arc.FailPut = errors.New("archive put: AccessDenied")
	_, _ = testWorker(st, &countingSource{audio: Audio{Bytes: []byte("a"), ContentType: "audio/mpeg"}}, goodDG(), arc, &bytes.Buffer{}).RunOnce(context.Background())
	rec := st.recordings[rkey(tenant, "retreaver", "c3")]
	if rec.ArchiveState != "not_archived" || !strings.Contains(rec.ArchiveError, "AccessDenied") {
		t.Fatalf("want not_archived with the reason, got %+v", rec)
	}
	if st.finished["job-c3"][0] != StateDone || len(st.transcripts) != 1 {
		t.Fatal("transcription must go on when the archive fails")
	}
}

func TestArchiveReuploadIsANoop(t *testing.T) {
	arc := NewMemoryStore()
	audio := Audio{Bytes: []byte("same"), ContentType: "audio/mpeg"}
	for _, id := range []string{"c4", "c4b"} {
		j := job(id)
		_, _ = testWorker(newFakeStore(j), &countingSource{audio: audio}, goodDG(), arc, &bytes.Buffer{}).RunOnce(context.Background())
	}
	if arc.Puts != 1 || len(arc.Objects) != 1 {
		t.Fatalf("content-addressed: identical audio is stored once, got %d puts", arc.Puts)
	}
}

func TestDailyCapStopsBeforeAnyFetch(t *testing.T) {
	st := newFakeStore(job("c5"))
	st.minutes[tenant] = 799
	src, dg := &countingSource{audio: Audio{Bytes: []byte("a")}}, goodDG()
	w := testWorker(st, src, dg, NewMemoryStore(), &bytes.Buffer{})
	_, _ = w.RunOnce(context.Background())
	if st.finished["job-c5"][0] != StateCapped || src.calls != 0 || dg.calls != 0 {
		t.Fatalf("cap must stop the job before fetching: %v fetch=%d dg=%d", st.finished["job-c5"], src.calls, dg.calls)
	}
	if next := st.nextTry["job-c5"]; next == nil || next.In(eastern).Hour() != 0 || !next.After(w.now()) {
		t.Fatalf("a capped job waits for the next Eastern midnight, got %v", next)
	}
	// The next job of the same tenant is capped too, without another database read.
	st.jobs = []*Job{job("c6")}
	_, _ = w.RunOnce(context.Background())
	if st.finished["job-c6"][0] != StateCapped {
		t.Fatal("the queue stays stopped for the rest of the day")
	}
}

func TestNoRecordingIsAFinalStateWithReason(t *testing.T) {
	st := newFakeStore(job("c7"))
	src := &countingSource{err: &NoRecordingError{Reason: NoRecNotAnswered, Detail: "call was not answered"}}
	dg := goodDG()
	_, _ = testWorker(st, src, dg, NewMemoryStore(), &bytes.Buffer{}).RunOnce(context.Background())
	if f := st.finished["job-c7"]; f[0] != StateNoRecording || f[1] != NoRecNotAnswered || dg.calls != 0 {
		t.Fatalf("got %v dg=%d", f, dg.calls)
	}
}

func TestTransientFailuresRetryThenFail(t *testing.T) {
	j := job("c8")
	st := newFakeStore(j)
	src := &countingSource{err: &TransientError{Err: errors.New("recording fetch: HTTP 503")}}
	w := testWorker(st, src, goodDG(), NewMemoryStore(), &bytes.Buffer{})
	_, _ = w.RunOnce(context.Background())
	if f := st.finished["job-c8"]; f[0] != StateQueued || st.nextTry["job-c8"] == nil || !st.nextTry["job-c8"].After(w.now()) {
		t.Fatalf("first failure retries later: %v", f)
	}
	j.Attempts = 2 // the third attempt is the last (MaxAttempts 3)
	st.jobs = []*Job{j}
	_, _ = w.RunOnce(context.Background())
	if f := st.finished["job-c8"]; f[0] != StateFailed {
		t.Fatalf("after MaxAttempts the job fails, got %v", f)
	}
}

func TestDeepgramRefusalIsFinal(t *testing.T) {
	st := newFakeStore(job("c9"))
	dg := &fakeDG{err: &FinalError{Err: errors.New("deepgram: HTTP 400: unsupported audio")}}
	_, _ = testWorker(st, &countingSource{audio: Audio{Bytes: []byte("a")}}, dg, NewMemoryStore(), &bytes.Buffer{}).RunOnce(context.Background())
	if f := st.finished["job-c9"]; f[0] != StateFailed {
		t.Fatalf("got %v", f)
	}
	if st.recordings[rkey(tenant, "retreaver", "c9")].ArchiveState != "archived" {
		t.Fatal("the recording is archived even when Deepgram refuses it")
	}
}

func TestArchiveOnlyJobNeverCallsDeepgram(t *testing.T) {
	j := job("c10")
	j.Reason = "archive_only"
	st := newFakeStore(j)
	dg := goodDG()
	_, _ = testWorker(st, &countingSource{audio: Audio{Bytes: []byte("a"), ContentType: "audio/mpeg"}}, dg, NewMemoryStore(), &bytes.Buffer{}).RunOnce(context.Background())
	if dg.calls != 0 || st.finished["job-c10"][0] != StateDone || st.recordings[rkey(tenant, "retreaver", "c10")].ArchiveState != "archived" {
		t.Fatal("archive-only: archived, done, no Deepgram")
	}
}

func TestSpanishCampaignOption(t *testing.T) {
	j := job("c11")
	j.CampaignName = "ACA Spanish"
	dg := goodDG()
	_, _ = testWorker(newFakeStore(j), &countingSource{audio: Audio{Bytes: []byte("a")}}, dg, NewMemoryStore(), &bytes.Buffer{}).RunOnce(context.Background())
	if !dg.opts[0].Spanish {
		t.Fatal("Spanish campaigns must use the Spanish options")
	}
}

func TestTestModeIsRefusedInProduction(t *testing.T) {
	j := job("c12")
	j.SyncMode = "mock"
	st := newFakeStore(j)
	w := testWorker(st, &countingSource{}, goodDG(), NewMemoryStore(), &bytes.Buffer{})
	w.Mock = MockSource{}
	w.Cfg.AllowMock = false
	_, _ = w.RunOnce(context.Background())
	if st.finished["job-c12"][0] != StateFailed {
		t.Fatal("Test-mode jobs must not run when mock is not allowed")
	}
}

func TestNoURLOrCredentialInLogsOrErrors(t *testing.T) {
	secretURL := "https://ringba-recordings.s3.amazonaws.com/SECRETPATH.mp3?X-Amz-Signature=SIGNATURE"
	j := job("c13")
	st := newFakeStore(j)
	logs := &bytes.Buffer{}
	src := &countingSource{err: &TransientError{Err: errors.New("Get \"" + secretURL + "\": dial tcp: i/o timeout")}}
	w := testWorker(st, src, goodDG(), NewMemoryStore(), logs)
	w.Decrypt = func([]byte) (map[string]string, error) {
		return map[string]string{"username": "calyxrecordings", "password": "TOPSECRETPASS", "api_key": "APIKEYSECRET"}, nil
	}
	j.RecordingAccessBlob = []byte("x")
	_, _ = w.RunOnce(context.Background())
	all := logs.String() + strings.Join(st.finished["job-c13"], " ")
	for _, s := range []string{"SECRETPATH", "SIGNATURE", "amazonaws", "TOPSECRETPASS", "APIKEYSECRET"} {
		if strings.Contains(all, s) {
			t.Fatalf("%q leaked into logs or the job error: %s", s, all)
		}
	}
}

func TestCleanupExpiresAudioButKeepsTranscript(t *testing.T) {
	st := newFakeStore(job("c14"))
	arc := NewMemoryStore()
	w := testWorker(st, &countingSource{audio: Audio{Bytes: []byte("old-audio"), ContentType: "audio/mpeg"}}, goodDG(), arc, &bytes.Buffer{})
	w.Now = func() time.Time { return time.Date(2024, 9, 1, 12, 0, 0, 0, time.UTC) } // 25 months ago
	_, _ = w.RunOnce(context.Background())
	key := st.recordings[rkey(tenant, "retreaver", "c14")].ObjectKey
	c := &Cleanup{Store: st, Archive: arc, RetentionMonths: 24, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Now: func() time.Time { return time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC) }}
	n, err := c.RunOnce(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("expired %d, err %v", n, err)
	}
	if _, still := arc.Objects[key]; still || !st.expired["rec-c14"] || len(st.transcripts) != 1 {
		t.Fatal("expiry deletes the audio, marks the row expired and keeps the transcript")
	}
}

func TestRoutesVerifyTheHashAndPack(t *testing.T) {
	st := newFakeStore(job("c15"))
	arc := NewMemoryStore()
	w := testWorker(st, &countingSource{audio: Audio{Bytes: []byte("evidence-audio"), ContentType: "audio/mpeg"}}, goodDG(), arc, &bytes.Buffer{})
	_, _ = w.RunOnce(context.Background())
	rt := &Routes{Store: st, Archive: arc, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	q := "?tenant_id=" + tenant + "&platform=retreaver&external_call_id=c15"

	rec := httptest.NewRecorder()
	rt.Audio(rec, httptest.NewRequest(http.MethodGet, "/internal/recordings/audio"+q, nil))
	if rec.Code != 200 || rec.Body.String() != "evidence-audio" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("audio: %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	rt.Evidence(rec, httptest.NewRequest(http.MethodGet, "/internal/recordings/evidence"+q, nil))
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if rec.Code != 200 || err != nil {
		t.Fatalf("evidence: %d %v", rec.Code, err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
		if f.Modified.Year() < 2020 {
			t.Fatalf("%s has no real timestamp: %v", f.Name, f.Modified)
		}
	}
	for _, n := range []string{"recording.mp3", "transcript.json", "metadata.json", "README.txt"} {
		if !names[n] {
			t.Fatalf("evidence pack misses %s (has %v)", n, names)
		}
	}

	// Tampered object: refused, nothing served.
	key := st.recordings[rkey(tenant, "retreaver", "c15")].ObjectKey
	arc.Objects[key] = []byte("tampered")
	rec = httptest.NewRecorder()
	rt.Audio(rec, httptest.NewRequest(http.MethodGet, "/internal/recordings/audio"+q, nil))
	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "tampered") {
		t.Fatalf("a hash mismatch must be refused, got %d %q", rec.Code, rec.Body.String())
	}

	// Unknown call → 404.
	rec = httptest.NewRecorder()
	rt.Audio(rec, httptest.NewRequest(http.MethodGet, "/internal/recordings/audio?tenant_id="+tenant+"&platform=retreaver&external_call_id=none", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown call: %d", rec.Code)
	}
}
