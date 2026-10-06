package writeback

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"
)

// SettleWindow is the default wait for Ringba to confirm a convert or cancel.
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
	store        IntegrationLoader
	audit        AuditSink
	decryptor    BlobDecryptor
	ringba       RingbaAPI
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
	return &Service{store: st, audit: audit, decryptor: dec, ringba: ringba, lock: lock, globalEnabled: true}
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
	if !strings.EqualFold(item.Platform, "ringba") {
		return s.reject(ctx, req, action, amount, ErrNotRingba)
	}
	if item.SyncMode != "real" {
		return s.reject(ctx, req, action, amount, ErrNotLive)
	}

	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		return s.reject(ctx, req, action, amount, fmt.Errorf("decrypt credentials: %w", err))
	}
	accountID := strings.TrimSpace(creds["account_id"])
	if accountID == "" {
		return s.reject(ctx, req, action, amount, fmt.Errorf("writeback: Ringba account_id missing on this tenant's credentials"))
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

	call, err := s.ringba.LookupCall(ctx, creds, req.InboundCallID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			_, _ = s.writeAudit(ctx, req, action, 0, "failed", "", "", err.Error(), "absent")
			return Result{}, ErrCallNotFound
		}
		return Result{}, err
	}
	// Lookup used this tenant's account_id. Finding the call is the account bind.
	// A call from another Ringba account is not returned by LookupCall.
	if call.ExternalCallID != req.InboundCallID {
		_, _ = s.writeAudit(ctx, req, action, req.Amount, "failed", call.CampaignID, "", "account mismatch", offlineLabel(call))
		return Result{}, ErrAccountMismatch
	}

	off := offlineLabel(call)
	res.OfflineUploaded = off
	res.InboundCallID = req.InboundCallID
	res.CampaignID = call.CampaignID
	res.HasConverted = isConverted(call)
	res.ConversionAmount = call.Revenue

	if action == "convert" {
		if isConverted(call) {
			id, _ := s.writeAudit(ctx, req, action, req.Amount, "failed", call.CampaignID, "", ErrAlreadyConverted.Error(), off)
			res.AuditID = id
			res.Outcome = "failed"
			res.Message = ErrAlreadyConverted.Error()
			return res, ErrAlreadyConverted
		}
		hooks, herr := s.ringba.ListConversionWebhooks(ctx, creds)
		if herr != nil {
			return Result{}, herr
		}
		hook, herr := pickConversionWebhook(hooks)
		if herr != nil {
			return Result{}, herr
		}
		res.WebhookName = hook.Name
		if ferr := s.ringba.FireInboundConversion(ctx, hook.ID, req.InboundCallID, req.Amount); ferr != nil {
			id, _ := s.writeAudit(ctx, req, action, req.Amount, "failed", call.CampaignID, "", ferr.Error(), off)
			res.AuditID = id
			res.Outcome = "failed"
			res.Message = ferr.Error()
			return res, fmt.Errorf("%w: %v", ErrFireRejected, ferr)
		}
		after, settled := s.pollConvert(ctx, creds, req.InboundCallID, req.Amount)
		res.HasConverted = isConverted(after)
		res.ConversionAmount = after.Revenue
		res.CampaignID = after.CampaignID
		if settled {
			res.Outcome = "confirmed"
			res.Amount = req.Amount
			res.Message = "confirmed"
			id, _ := s.writeAudit(ctx, req, action, req.Amount, "confirmed", after.CampaignID, "", "confirmed", off)
			res.AuditID = id
			return res, nil
		}
		res.Outcome = "pending"
		res.Amount = req.Amount
		res.Message = fmt.Sprintf("fire accepted; not settled within %s", s.window())
		id, _ := s.writeAudit(ctx, req, action, req.Amount, "pending", after.CampaignID, "", res.Message, off)
		res.AuditID = id
		_ = s.lock.MarkPending(ctx, req.TenantID, action, req.InboundCallID)
		release = false
		return res, nil
	}

	// void
	if !isConverted(call) {
		id, _ := s.writeAudit(ctx, req, action, call.Revenue, "failed", call.CampaignID, "", ErrNotConverted.Error(), off)
		res.AuditID = id
		res.Outcome = "failed"
		res.Message = ErrNotConverted.Error()
		res.Amount = call.Revenue
		return res, ErrNotConverted
	}
	voidAmt := call.Revenue
	ref, verr := s.ringba.VoidCall(ctx, creds, req.InboundCallID, call.Revenue, call.Payout)
	if verr != nil {
		id, _ := s.writeAudit(ctx, req, action, voidAmt, "failed", call.CampaignID, "", verr.Error(), off)
		res.AuditID = id
		res.Outcome = "failed"
		res.Message = verr.Error()
		res.Amount = voidAmt
		return res, fmt.Errorf("%w: %v", ErrFireRejected, verr)
	}
	res.RingbaRef = ref
	after, settled := s.pollVoid(ctx, creds, req.InboundCallID)
	res.HasConverted = isConverted(after)
	res.ConversionAmount = after.Revenue
	res.VoidReversible = true
	res.Amount = voidAmt
	if settled {
		res.Outcome = "confirmed"
		res.Message = "void confirmed; reversible via a later convert"
		id, _ := s.writeAudit(ctx, req, action, voidAmt, "confirmed", after.CampaignID, ref, res.Message, off)
		res.AuditID = id
		return res, nil
	}
	res.Outcome = "pending"
	res.Message = fmt.Sprintf("void accepted; not settled within %s", s.window())
	id, _ := s.writeAudit(ctx, req, action, voidAmt, "pending", after.CampaignID, ref, res.Message, off)
	res.AuditID = id
	_ = s.lock.MarkPending(ctx, req.TenantID, action, req.InboundCallID)
	release = false
	return res, nil
}

