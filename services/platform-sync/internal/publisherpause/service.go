// Package publisherpause implements POST /publisher/pause and /publisher/unpause (v2 Phase 2D,
// docs/v2-2d-contract.md). It is the only v2 code that writes to Ringba or Retreaver, and it
// writes exactly one thing: a publisher's paused/enabled flag.
//
// Every attempt — including every refusal — writes one publisher_action_audit row through
// insert_publisher_action_audit(). Messages and request_redacted are redacted; no key, token,
// account id or URL query is ever stored, logged or returned.
package publisherpause

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"
)

const (
	ActionPause   = "pause"
	ActionUnpause = "unpause"

	OutcomeConfirmed           = "confirmed"
	OutcomeConfirmedByResponse = "confirmed_by_response"
	OutcomeFailed              = "failed"
	OutcomeRejected            = "rejected"

	ReadBackOK       = "ok"
	ReadBackNone     = "none"
	ReadBackMismatch = "mismatch"
)

// Request is one pause / unpause request (already strictly decoded by the handler).
type Request struct {
	TenantID      string
	IntegrationID string
	PublisherID   string
	UserID        string // "" = auto / timer
	Mode          string // manual | auto | timer
	RuleID        string
	AlertEventID  string
	Numbers       json.RawMessage // a JSON object or empty
}

// Result is the 200 body.
type Result struct {
	Outcome  string `json:"outcome"`
	Paused   bool   `json:"paused"`
	ReadBack string `json:"read_back"`
	Message  string `json:"message"`
	AuditID  string `json:"audit_id"`
}

// Error is every non-200 outcome. Status is the HTTP status from the contract.
type Error struct {
	Status  int
	Outcome string // rejected | failed
	Message string
	AuditID string // "" when the audit insert failed
}

func (e *Error) Error() string { return e.Message }

// IntegrationLoader reads the integration via get_publisher_pause_integration().
type IntegrationLoader interface {
	GetPublisherPauseIntegration(ctx context.Context, tenantID, integrationID string) (*store.PauseIntegration, error)
}

// AuditSink writes via insert_publisher_action_audit().
type AuditSink interface {
	InsertPublisherActionAudit(ctx context.Context, row store.PublisherAuditRow) (string, error)
}

// BlobDecryptor decrypts api_credentials_blob (crypto.CredentialDecryptor).
type BlobDecryptor interface {
	DecryptMap(blob []byte) (map[string]string, error)
}

// RingbaAPI is the Ringba publisher transport (adapter.RingbaPublisherClient).
type RingbaAPI interface {
	GetPublisher(ctx context.Context, creds map[string]string, publisherID string) (adapter.RingbaPublisher, error)
	SetPublisherEnabled(ctx context.Context, creds map[string]string, publisherID string, enabled bool) error
}

// RetreaverAPI is the Retreaver affiliate transport (adapter.RetreaverAffiliateClient).
type RetreaverAPI interface {
	ResolveAffiliate(ctx context.Context, creds map[string]string, afid string) (adapter.RetreaverAffiliate, []string, error)
	GetAffiliate(ctx context.Context, creds map[string]string, id string) (adapter.RetreaverAffiliate, error)
	SetPaused(ctx context.Context, creds map[string]string, id string, paused bool) (adapter.RetreaverAffiliate, error)
}

// Config holds the process switches. Both default off.
type Config struct {
	ActionsEnabled      bool // WORKFLOW_ACTIONS_ENABLED
	RingbaPauseVerified bool // WORKFLOW_RINGBA_PAUSE_VERIFIED
}

type Service struct {
	cfg       Config
	store     IntegrationLoader
	audit     AuditSink
	decryptor BlobDecryptor
	lock      Locker
	ringba    RingbaAPI
	retreaver RetreaverAPI

	// Ringba read-back: re-read up to readBackAttempts times, readBackEvery apart, while it does
	// not yet show the written value. Still wrong after the last read → 502 mismatch.
	readBackAttempts int
	readBackEvery    time.Duration
}

