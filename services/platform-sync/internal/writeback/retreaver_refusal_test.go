package writeback

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The 422 body Retreaver returned in Phase 3 (tracking number replaced by a fake one).
const recordingAccessBody = `{"call":{"errors":{"call_base":{"label":"Base","messages":["Access denied to 'recording_access'."]}},"campaign":{"id":54339,"name":"3d4d4d1c - Calyx Sandox - Do not use"},"number":{"number":"+18005550123","name":null}}}`

func TestRetreaverRecordingAccessRefusalIsPlainAndAudited(t *testing.T) {
	for _, tc := range []struct {
		name    string
		convert bool
	}{{"convert", true}, {"void", false}} {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeRetreaverAPI()
			if !tc.convert {
				api.revenue, api.converted = f64(1), true
			}
			api.putStatuses = []int{422, 422, 422}
			api.putBody = recordingAccessBody
			audit := &memAudit{}
			svc := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil)
			var (
				res Result
				err error
			)
			if tc.convert {
				res, err = svc.Convert(context.Background(), rtReq())
			} else {
				res, err = svc.Void(context.Background(), rtReq())
			}
			if !errors.Is(err, ErrFireRejected) || HTTPStatus(err) != 502 {
				t.Fatalf("err=%v", err)
			}
			if api.putCount() != 1 {
				t.Fatalf("422 retried: puts=%d", api.putCount())
			}
			if res.Outcome != "failed" || res.Message != ErrRetreaverRecordingAccess {
				t.Fatalf("result %+v", res)
			}
			if !strings.Contains(err.Error(), "needs recording access") {
				t.Fatalf("error should say what to fix: %v", err)
			}
			if len(audit.rows) != 1 || audit.rows[0].Outcome != "failed" || audit.rows[0].Message != ErrRetreaverRecordingAccess {
				t.Fatalf("rows %+v", audit.rows)
			}
			for where, text := range map[string]string{"error": err.Error(), "message": res.Message, "audit": audit.rows[0].Message} {
				if strings.Contains(text, "5550123") {
					t.Fatalf("%s leaks the tracking number: %s", where, text)
				}
				noSecret(t, where, text)
			}
		})
	}
}

func TestRetreaverWriteErrorMasksPhoneNumbers(t *testing.T) {
	api := newFakeRetreaverAPI()
	api.putStatuses = []int{422, 422, 422}
	api.putBody = `{"call":{"errors":{"revenue":["is invalid"]}},"number":{"number":"+18005550123"}}`
	audit := &memAudit{}
	res, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
	if !errors.Is(err, ErrFireRejected) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(res.Message, "is invalid") || !strings.Contains(res.Message, "***") {
		t.Fatalf("message should keep the reason and mask the number: %q", res.Message)
	}
	for where, text := range map[string]string{"error": err.Error(), "message": res.Message, "audit": audit.rows[0].Message} {
		if strings.Contains(text, "5550123") {
			t.Fatalf("%s leaks the tracking number: %s", where, text)
		}
	}
}

func TestRetreaverWriteErrPreservesWrappedError(t *testing.T) {
	inner := errors.New(`retreaver HTTP 422: body={"messages":["Access denied to 'recording_access'."]}`)
	got := retreaverWriteErr(inner)
	if got.Error() != ErrRetreaverRecordingAccess || !errors.Is(got, inner) {
		t.Fatalf("got %v", got)
	}
	plain := errors.New("retreaver: call not found (HTTP 404)")
	if retreaverWriteErr(plain) != plain {
		t.Fatal("an error without a phone number or refusal must pass through unchanged")
	}
	if retreaverWriteErr(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}
