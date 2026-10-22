package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// v2 Phase 2B — single-call reads for the transcription worker. They are reads only, go through the
// same per-account limiters as the Live sync (ringbaLimiters / retreaverLimiters), and do not touch
// the sync, its cursor or what it publishes.
//
// The recording URL they return is SENSITIVE: Ringba's link opens without auth for days and
// Retreaver's is public and permanent. Callers keep it in memory for one fetch and must never log,
// store, hash, publish or render it. Nothing in this file prints it either.

// RingbaRecordingInfo is the part of one Ringba Call Log row the worker needs.
type RingbaRecordingInfo struct {
	HasConnected bool
	HasRecording bool
	WasBlocked   bool
	URL          string // sensitive, see above
}

// ringbaRecordingWindow is how far around the call's own time the lookup searches.
const ringbaRecordingWindow = 3 * time.Hour

// LookupRecording finds one inboundCallId in a narrow Get Call Log window around `around` (the call's
// time) and returns its recording link. A zero `around` falls back to the last 7 days.
func (a *RingbaAdapter) LookupRecording(ctx context.Context, creds map[string]string, inboundCallID string, around time.Time) (RingbaRecordingInfo, error) {
	inboundCallID = strings.TrimSpace(inboundCallID)
	if inboundCallID == "" {
		return RingbaRecordingInfo{}, fmt.Errorf("ringba: inbound call id required")
	}
	token, accountID, err := ringbaCreds(creds)
	if err != nil {
		return RingbaRecordingInfo{}, err
	}
	start, end := around.Add(-ringbaRecordingWindow), around.Add(ringbaRecordingWindow)
	if around.IsZero() {
		end = time.Now().UTC().Add(2 * time.Hour)
		start = end.Add(-7 * 24 * time.Hour)
	}
	const size = ringbaMaxPageSize
	for offset, page := 0, 0; page < 5; page, offset = page+1, offset+size {
		body, _ := json.Marshal(map[string]interface{}{
			"reportStart": start.UTC().Format(time.RFC3339),
			"reportEnd":   end.UTC().Format(time.RFC3339),
			"filters":     []interface{}{},
			"size":        size,
			"offset":      offset,
			"valueColumns": []map[string]string{
				{"column": "inboundCallId"}, {"column": "recordingUrl"}, {"column": "hasRecording"},
				{"column": "hasConnected"}, {"column": "wasBlocked"},
			},
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			fmt.Sprintf("%s/%s/calllogs", ringbaAPIBase(), accountID), bytes.NewReader(body))
		if err != nil {
			return RingbaRecordingInfo{}, err
		}
		req.Header.Set("Authorization", "Token "+token)
		req.Header.Set("Content-Type", "application/json")
		ringbaLimiters.ForAccount(accountID).Wait()
		resp, respBody, err := a.http.Do(req)
		if err != nil {
			return RingbaRecordingInfo{}, err
		}
		if err := checkHTTPStatus(resp.StatusCode, respBody, resp.Header, "ringba"); err != nil {
			return RingbaRecordingInfo{}, err
		}
		records, err := parseRingbaCallLogResponse(respBody)
		if err != nil {
			return RingbaRecordingInfo{}, err
		}
		for _, raw := range records {
			f := ringbaFieldLookup(raw)
			if strings.TrimSpace(f("inboundCallId")) != inboundCallID {
				continue
			}
			hasRec, _ := parseRingbaBool(f("hasRecording"))
			connected, _ := parseRingbaBool(f("hasConnected"))
			blocked, _ := parseRingbaBool(f("wasBlocked"))
			return RingbaRecordingInfo{
				HasConnected: connected,
				HasRecording: hasRec,
				WasBlocked:   blocked,
				URL:          strings.TrimSpace(f("recordingUrl")),
			}, nil
		}
		if len(records) < size {
			break
		}
	}
	return RingbaRecordingInfo{}, ErrRecordingCallNotFound
}

// ErrRecordingCallNotFound: the call is not in the platform's data (any more).
var ErrRecordingCallNotFound = fmt.Errorf("call not found on the platform")

// RetreaverRecordingInfo is the part of one Retreaver call the worker needs.
type RetreaverRecordingInfo struct {
	Connected bool
	Blocked   bool
	URL       string // sensitive (public, permanent link), see above
}

// RecordingLink reads one call (GET /api/v2/calls/{uuid}.json) and returns its recording_url.
func (c *RetreaverCallClient) RecordingLink(ctx context.Context, creds map[string]string, uuid string) (RetreaverRecordingInfo, error) {
	apiKey, companyID, err := retreaverCreds(creds)
	if err != nil {
		return RetreaverRecordingInfo{}, err
	}
	uuid = strings.TrimSpace(uuid)
	if !IsRetreaverCallUUID(uuid) {
		return RetreaverRecordingInfo{}, fmt.Errorf("retreaver: invalid call uuid")
	}
	status, body, hdr, err := c.do(ctx, http.MethodGet, c.callURL("v2", uuid, apiKey, companyID), companyID, nil)
	if err != nil {
		return RetreaverRecordingInfo{}, err
	}
	if status == http.StatusNotFound {
		return RetreaverRecordingInfo{}, ErrRecordingCallNotFound
	}
	if err := checkHTTPStatus(status, body, hdr, "retreaver"); err != nil {
		return RetreaverRecordingInfo{}, err
	}
	var row struct {
		Status       string          `json:"status"`
		Connected    *bool           `json:"connected"`
		RecordingURL json.RawMessage `json:"recording_url"`
	}
	if err := json.Unmarshal(unwrapRetreaverCallPayload(body), &row); err != nil {
		return RetreaverRecordingInfo{}, fmt.Errorf("retreaver: parse call: %w", err)
	}
	var link string
	_ = json.Unmarshal(row.RecordingURL, &link) // null or missing → ""
	st := strings.ToLower(row.Status)
	return RetreaverRecordingInfo{
		Connected: row.Connected == nil || *row.Connected,
		Blocked:   strings.Contains(st, "block") || strings.Contains(st, "reject"),
		URL:       strings.TrimSpace(link),
	}, nil
}