func NewService(cfg Config, st IntegrationLoader, audit AuditSink, dec BlobDecryptor, lock Locker, ringba RingbaAPI, retreaver RetreaverAPI) *Service {
	return &Service{
		cfg: cfg, store: st, audit: audit, decryptor: dec, lock: lock, ringba: ringba, retreaver: retreaver,
		readBackAttempts: 3,
		readBackEvery:    time.Second,
	}
}

// SetReadBackPolicy overrides the Ringba read-back retries (tests).
func (s *Service) SetReadBackPolicy(attempts int, every time.Duration) {
	if attempts >= 1 {
		s.readBackAttempts = attempts
	}
	if every >= 0 {
		s.readBackEvery = every
	}
}

func (s *Service) Pause(ctx context.Context, req Request) (Result, error) {
	return s.run(ctx, req, ActionPause)
}

func (s *Service) Unpause(ctx context.Context, req Request) (Result, error) {
	return s.run(ctx, req, ActionUnpause)
}

// attempt carries what every audit row of one request shares.
type attempt struct {
	svc      *Service
	req      Request
	action   string
	platform string
	intID    string // "" when the integration was not found (FK)
	creds    map[string]string
	steps    []string
}

func (a *attempt) step(s string) { a.steps = append(a.steps, s) }

// clean redacts a message: the shared helper first, then any literal credential value.
func (a *attempt) clean(msg string) string {
	out := redact.RedactMessage(msg)
	for _, v := range a.creds {
		v = strings.TrimSpace(v)
		if len(v) < 4 {
			continue
		}
		out = strings.ReplaceAll(out, v, "[REDACTED]")
	}
	return out
}

func (a *attempt) audit(ctx context.Context, outcome, readBack, msg string) string {
	if a.svc.audit == nil {
		return ""
	}
	platform := a.platform
	if platform == "" {
		platform = "unknown"
	}
	row := store.PublisherAuditRow{
		TenantID:        a.req.TenantID,
		UserID:          a.req.UserID,
		IntegrationID:   a.intID,
		Platform:        platform,
		PublisherID:     auditPublisherID(a.req.PublisherID),
		Action:          a.action,
		Mode:            a.req.Mode,
		RuleID:          a.req.RuleID,
		AlertEventID:    a.req.AlertEventID,
		Outcome:         outcome,
		ReadBack:        readBack,
		Message:         a.clean(msg),
		RequestRedacted: a.clean(strings.Join(a.steps, " ; ")),
		Numbers:         string(a.req.Numbers),
	}
	id, err := a.svc.audit.InsertPublisherActionAudit(ctx, row)
	if err != nil {
		return ""
	}
	return id
}

// auditPublisherID keeps a refused, malformed id from carrying arbitrary text into the audit row.
func auditPublisherID(id string) string {
	id = strings.TrimSpace(id)
	if adapter.ValidPlatformID(id) {
		return id
	}
	if id == "" {
		return "(empty)"
	}
	return "(invalid)"
}

func (a *attempt) fail(ctx context.Context, status int, outcome, readBack, msg string) (Result, error) {
	msg = a.clean(msg)
	id := a.audit(ctx, outcome, readBack, msg)
	return Result{}, &Error{Status: status, Outcome: outcome, Message: msg, AuditID: id}
}

func (a *attempt) reject(ctx context.Context, status int, msg string) (Result, error) {
	return a.fail(ctx, status, OutcomeRejected, ReadBackNone, msg)
}

func (a *attempt) ok(ctx context.Context, outcome, readBack, msg string) (Result, error) {
	msg = a.clean(msg)
	id := a.audit(ctx, outcome, readBack, msg)
	return Result{
		Outcome:  outcome,
		Paused:   a.action == ActionPause,
		ReadBack: readBack,
		Message:  msg,
		AuditID:  id,
	}, nil
}

