package publisherpause

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
)

const (
	maxRequestBytes = 64 << 10
	requestTimeout  = 90 * time.Second
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// httpRequest is the contract's request body. Unknown fields are rejected.
type httpRequest struct {
	TenantID      string          `json:"tenant_id"`
	IntegrationID string          `json:"integration_id"`
	PublisherID   string          `json:"publisher_id"`
	UserID        *string         `json:"user_id"`
	Mode          string          `json:"mode"`
	RuleID        *string         `json:"rule_id"`
	AlertEventID  *string         `json:"alert_event_id"`
	Numbers       json.RawMessage `json:"numbers"`
}

type errorBody struct {
	Message string  `json:"message"`
	Outcome string  `json:"outcome"`
	AuditID *string `json:"audit_id"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, errorBody{Message: msg, Outcome: OutcomeRejected})
}

// DecodeRequest strictly decodes and validates one request body. Exported for tests.
func DecodeRequest(r io.Reader, action string) (Request, error) {
	dec := json.NewDecoder(io.LimitReader(r, maxRequestBytes))
	dec.DisallowUnknownFields()
	var in httpRequest
	if err := dec.Decode(&in); err != nil {
		return Request{}, errors.New("invalid JSON body")
	}
	if dec.More() {
		return Request{}, errors.New("invalid JSON body: trailing data")
	}
	if _, err := dec.Token(); err != io.EOF {
		return Request{}, errors.New("invalid JSON body: trailing data")
	}
	if !uuidRE.MatchString(in.TenantID) {
		return Request{}, errors.New("tenant_id must be a UUID")
	}
	if !uuidRE.MatchString(in.IntegrationID) {
		return Request{}, errors.New("integration_id must be a UUID")
	}
	if strings.TrimSpace(in.PublisherID) == "" {
		return Request{}, errors.New("publisher_id required")
	}
	opt := func(name string, v *string) (string, error) {
		if v == nil || *v == "" {
			return "", nil
		}
		if !uuidRE.MatchString(*v) {
			return "", errors.New(name + " must be a UUID or null")
		}
		return *v, nil
	}
	userID, err := opt("user_id", in.UserID)
	if err != nil {
		return Request{}, err
	}
	ruleID, err := opt("rule_id", in.RuleID)
	if err != nil {
		return Request{}, err
	}
	alertID, err := opt("alert_event_id", in.AlertEventID)
	if err != nil {
		return Request{}, err
	}
	switch in.Mode {
	case "manual", "auto":
	case "timer":
		if action != ActionUnpause {
			return Request{}, errors.New("mode timer is only valid for unpause")
		}
	default:
		return Request{}, errors.New("mode must be manual, auto or timer")
	}
	numbers := bytes.TrimSpace(in.Numbers)
	if len(numbers) == 0 || string(numbers) == "null" {
		numbers = []byte("{}")
	} else if numbers[0] != '{' {
		return Request{}, errors.New("numbers must be a JSON object")
	}
	return Request{
		TenantID:      strings.ToLower(in.TenantID),
		IntegrationID: strings.ToLower(in.IntegrationID),
		PublisherID:   in.PublisherID,
		UserID:        userID,
		Mode:          in.Mode,
		RuleID:        ruleID,
		AlertEventID:  alertID,
		Numbers:       json.RawMessage(numbers),
	}, nil
}

// Handler serves POST /publisher/pause (action "pause") or /publisher/unpause ("unpause").
// main wraps it in writeback.RequireBearer(WRITEBACK_INTERNAL_TOKEN, …).
func Handler(svc *Service, action string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := DecodeRequest(r.Body, action)
		if err != nil {
			badRequest(w, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		var res Result
		if action == ActionUnpause {
			res, err = svc.Unpause(ctx, req)
		} else {
			res, err = svc.Pause(ctx, req)
		}
		if err != nil {
			var pe *Error
			if !errors.As(err, &pe) {
				pe = &Error{Status: http.StatusInternalServerError, Outcome: OutcomeFailed, Message: redact.SanitizeErr(err)}
			}
			body := errorBody{Message: redact.RedactMessage(pe.Message), Outcome: pe.Outcome}
			if pe.AuditID != "" {
				id := pe.AuditID
				body.AuditID = &id
			}
			if logger != nil {
				logger.Warn("publisher action refused or failed",
					"action", action, "status", pe.Status, "outcome", pe.Outcome,
					"tenant_id", req.TenantID, "integration_id", req.IntegrationID,
					"mode", req.Mode, "message", body.Message)
			}
			writeJSON(w, pe.Status, body)
			return
		}
		if logger != nil {
			logger.Info("publisher action done",
				"action", action, "outcome", res.Outcome, "read_back", res.ReadBack,
				"tenant_id", req.TenantID, "integration_id", req.IntegrationID, "mode", req.Mode)
		}
		writeJSON(w, http.StatusOK, res)
	}
}
