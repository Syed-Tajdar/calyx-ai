package writeback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"
)

type fakeDec struct{}

func (fakeDec) DecryptMap([]byte) (map[string]string, error) {
	return map[string]string{"api_token": "t", "account_id": "RA1"}, nil
}

type fakeStore struct {
	item *models.SyncIntegration
	err  error
}

func (f *fakeStore) GetAPISyncAny(context.Context, string, string) (*models.SyncIntegration, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.item, nil
}

type memAudit struct {
	mu   sync.Mutex
	rows []store.AuditRow
}

func (m *memAudit) InsertWritebackAudit(_ context.Context, row store.AuditRow) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("aud-%d", len(m.rows)+1)
	m.rows = append(m.rows, row)
	return id, nil
}

type fakeRingba struct {
	mu        sync.Mutex
	call      models.RemoteCall
	lookupErr error
	fireErr   error
	voidErr   error
	fires     int32
	voids     int32
	afterFire models.RemoteCall
	afterVoid models.RemoteCall
	slowFire  time.Duration
}

func (f *fakeRingba) LookupCall(context.Context, map[string]string, string) (models.RemoteCall, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lookupErr != nil {
		return models.RemoteCall{}, f.lookupErr
	}
	return f.call, nil
}

func (f *fakeRingba) ListConversionWebhooks(context.Context, map[string]string) ([]adapter.RingbaConversionWebhook, error) {
	return []adapter.RingbaConversionWebhook{{ID: "WH1", Name: "My Webhook"}}, nil
}

func (f *fakeRingba) FireInboundConversion(context.Context, string, string, float64) error {
	atomic.AddInt32(&f.fires, 1)
	if f.slowFire > 0 {
		time.Sleep(f.slowFire)
	}
	if f.fireErr != nil {
		return f.fireErr
	}
	f.mu.Lock()
	if f.afterFire.ExternalCallID != "" {
		f.call = f.afterFire
	}
	f.mu.Unlock()
	return nil
}

func (f *fakeRingba) VoidCall(context.Context, map[string]string, string, float64, float64) (string, error) {
	atomic.AddInt32(&f.voids, 1)
	if f.voidErr != nil {
		return "", f.voidErr
	}
	f.mu.Lock()
	if f.afterVoid.ExternalCallID != "" {
		f.call = f.afterVoid
	}
	f.mu.Unlock()
	return "TR-void", nil
}

func enabledItem() *models.SyncIntegration {
	return &models.SyncIntegration{
		ID: "int-1", TenantID: "ten-a", Platform: "ringba",
		SyncMode: "real", WritebackEnabled: true, APICredentialsBlob: []byte("x"),
	}
}

func testSvc(st IntegrationLoader, rb RingbaAPI, lock Locker, audit AuditSink) *Service {
	s := NewService(st, audit, fakeDec{}, rb, lock)
	s.settleWindow = 40 * time.Millisecond
	s.settleEvery = 5 * time.Millisecond
	return s
}

func baseReq() ConvertRequest {
	return ConvertRequest{
		TenantID: "ten-a", UserID: "user-1", IntegrationID: "int-1",
		InboundCallID: "RGBTEST01", Amount: 1.00,
	}
}

func TestValidateAmount(t *testing.T) {
	if err := ValidateAmount(0); !errors.Is(err, ErrInvalidAmount) {
		t.Fatal(err)
	}
	if err := ValidateAmount(-1); !errors.Is(err, ErrInvalidAmount) {
		t.Fatal(err)
	}
	if err := ValidateAmount(250); err != nil {
		t.Fatal("no cap", err)
	}
}

func TestAuthBearer(t *testing.T) {
	h := RequireBearer("secret", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	unauth := httptest.NewRequest(http.MethodPost, "/writeback/convert", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, unauth)
	if rr.Code != 401 {
		t.Fatalf("unauth got %d", rr.Code)
	}
	auth := httptest.NewRequest(http.MethodPost, "/writeback/convert", nil)
	auth.Header.Set("Authorization", "Bearer secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, auth)
	if rr.Code != 200 {
		t.Fatalf("auth got %d", rr.Code)
	}
}

func TestTenantDisabled(t *testing.T) {
	item := enabledItem()
	item.WritebackEnabled = false
	_, err := testSvc(&fakeStore{item: item}, &fakeRingba{}, NewMemoryLocker(), &memAudit{}).Convert(context.Background(), baseReq())
	if !errors.Is(err, ErrTenantDisabled) {
		t.Fatal(err)
	}
}

func TestCrossTenantIntegrationMissing(t *testing.T) {
	_, err := testSvc(&fakeStore{err: fmt.Errorf("integration not found")}, &fakeRingba{}, NewMemoryLocker(), &memAudit{}).Convert(context.Background(), baseReq())
	if !errors.Is(err, ErrIntegrationMissing) {
		t.Fatal(err)
	}
}

func TestCallNotOnThisAccount(t *testing.T) {
	rb := &fakeRingba{lookupErr: fmt.Errorf("ringba: call not found in 7d window")}
	_, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), &memAudit{}).Convert(context.Background(), baseReq())
	if !errors.Is(err, ErrCallNotFound) {
		t.Fatal(err)
	}
}