func (s *Service) run(ctx context.Context, req Request, action string) (Result, error) {
	req.PublisherID = strings.TrimSpace(req.PublisherID)
	a := &attempt{svc: s, req: req, action: action}

	it, err := s.store.GetPublisherPauseIntegration(ctx, req.TenantID, req.IntegrationID)
	if err != nil || it == nil {
		return a.reject(ctx, http.StatusForbidden, "integration not found, not API sync, or has no credentials")
	}
	a.intID = it.ID
	a.platform = strings.ToLower(strings.TrimSpace(it.Platform))
	if it.TenantID != "" && it.TenantID != req.TenantID {
		a.intID = ""
		return a.reject(ctx, http.StatusForbidden, "integration does not belong to this tenant")
	}

	if !s.cfg.ActionsEnabled {
		return a.reject(ctx, http.StatusForbidden, "publisher actions are disabled (WORKFLOW_ACTIONS_ENABLED is off)")
	}
	if a.platform != "ringba" && a.platform != "retreaver" {
		return a.reject(ctx, http.StatusForbidden, "publisher pause is not supported for this platform")
	}
	if it.SyncMode != "real" {
		return a.reject(ctx, http.StatusForbidden, "integration is not Live (Test integrations are never paused through)")
	}
	if !it.IsActive {
		return a.reject(ctx, http.StatusForbidden, "integration is paused (inactive)")
	}
	if !it.AllowPublisherPause {
		return a.reject(ctx, http.StatusForbidden, "\"Allow Calyx to pause publishers\" is off for this integration")
	}
	if a.platform == "ringba" && !s.cfg.RingbaPauseVerified {
		return a.reject(ctx, http.StatusForbidden, "Ringba publisher pause is not verified yet (WORKFLOW_RINGBA_PAUSE_VERIFIED is off); pause it in Ringba")
	}
	if !adapter.ValidPlatformID(req.PublisherID) {
		return a.reject(ctx, http.StatusBadRequest, "publisher_id is not a valid platform id")
	}

	creds, err := s.decryptor.DecryptMap(it.APICredentialsBlob)
	if err != nil {
		return a.reject(ctx, http.StatusForbidden, "stored credentials cannot be read; re-enter credentials")
	}
	a.creds = creds
	if a.platform == "ringba" && s.ringba == nil || a.platform == "retreaver" && s.retreaver == nil {
		return a.reject(ctx, http.StatusForbidden, "publisher pause is not supported for this platform")
	}

	got, err := s.lock.Acquire(ctx, req.TenantID, a.platform, req.PublisherID)
	if err != nil {
		return a.reject(ctx, http.StatusServiceUnavailable, "lock unavailable: "+err.Error())
	}
	if !got {
		return a.reject(ctx, http.StatusConflict, "another pause/unpause for this publisher is in progress")
	}
	defer func() { _ = s.lock.Release(context.Background(), req.TenantID, a.platform, req.PublisherID) }()

	if a.platform == "ringba" {
		return s.runRingba(ctx, a)
	}
	return s.runRetreaver(ctx, a)
}

func platformErrStatus(err error) int {
	if errors.Is(err, adapter.ErrPublisherNotFound) {
		return http.StatusNotFound
	}
	return http.StatusBadGateway
}

