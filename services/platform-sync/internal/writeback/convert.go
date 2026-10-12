package writeback

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"
)

// SettleWindow is the default wait for a platform to confirm a convert or cancel (Ringba;
// Retreaver has its own WRITEBACK_SETTLE_SECONDS_RETREAVER, same 20s default).
// WRITEBACK_SETTLE_SECONDS overrides it. 12s was tight against an observed ~10.4s read-back.
const SettleWindow = 20 * time.Second
const settlePoll = 2 * time.Second

type ConvertRequest struct {
	TenantID      string
	UserID        string
	IntegrationID string
	InboundCallID string
	Amount        float64
	Action        string // convert | void
}

type Result struct {
	Outcome          string  `json:"outcome"`
	InboundCallID    string  `json:"inbound_call_id"`
	CampaignID       string  `json:"campaign_id,omitempty"`
	WebhookName      string  `json:"webhook_name,omitempty"`
	Amount           float64 `json:"amount"`
	HasConverted     bool    `json:"has_converted"`
	ConversionAmount float64 `json:"conversion_amount"`
	AuditID          string  `json:"audit_id,omitempty"`
	Message          string  `json:"message,omitempty"`
	OfflineUploaded  string  `json:"offline_conversion_uploaded"`
	RingbaRef        string  `json:"ringba_response_ref,omitempty"`
	VoidReversible   bool    `json:"void_reversible,omitempty"`
	// Platform is set for every platform except Ringba, whose response shape is unchanged.
	Platform string `json:"platform,omitempty"`
	// ReadBack is the platform's read-back before and after the write (Retreaver). Nil for
	// Ringba, whose read-back is the mapped call above.
	ReadBack *ReadBackSummary `json:"read_back,omitempty"`
}

type RingbaAPI interface {
	LookupCall(ctx context.Context, creds map[string]string, inboundCallID string) (models.RemoteCall, error)
	ListConversionWebhooks(ctx context.Context, creds map[string]string) ([]adapter.RingbaConversionWebhook, error)
	FireInboundConversion(ctx context.Context, hookID, inboundCallID string, amount float64) error
	VoidCall(ctx context.Context, creds map[string]string, inboundCallID string, convAmt, payAmt float64) (string, error)
}

type IntegrationLoader interface {
	GetAPISyncAny(ctx context.Context, tenantID, integrationID string) (*models.SyncIntegration, error)
}

type AuditSink interface {
	InsertWritebackAudit(ctx context.Context, row store.AuditRow) (string, error)
}

type BlobDecryptor interface {
	DecryptMap(blob []byte) (map[string]string, error)
}

type Service struct {
	store     IntegrationLoader
	audit     AuditSink
	decryptor BlobDecryptor
	// writers is the per-platform write-back registry (Stage 4b). NewService registers
	// Ringba; main registers Retreaver. A platform without a writer is refused (403).
	writers      map[string]Writer
	lock         Locker
	settleWindow time.Duration
	settleEvery  time.Duration
	// globalEnabled is WRITEBACK_ENABLED. NewService defaults it on so existing callers
	// keep working; main turns it off when the process flag is false.
	globalEnabled bool
}

func NewService(
	st IntegrationLoader,
	audit AuditSink,
	dec BlobDecryptor,
	ringba RingbaAPI,
	lock Locker,
) *Service {
	s := &Service{store: st, audit: audit, decryptor: dec, lock: lock, globalEnabled: true}
	if ringba != nil {
		s.Register(NewRingbaWriter(ringba))
	}
	return s
}

// SetGlobalEnabled records WRITEBACK_ENABLED. A request that still reaches the service
// while this is false is audited and refused before the lock.
func (s *Service) SetGlobalEnabled(on bool) {
	s.globalEnabled = on
}

// ParseSettleWindow reads WRITEBACK_SETTLE_SECONDS. Empty or invalid values keep the 20s default.
func ParseSettleWindow(raw string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return SettleWindow
	}
	return time.Duration(n) * time.Second
}

func (s *Service) SetSettleWindow(d time.Duration) {
	if d > 0 {
		s.settleWindow = d
	}
}

func ValidateAmount(amount float64) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return ErrInvalidAmount
	}
	return nil
}

func (s *Service) window() time.Duration {
	if s.settleWindow > 0 {
		return s.settleWindow
	}
	return SettleWindow
}

// windowFor is the writer's own settle window when it has one, else the Service window.
func (s *Service) windowFor(w Writer) time.Duration {
	if w != nil {
		if d := w.SettleWindow(); d > 0 {
			return d
		}
	}
	return s.window()
}

