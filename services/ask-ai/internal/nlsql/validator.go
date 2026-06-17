package nlsql

import (
	"fmt"
	"regexp"
	"strings"
)

var forbidden = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|TRUNCATE|CREATE|GRANT|REVOKE|ATTACH|DETACH|SYSTEM|OPTIMIZE|KILL)\b`)

// Validate ensures SQL is read-only and tenant-scoped.
func Validate(sql string, tenantID string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return fmt.Errorf("validate: empty sql")
	}
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") {
		return fmt.Errorf("validate: only SELECT allowed")
	}
	if strings.Contains(trimmed, ";") {
		return fmt.Errorf("validate: multiple statements not allowed")
	}
	if forbidden.MatchString(trimmed) {
		return fmt.Errorf("validate: forbidden keyword detected")
	}
	tenantNeedle := fmt.Sprintf("tenant_id = '%s'", tenantID)
	if !strings.Contains(trimmed, tenantNeedle) {
		return fmt.Errorf("validate: tenant_id filter required")
	}
	return nil
}
