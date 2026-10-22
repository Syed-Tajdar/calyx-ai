package transcribe

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

// Internal routes (platform-sync ↔ API on the compose network, bearer WRITEBACK_INTERNAL_TOKEN). The API
// has already checked that the user is an admin and written the recording_access_audit row; the browser
// never sees a bucket URL, signed or not. Every object is checked against its stored SHA-256 and size
// before a single byte is served.
type Routes struct {
	Store   Store
	Archive ObjectStore
	Logger  *slog.Logger
	// Status is what GET /internal/transcription/status reports (yes/no only, never a key).
	Status StatusInfo
}

type StatusInfo struct {
	Enabled           bool    `json:"enabled"`
	DeepgramKeySet    bool    `json:"deepgram_key_set"`
	ArchiveConfigured bool    `json:"archive_configured"`
	DailyCapMinutes   float64 `json:"daily_cap_minutes"`
	RetentionMonths   int     `json:"retention_months"`
}

// StatusHandler: GET /internal/transcription/status
func (rt *Routes) StatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, rt.Status)
}

type recordingRef struct{ tenant, platform, callID string }

func refFrom(r *http.Request) (recordingRef, bool) {
	q := r.URL.Query()
	ref := recordingRef{strings.TrimSpace(q.Get("tenant_id")), strings.TrimSpace(q.Get("platform")), strings.TrimSpace(q.Get("external_call_id"))}
	return ref, ref.tenant != "" && ref.platform != "" && ref.callID != ""
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// load returns the verified audio and its row, or writes the error response.
func (rt *Routes) load(w http.ResponseWriter, r *http.Request) (*RecordingObject, []byte, bool) {
	ref, ok := refFrom(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant_id, platform and external_call_id required"})
		return nil, nil, false
	}
	obj, err := rt.Store.GetRecordingObject(r.Context(), ref.tenant, ref.platform, ref.callID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "lookup failed"})
		return nil, nil, false
	}
	if obj == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no recording", "state": "none"})
		return nil, nil, false
	}
	if obj.ArchiveState != "archived" || obj.ObjectKey == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "recording not in the archive", "state": obj.ArchiveState})
		return nil, nil, false
	}
	if rt.Archive == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "archive not configured"})
		return nil, nil, false
	}
	data, err := rt.Archive.Get(r.Context(), obj.ObjectKey)
	if err != nil {
		rt.Logger.Error("archived recording not readable", "recording_id", obj.ID, "error", safeErr(err))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "archive read failed"})
		return nil, nil, false
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != obj.SHA256 || int64(len(data)) != obj.Bytes {
		rt.Logger.Error("archived recording failed its integrity check", "recording_id", obj.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "integrity check failed"})
		return nil, nil, false
	}
	return obj, data, true
}

// Audio: GET /internal/recordings/audio
func (rt *Routes) Audio(w http.ResponseWriter, r *http.Request) {
	obj, data, ok := rt.load(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Recording-SHA256", obj.SHA256)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// Evidence: GET /internal/recordings/evidence — one zip: the audio, the transcript, the hash and
// metadata file, and a README. (2C adds the QA result with its quotes and timestamps.)
func (rt *Routes) Evidence(w http.ResponseWriter, r *http.Request) {
	obj, data, ok := rt.load(w, r)
	if !ok {
		return
	}
	ref, _ := refFrom(r)
	tr, err := rt.Store.LatestTranscript(r.Context(), ref.tenant, ref.platform, ref.callID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "transcript lookup failed"})
		return
	}
	ext := strings.TrimPrefix(path.Ext(obj.ObjectKey), ".")
	meta := map[string]interface{}{
		"platform":           ref.platform,
		"external_call_id":   ref.callID,
		"recording_sha256":   obj.SHA256,
		"recording_bytes":    obj.Bytes,
		"duration_seconds":   obj.DurationSeconds,
		"content_type":       obj.ContentType,
		"downloaded_at_utc":  obj.DownloadedAt.UTC().Format(time.RFC3339),
		"integrity_verified": true,
		"generated_at_utc":   time.Now().UTC().Format(time.RFC3339),
	}
	if tr != nil {
		meta["transcript_version"] = tr.Version
		meta["transcript_model"] = tr.Model
		meta["deepgram_request_id"] = tr.RequestID
		meta["transcript_recording_sha256"] = tr.RecordingSHA256
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	generated := time.Now().UTC()
	add := func(name string, body []byte) {
		f, _ := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: generated})
		_, _ = f.Write(body)
	}
	add("recording."+ext, data)
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	add("metadata.json", metaJSON)
	if tr != nil {
		trJSON, _ := json.MarshalIndent(tr, "", "  ")
		add("transcript.json", trJSON)
	}
	add("README.txt", []byte(fmt.Sprintf(
		"Calyx dispute evidence pack\n\nrecording.%s  the call audio exactly as fetched from %s once (SHA-256 %s)\n"+
			"transcript.json  the stored transcript; recording_sha256 links it to this audio\n"+
			"metadata.json  size, duration, download time (UTC), Deepgram request id and model\n\n"+
			"Check: sha256sum recording.%s must print %s\n", ext, ref.platform, obj.SHA256, ext, obj.SHA256)))
	if err := zw.Close(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "zip failed"})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="calyx-evidence-%s-%s.zip"`, ref.platform, safeName(ref.callID)))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