func (s *Service) pollConvert(ctx context.Context, creds map[string]string, callID string, amount float64) (models.RemoteCall, bool) {
	deadline := time.Now().Add(s.window())
	var last models.RemoteCall
	for {
		c, err := s.ringba.LookupCall(ctx, creds, callID)
		if err == nil {
			last = c
			if c.HasConverted && c.Revenue > 0 && math.Abs(c.Revenue-amount) < 0.011 {
				return c, true
			}
			if c.HasConverted && c.Revenue > 0 {
				return c, true
			}
		}
		if time.Now().After(deadline) {
			return last, false
		}
		select {
		case <-ctx.Done():
			return last, false
		case <-time.After(s.pollEvery()):
		}
	}
}

func (s *Service) pollVoid(ctx context.Context, creds map[string]string, callID string) (models.RemoteCall, bool) {
	deadline := time.Now().Add(s.window())
	var last models.RemoteCall
	for {
		c, err := s.ringba.LookupCall(ctx, creds, callID)
		if err == nil {
			last = c
			if !isConverted(c) {
				return c, true
			}
		}
		if time.Now().After(deadline) {
			return last, false
		}
		select {
		case <-ctx.Done():
			return last, false
		case <-time.After(s.pollEvery()):
		}
	}
}

func (s *Service) writeAudit(ctx context.Context, req ConvertRequest, action string, amount float64, outcome, campaign, ref, msg, offline string) (string, error) {
	if s.audit == nil {
		return "", nil
	}
	return s.audit.InsertWritebackAudit(ctx, store.AuditRow{
		TenantID:        req.TenantID,
		UserID:          req.UserID,
		IntegrationID:   req.IntegrationID,
		InboundCallID:   req.InboundCallID,
		Action:          action,
		Amount:          amount,
		Outcome:         outcome,
		RingbaRef:       ref,
		CampaignID:      campaign,
		Message:         msg,
		OfflineUploaded: offline,
	})
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