func (s *Service) windowForPlatform(platform string) time.Duration {
	w, _ := s.writerFor(platform)
	return s.windowFor(w)
}

func (s *Service) pollEvery() time.Duration {
	if s.settleEvery > 0 {
		return s.settleEvery
	}
	return settlePoll
}

// isConverted matches the API state `converted`: the platform flag is true and revenue is
// greater than zero. A true flag with $0 is `converted_zero`, not a confirmed write-back.
func isConverted(c models.RemoteCall) bool {
	return c.HasConverted && c.Revenue > 0
}

func offlineLabel(c models.RemoteCall) string {
	if !c.OfflineConversionUploadedPresent {
		return "absent"
	}
	if c.OfflineConversionUploaded {
		return "true"
	}
	return "false"
}

func (s *Service) Convert(ctx context.Context, req ConvertRequest) (Result, error) {
	return s.run(ctx, req, "convert")
}

func (s *Service) Void(ctx context.Context, req ConvertRequest) (Result, error) {
	req.Action = "void"
	return s.run(ctx, req, "void")
}

func (s *Service) reject(ctx context.Context, req ConvertRequest, action string, amount float64, err error) (Result, error) {
	id, auditErr := s.writeAudit(ctx, req, action, amount, "rejected", "", "", err.Error(), "absent")
	if auditErr != nil {
		return Result{
			Outcome:       "rejected",
			InboundCallID: req.InboundCallID,
			Amount:        amount,
			Message:       err.Error(),
		}, fmt.Errorf("%w (audit: %v)", err, auditErr)
	}
	return Result{
		Outcome:       "rejected",
		InboundCallID: req.InboundCallID,
		Amount:        amount,
		AuditID:       id,
		Message:       err.Error(),
	}, err
}

