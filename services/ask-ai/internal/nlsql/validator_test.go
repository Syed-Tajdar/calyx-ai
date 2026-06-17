package nlsql

import "testing"

func TestValidateRejectsNonSelect(t *testing.T) {
	err := Validate("DELETE FROM pings WHERE tenant_id = '11111111-1111-1111-1111-111111111111'", "11111111-1111-1111-1111-111111111111")
	if err == nil {
		t.Fatal("expected error for non-SELECT")
	}
}

func TestValidateRequiresTenantFilter(t *testing.T) {
	sql := "SELECT count() FROM pings"
	tenant := "11111111-1111-1111-1111-111111111111"
	if err := Validate(sql, tenant); err == nil {
		t.Fatal("expected missing tenant filter error")
	}
}

func TestValidateAcceptsSafeSelect(t *testing.T) {
	tenant := "11111111-1111-1111-1111-111111111111"
	sql, err := BuildTemplateSQL(IntentTotalPings, tenant, 10)
	if err != nil {
		t.Fatalf("template: %v", err)
	}
	if err := Validate(sql, tenant); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestParseIntentRevenue(t *testing.T) {
	if got := ParseIntent("What is my total revenue?"); got != IntentTotalRevenue {
		t.Fatalf("got %s want %s", got, IntentTotalRevenue)
	}
}
