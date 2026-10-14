package store

import (
	"context"
	"testing"
)

// Stage 4b. Ringba audit rows keep the original 11-argument insert (platform and read_back stay
// NULL); Retreaver rows go through insert_writeback_audit_v2 (migration 000032) and keep the
// platform and the read-back JSON. Database-backed: skips unless DATABASE_URL is set.
func TestInsertWritebackAuditRingbaAndRetreaver(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	st := NewIntegrationStore(pool)
	ctx := context.Background()

	ringbaID, err := st.InsertWritebackAudit(ctx, AuditRow{
		TenantID: testTenantID, InboundCallID: "RGBTEST01", Action: "convert", Amount: 1,
		Outcome: "confirmed", CampaignID: "CA1", Message: "confirmed", OfflineUploaded: "absent",
	})
	if err != nil {
		t.Fatalf("ringba insert: %v", err)
	}
	retID, err := st.InsertWritebackAudit(ctx, AuditRow{
		TenantID: testTenantID, InboundCallID: "addcf985-017e-4962-be34-cf5d55e74afc", Action: "void",
		Amount: 1, Outcome: "confirmed", Message: "void confirmed", OfflineUploaded: "absent",
		Platform: "retreaver",
		ReadBack: `{"after":{"revenue":null,"converted":false,"fired_pixels_count":0},"settle_seconds":0.4}`,
	})
	if err != nil {
		t.Fatalf("retreaver insert: %v", err)
	}

	var platform *string
	var readBack *string
	if err := pool.QueryRow(ctx, `SELECT platform, read_back::text FROM writeback_audit WHERE id = $1::uuid`, ringbaID).Scan(&platform, &readBack); err != nil {
		t.Fatal(err)
	}
	if platform != nil || readBack != nil {
		t.Fatalf("ringba row must keep NULL platform/read_back, got %v %v", platform, readBack)
	}
	var revenueIsNull bool
	if err := pool.QueryRow(ctx, `SELECT platform, read_back::text, (read_back->'after'->'revenue') = 'null'::jsonb FROM writeback_audit WHERE id = $1::uuid`, retID).Scan(&platform, &readBack, &revenueIsNull); err != nil {
		t.Fatal(err)
	}
	if platform == nil || *platform != "retreaver" || readBack == nil || !revenueIsNull {
		t.Fatalf("retreaver row: platform=%v read_back=%v revenueIsNull=%v", platform, readBack, revenueIsNull)
	}
}