func (s *Service) run(ctx context.Context, req ConvertRequest, action string) (res Result, err error) {
	req.InboundCallID = strings.TrimSpace(req.InboundCallID)
	amount := req.Amount
	if !s.globalEnabled {
		return s.reject(ctx, req, action, amount, ErrGloballyDisabled)
	}
	if req.InboundCallID == "" {
		return s.reject(ctx, req, action, amount, fmt.Errorf("inbound_call_id required"))
	}
	if action == "convert" {
		if err := ValidateAmount(req.Amount); err != nil {
			return s.reject(ctx, req, action, amount, err)
		}
	}

	item, err := s.store.GetAPISyncAny(ctx, req.TenantID, req.IntegrationID)
	if err != nil {
		return s.reject(ctx, req, action, amount, ErrIntegrationMissing)
	}
	if item.TenantID != "" && item.TenantID != req.TenantID {
		return s.reject(ctx, req, action, amount, ErrAccountMismatch)
	}
	if !item.WritebackEnabled {
		return s.reject(ctx, req, action, amount, ErrTenantDisabled)
	}
	w, ok := s.writerFor(item.Platform)
	if !ok {
		return s.reject(ctx, req, action, amount, ErrPlatformUnsupported)
	}
	if item.SyncMode != "real" {
		return s.reject(ctx, req, action, amount, ErrNotLive)
	}
	platform := w.Platform()

	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		return s.reject(ctx, req, action, amount, fmt.Errorf("Re-enter credentials"))
	}
	if err := w.CheckCredentials(creds); err != nil {
		return s.reject(ctx, req, action, amount, err)
	}
	if v, ok := w.(RequestValidator); ok {
		if err := v.ValidateRequest(action, req.InboundCallID, req.Amount); err != nil {
			return s.reject(ctx, req, action, amount, err)
		}
	}

	gotLock, err := s.lock.Acquire(ctx, req.TenantID, action, req.InboundCallID)
	if err != nil {
		return s.reject(ctx, req, action, amount, err)
	}
	if !gotLock {
		if action == "void" {
			return s.reject(ctx, req, action, amount, ErrAlreadyVoiding)
		}
		return s.reject(ctx, req, action, amount, ErrAlreadyProcessing)
	}
	release := true
	defer func() {
		if release {
			_ = s.lock.Release(context.Background(), req.TenantID, action, req.InboundCallID)
		}
	}()

	a := &attempt{svc: s, req: req, action: action, platform: platform}
	if platform != "ringba" {
		res.Platform = platform
	}

	before, err := w.LookupCall(ctx, creds, req.InboundCallID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			_, _ = a.audit(ctx, 0, "failed", "", "", err.Error(), "absent", nil, nil)
			return Result{}, forPlatform(ErrCallNotFound, platform)
		}
		if platform != "ringba" {
			// Audit every attempt. Ringba keeps its pre-registry behaviour here (no row).
			_, _ = a.audit(ctx, req.Amount, "failed", "", "", err.Error(), "absent", nil, nil)
		}
		return Result{}, err
	}
	call := before.Call
	// Lookup used this tenant's credentials. Finding the call is the account bind.
	// A call from another account is not returned by LookupCall.
	if call.ExternalCallID != req.InboundCallID {
		_, _ = a.audit(ctx, req.Amount, "failed", call.CampaignID, "", "account mismatch", offlineLabel(call), &before, nil)
		return Result{}, forPlatform(ErrAccountMismatch, platform)
	}

	off := offlineLabel(call)
	res.OfflineUploaded = off
	res.InboundCallID = req.InboundCallID
	res.CampaignID = call.CampaignID
	res.HasConverted = w.IsConverted(before)
	res.ConversionAmount = call.Revenue

	if action == "convert" {
		if w.IsConverted(before) {
			id, _ := a.audit(ctx, req.Amount, "failed", call.CampaignID, "", ErrAlreadyConverted.Error(), off, &before, nil)
			res.AuditID = id
			res.Outcome = "failed"
			res.Message = ErrAlreadyConverted.Error()
			return res, ErrAlreadyConverted
		}
		info, ferr := w.Convert(ctx, creds, req.InboundCallID, req.Amount)
		if bare, pre := asPreWrite(ferr); pre {
			return Result{}, bare
		}
		res.WebhookName = info.WebhookName
		if ferr != nil {
			id, _ := a.audit(ctx, req.Amount, "failed", call.CampaignID, "", ferr.Error(), off, &before, nil)
			res.AuditID = id
			res.Outcome = "failed"
			res.Message = ferr.Error()
			return res, fmt.Errorf("%w: %v", forPlatform(ErrFireRejected, platform), ferr)
		}
		after, settled, secs := s.poll(ctx, w, creds, req.InboundCallID, func(c CallState) bool { return w.ConvertSettled(c, req.Amount) })
		res.HasConverted = w.IsConverted(after)
		res.ConversionAmount = after.Call.Revenue
		res.CampaignID = after.Call.CampaignID
		res.ReadBack = summarize(&before, &after, secs)
		if settled {
			res.Outcome = "confirmed"
			res.Amount = req.Amount
			res.Message = "confirmed"
			id, _ := a.audit(ctx, req.Amount, "confirmed", after.Call.CampaignID, "", "confirmed", off, &before, res.ReadBack)
			res.AuditID = id
			return res, nil
		}
		res.Amount = req.Amount
		if w.FailOnUnsettled() {
			return a.mismatch(ctx, res, req.Amount, after, off, &before, fmt.Sprintf("expected revenue=%.2f converted=true", req.Amount))
		}
		res.Outcome = "pending"
		res.Message = fmt.Sprintf("fire accepted; not settled within %s", s.windowFor(w))
		id, _ := a.audit(ctx, req.Amount, "pending", after.Call.CampaignID, "", res.Message, off, &before, res.ReadBack)
		res.AuditID = id
		_ = s.lock.MarkPending(ctx, req.TenantID, action, req.InboundCallID)
		release = false
		return res, nil
	}

	// void
	if !w.IsConverted(before) {
		id, _ := a.audit(ctx, call.Revenue, "failed", call.CampaignID, "", ErrNotConverted.Error(), off, &before, nil)
		res.AuditID = id
		res.Outcome = "failed"
		res.Message = ErrNotConverted.Error()
		res.Amount = call.Revenue
		return res, ErrNotConverted
	}
	voidAmt := call.Revenue
	info, verr := w.Void(ctx, creds, req.InboundCallID, before)
	if bare, pre := asPreWrite(verr); pre {
		return Result{}, bare
	}
	if verr != nil {
		id, _ := a.audit(ctx, voidAmt, "failed", call.CampaignID, "", verr.Error(), off, &before, nil)
		res.AuditID = id
		res.Outcome = "failed"
		res.Message = verr.Error()
		res.Amount = voidAmt
		return res, fmt.Errorf("%w: %v", forPlatform(ErrFireRejected, platform), verr)
	}
	ref := info.Ref
	res.RingbaRef = ref
	after, settled, secs := s.poll(ctx, w, creds, req.InboundCallID, w.VoidSettled)
	res.HasConverted = w.IsConverted(after)
	res.ConversionAmount = after.Call.Revenue
	res.VoidReversible = true
	res.Amount = voidAmt
	res.ReadBack = summarize(&before, &after, secs)
	if settled {
		res.Outcome = "confirmed"
		res.Message = "void confirmed; reversible via a later convert"
		id, _ := a.audit(ctx, voidAmt, "confirmed", after.Call.CampaignID, ref, res.Message, off, &before, res.ReadBack)
		res.AuditID = id
		return res, nil
	}
	if w.FailOnUnsettled() {
		return a.mismatch(ctx, res, voidAmt, after, off, &before, "expected revenue=0 or null")
	}
	res.Outcome = "pending"
	res.Message = fmt.Sprintf("void accepted; not settled within %s", s.windowFor(w))
	id, _ := a.audit(ctx, voidAmt, "pending", after.Call.CampaignID, ref, res.Message, off, &before, res.ReadBack)
	res.AuditID = id
	_ = s.lock.MarkPending(ctx, req.TenantID, action, req.InboundCallID)
	release = false
	return res, nil
}