func (s *Service) runRingba(ctx context.Context, a *attempt) (Result, error) {
	pub := a.req.PublisherID
	path := adapter.RingbaPublisherPath(pub)
	target := a.action == ActionUnpause // enabled=true means not paused
	verb := "paused"
	if target {
		verb = "unpaused"
	}

	a.step("GET " + path)
	before, err := s.ringba.GetPublisher(ctx, a.creds, pub)
	if err != nil {
		return a.fail(ctx, platformErrStatus(err), OutcomeFailed, ReadBackNone, "read before write failed: "+err.Error())
	}
	if before.Enabled == nil {
		return a.fail(ctx, http.StatusBadGateway, OutcomeFailed, ReadBackNone, "Ringba publisher response has no \"enabled\" field; nothing was written")
	}
	if *before.Enabled == target {
		return a.ok(ctx, OutcomeConfirmed, ReadBackOK, "already "+verb+" on Ringba; nothing was written")
	}

	a.step(fmt.Sprintf(`PATCH %s {"enabled":%v}`, path, target))
	if err := s.ringba.SetPublisherEnabled(ctx, a.creds, pub, target); err != nil {
		return a.fail(ctx, platformErrStatus(err), OutcomeFailed, ReadBackNone, "write failed: "+err.Error())
	}

	var after adapter.RingbaPublisher
	var readErr error
	for i := 1; i <= s.readBackAttempts; i++ {
		if i > 1 && s.readBackEvery > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(s.readBackEvery):
			}
		}
		a.step("GET " + path)
		after, readErr = s.ringba.GetPublisher(ctx, a.creds, pub)
		if readErr == nil && after.Enabled != nil && *after.Enabled == target {
			return a.ok(ctx, OutcomeConfirmed, ReadBackOK, verb+" on Ringba; read-back confirms enabled="+fmt.Sprint(target))
		}
		if ctx.Err() != nil {
			break
		}
	}
	if readErr != nil {
		return a.fail(ctx, http.StatusBadGateway, OutcomeFailed, ReadBackNone, "write accepted but read-back failed: "+readErr.Error())
	}
	got := "absent"
	if after.Enabled != nil {
		got = fmt.Sprint(*after.Enabled)
	}
	return a.fail(ctx, http.StatusBadGateway, OutcomeFailed, ReadBackMismatch,
		fmt.Sprintf("read-back contradicts the write: expected enabled=%v, got enabled=%s", target, got))
}

func (s *Service) runRetreaver(ctx context.Context, a *attempt) (Result, error) {
	target := a.action == ActionPause
	verb := "paused"
	if !target {
		verb = "unpaused"
	}

	aff, steps, err := s.retreaver.ResolveAffiliate(ctx, a.creds, a.req.PublisherID)
	a.steps = append(a.steps, steps...)
	if err != nil {
		return a.fail(ctx, platformErrStatus(err), OutcomeFailed, ReadBackNone, "affiliate lookup failed: "+err.Error())
	}
	if aff.Paused != nil && *aff.Paused == target {
		return a.ok(ctx, OutcomeConfirmed, ReadBackOK, "already "+verb+" on Retreaver; nothing was written")
	}

	path := adapter.RetreaverAffiliatePath(aff.ID)
	a.step(fmt.Sprintf(`PUT %s {"affiliate":{"paused":%v}}`, path, target))
	if _, err := s.retreaver.SetPaused(ctx, a.creds, aff.ID, target); err != nil {
		return a.fail(ctx, platformErrStatus(err), OutcomeFailed, ReadBackNone, "write failed: "+err.Error())
	}

	a.step("GET " + path)
	after, err := s.retreaver.GetAffiliate(ctx, a.creds, aff.ID)
	if err != nil {
		// The PUT itself returned 2xx; the documented GET has no "paused" anyway (owner Q10).
		return a.ok(ctx, OutcomeConfirmedByResponse, ReadBackNone,
			verb+" on Retreaver (confirmed by the write response); read-back unavailable: "+err.Error())
	}
	if after.Paused == nil {
		return a.ok(ctx, OutcomeConfirmedByResponse, ReadBackNone,
			verb+" on Retreaver (confirmed by the write response; the affiliate GET has no \"paused\" field)")
	}
	if *after.Paused == target {
		return a.ok(ctx, OutcomeConfirmed, ReadBackOK, verb+" on Retreaver; read-back confirms paused="+fmt.Sprint(target))
	}
	return a.fail(ctx, http.StatusBadGateway, OutcomeFailed, ReadBackMismatch,
		fmt.Sprintf("read-back contradicts the write: expected paused=%v, got paused=%v", target, *after.Paused))
}