func TestAlreadyConvertedRefused(t *testing.T) {
	rb := &fakeRingba{call: models.RemoteCall{ExternalCallID: "RGBTEST01", HasConverted: true, Revenue: 5}}
	_, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), &memAudit{}).Convert(context.Background(), baseReq())
	if !errors.Is(err, ErrAlreadyConverted) {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&rb.fires) != 0 {
		t.Fatal("must not fire")
	}
}

func TestConfirmedConvert(t *testing.T) {
	rb := &fakeRingba{
		call:      models.RemoteCall{ExternalCallID: "RGBTEST01", CampaignID: "CA1"},
		afterFire: models.RemoteCall{ExternalCallID: "RGBTEST01", CampaignID: "CA1", HasConverted: true, Revenue: 1},
	}
	audit := &memAudit{}
	res, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), audit).Convert(context.Background(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "confirmed" {
		t.Fatalf("got %s", res.Outcome)
	}
	if len(audit.rows) != 1 || audit.rows[0].Outcome != "confirmed" {
		t.Fatalf("audit %+v", audit.rows)
	}
}

func TestPendingConvert(t *testing.T) {
	rb := &fakeRingba{
		call: models.RemoteCall{ExternalCallID: "RGBTEST01"},
	}
	res, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), &memAudit{}).Convert(context.Background(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "pending" {
		t.Fatalf("got %s", res.Outcome)
	}
}

func TestFailedFire(t *testing.T) {
	rb := &fakeRingba{
		call:    models.RemoteCall{ExternalCallID: "RGBTEST01"},
		fireErr: fmt.Errorf("webhook HTTP 500"),
	}
	_, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), &memAudit{}).Convert(context.Background(), baseReq())
	if !errors.Is(err, ErrFireRejected) {
		t.Fatal(err)
	}
}

func TestDoubleClickOneFire(t *testing.T) {
	rb := &fakeRingba{
		call:      models.RemoteCall{ExternalCallID: "RGBTEST01"},
		afterFire: models.RemoteCall{ExternalCallID: "RGBTEST01", HasConverted: true, Revenue: 1},
		slowFire:  80 * time.Millisecond,
	}
	lock := NewMemoryLocker()
	svc := testSvc(&fakeStore{item: enabledItem()}, rb, lock, &memAudit{})
	var first, second error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, first = svc.Convert(context.Background(), baseReq())
	}()
	time.Sleep(15 * time.Millisecond)
	go func() {
		defer wg.Done()
		_, second = svc.Convert(context.Background(), baseReq())
	}()
	wg.Wait()
	if atomic.LoadInt32(&rb.fires) != 1 {
		t.Fatalf("fires=%d", rb.fires)
	}
	if first != nil {
		t.Fatalf("first %v", first)
	}
	if !errors.Is(second, ErrAlreadyProcessing) {
		t.Fatalf("second %v", second)
	}
}

func TestVoidConfirmed(t *testing.T) {
	rb := &fakeRingba{
		call:      models.RemoteCall{ExternalCallID: "RGBTEST01", HasConverted: true, Revenue: 1, Payout: 0},
		afterVoid: models.RemoteCall{ExternalCallID: "RGBTEST01", HasConverted: true, Revenue: 0},
	}
	res, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), &memAudit{}).Void(context.Background(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "confirmed" || !res.VoidReversible {
		t.Fatalf("%+v", res)
	}
}

func TestPickConversionWebhookPrefersMyWebhook(t *testing.T) {
	got, err := pickConversionWebhook([]adapter.RingbaConversionWebhook{
		{ID: "WH1", Name: "fdgf"},
		{ID: "WH2", Name: "My Webhook"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "WH2" {
		t.Fatalf("got %s", got.ID)
	}
}