// poll re-reads the call until settled or the platform's window ends. It returns the last
// read-back, whether it settled, and the seconds from the first poll to the last read.
func (s *Service) poll(ctx context.Context, w Writer, creds map[string]string, callID string, settledFn func(CallState) bool) (CallState, bool, float64) {
	start := time.Now()
	deadline := start.Add(s.windowFor(w))
	var last CallState
	for {
		c, err := w.ReadBack(ctx, creds, callID)
		if err == nil {
			last = c
			if settledFn(c) {
				return c, true, time.Since(start).Seconds()
			}
		}
		if time.Now().After(deadline) {
			return last, false, time.Since(start).Seconds()
		}
		select {
		case <-ctx.Done():
			return last, false, time.Since(start).Seconds()
		case <-time.After(s.pollEvery()):
		}
	}
}

// attempt carries what every audit row of one request shares.
type attempt struct {
	svc      *Service
	req      ConvertRequest
	action   string
	platform string
}

func (a *attempt) audit(ctx context.Context, amount float64, outcome, campaign, ref, msg, offline string, before *CallState, rb *ReadBackSummary) (string, error) {
	if rb == nil && before != nil && before.ReadBack != nil {
		rb = summarize(before, nil, 0)
	}
	return a.svc.writeAuditRow(ctx, a.req, a.action, amount, outcome, campaign, ref, msg, offline, a.platform, rb)
}

// mismatch audits a read-back that disagrees with what was written as a failure.
func (a *attempt) mismatch(ctx context.Context, res Result, amount float64, after CallState, off string, before *CallState, expected string) (Result, error) {
	msg := fmt.Sprintf("read-back mismatch after %s: %s, got %s", a.svc.windowForPlatform(a.platform), expected, describeReadBack(after))
	res.Outcome = "failed"
	res.Message = msg
	id, _ := a.audit(ctx, amount, "failed", after.Call.CampaignID, "", msg, off, before, res.ReadBack)
	res.AuditID = id
	return res, fmt.Errorf("%w: %s", ErrReadBackMismatch, msg)
}

func (s *Service) writeAudit(ctx context.Context, req ConvertRequest, action string, amount float64, outcome, campaign, ref, msg, offline string) (string, error) {
	return s.writeAuditRow(ctx, req, action, amount, outcome, campaign, ref, msg, offline, "", nil)
}

// writeAuditRow inserts one immutable audit row. The message passes through the redaction
// helper: a platform error can echo a request URL, and Retreaver's key travels in the URL.
func (s *Service) writeAuditRow(ctx context.Context, req ConvertRequest, action string, amount float64, outcome, campaign, ref, msg, offline, platform string, rb *ReadBackSummary) (string, error) {
	if s.audit == nil {
		return "", nil
	}
	row := store.AuditRow{
		TenantID:        req.TenantID,
		UserID:          req.UserID,
		IntegrationID:   req.IntegrationID,
		InboundCallID:   req.InboundCallID,
		Action:          action,
		Amount:          amount,
		Outcome:         outcome,
		RingbaRef:       ref,
		CampaignID:      campaign,
		Message:         redact.RedactMessage(msg),
		OfflineUploaded: offline,
	}
	if platform != "" && platform != "ringba" {
		row.Platform = platform
	}
	if rb != nil {
		if b, err := json.Marshal(rb); err == nil {
			row.ReadBack = redact.RedactMessage(string(b))
		}
	}
	return s.audit.InsertWritebackAudit(ctx, row)
}

func pickConversionWebhook(hooks []adapter.RingbaConversionWebhook) (adapter.RingbaConversionWebhook, error) {
	if len(hooks) == 0 {
		return adapter.RingbaConversionWebhook{}, fmt.Errorf("writeback: no enabled conversion webhook on account")
	}
	for _, h := range hooks {
		if strings.EqualFold(h.Name, "My Webhook") {
			return h, nil
		}
	}
	return hooks[0], nil
}
