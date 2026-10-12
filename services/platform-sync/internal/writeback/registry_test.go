package writeback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// Stage 4b registry tests. convert_test.go (the Ringba suite) is untouched; these pin the
// parts of Ringba's behaviour the registry refactor could have moved.

func TestRegistryRingbaRegisteredByNewService(t *testing.T) {
	svc := testSvc(&fakeStore{item: enabledItem()}, &fakeRingba{}, NewMemoryLocker(), &memAudit{})
	if !svc.Supports("ringba") || !svc.Supports("Ringba") {
		t.Fatal("NewService must register Ringba")
	}
	if svc.Supports("retreaver") {
		t.Fatal("Retreaver is registered by main, not by NewService")
	}
	if got := svc.SupportedPlatforms(); len(got) != 1 || got[0] != "ringba" {
		t.Fatalf("platforms %v", got)
	}
}

func TestRegistryUnsupportedPlatformRejectedAndAudited(t *testing.T) {
	for _, platform := range []string{"trackdrive", "callgrid", "retreaver"} {
		t.Run(platform, func(t *testing.T) {
			item := enabledItem()
			item.Platform = platform
			audit := &memAudit{}
			rb := &fakeRingba{}
			_, err := testSvc(&fakeStore{item: item}, rb, NewMemoryLocker(), audit).Convert(context.Background(), baseReq())
			if !errors.Is(err, ErrPlatformUnsupported) || !errors.Is(err, ErrNotRingba) {
				t.Fatalf("err=%v", err)
			}
			if HTTPStatus(err) != 403 {
				t.Fatalf("status %d", HTTPStatus(err))
			}
			if len(audit.rows) != 1 || audit.rows[0].Outcome != "rejected" {
				t.Fatalf("rows %+v", audit.rows)
			}
			if rb.fires != 0 || rb.voids != 0 {
				t.Fatal("no platform call for an unsupported platform")
			}
		})
	}
}

type listErrRingba struct{ fakeRingba }

func (l *listErrRingba) ListConversionWebhooks(context.Context, map[string]string) ([]adapter.RingbaConversionWebhook, error) {
	return nil, fmt.Errorf("ringba: list webhooks HTTP 500")
}

func TestRingbaWebhookListErrorStaysBareAndUnaudited(t *testing.T) {
	rb := &listErrRingba{fakeRingba{call: models.RemoteCall{ExternalCallID: "RGBTEST01"}}}
	audit := &memAudit{}
	_, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), audit).Convert(context.Background(), baseReq())
	if err == nil || errors.Is(err, ErrFireRejected) || err.Error() != "ringba: list webhooks HTTP 500" {
		t.Fatalf("err=%v", err)
	}
	if len(audit.rows) != 0 {
		t.Fatalf("pre-registry behaviour wrote no audit row here, got %+v", audit.rows)
	}
	if rb.fires != 0 {
		t.Fatal("must not fire")
	}
}

type voidArgsRingba struct {
	fakeRingba
	gotRev, gotPay float64
}

func (v *voidArgsRingba) VoidCall(ctx context.Context, creds map[string]string, id string, convAmt, payAmt float64) (string, error) {
	v.gotRev, v.gotPay = convAmt, payAmt
	return v.fakeRingba.VoidCall(ctx, creds, id, convAmt, payAmt)
}

func TestRingbaVoidPassesCurrentRevenueAndPayout(t *testing.T) {
	rb := &voidArgsRingba{fakeRingba: fakeRingba{
		call:      models.RemoteCall{ExternalCallID: "RGBTEST01", HasConverted: true, Revenue: 7.5, Payout: 2.25},
		afterVoid: models.RemoteCall{ExternalCallID: "RGBTEST01"},
	}}
	audit := &memAudit{}
	res, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), audit).Void(context.Background(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	if rb.gotRev != 7.5 || rb.gotPay != 2.25 {
		t.Fatalf("void args rev=%v pay=%v", rb.gotRev, rb.gotPay)
	}
	if res.RingbaRef != "TR-void" || res.Amount != 7.5 {
		t.Fatalf("%+v", res)
	}
	if len(audit.rows) != 1 || audit.rows[0].RingbaRef != "TR-void" || audit.rows[0].Amount != 7.5 {
		t.Fatalf("rows %+v", audit.rows)
	}
}

func TestRingbaResponseAndAuditShapeUnchanged(t *testing.T) {
	rb := &fakeRingba{
		call:      models.RemoteCall{ExternalCallID: "RGBTEST01", CampaignID: "CA1"},
		afterFire: models.RemoteCall{ExternalCallID: "RGBTEST01", CampaignID: "CA1", HasConverted: true, Revenue: 1},
	}
	audit := &memAudit{}
	res, err := testSvc(&fakeStore{item: enabledItem()}, rb, NewMemoryLocker(), audit).Convert(context.Background(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if _, ok := m["platform"]; ok {
		t.Fatalf("ringba response gained a platform key: %s", b)
	}
	if _, ok := m["read_back"]; ok {
		t.Fatalf("ringba response gained a read_back key: %s", b)
	}
	if res.WebhookName != "My Webhook" {
		t.Fatalf("webhook %q", res.WebhookName)
	}
	row := audit.rows[0]
	if row.Platform != "" || row.ReadBack != "" {
		t.Fatalf("ringba audit row must use the original insert: %+v", row)
	}
}
