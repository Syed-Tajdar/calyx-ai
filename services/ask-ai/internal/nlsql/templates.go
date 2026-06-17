package nlsql

import "fmt"

// BuildTemplateSQL returns a safe, tenant-scoped SELECT for a known intent.
func BuildTemplateSQL(intent Intent, tenantID string, maxRows int) (string, error) {
	switch intent {
	case IntentTotalPings:
		return fmt.Sprintf(`
SELECT count() AS total_pings
FROM pings
WHERE tenant_id = '%s' AND is_duplicate = 0
LIMIT 1`, tenantID), nil
	case IntentTotalRevenue:
		return fmt.Sprintf(`
SELECT sum(revenue) AS total_revenue
FROM pings
WHERE tenant_id = '%s' AND is_duplicate = 0
LIMIT 1`, tenantID), nil
	case IntentTopPublisher:
		return fmt.Sprintf(`
SELECT publisher_id, sum(revenue) AS revenue
FROM pings
WHERE tenant_id = '%s' AND is_duplicate = 0
GROUP BY publisher_id
ORDER BY revenue DESC
LIMIT %d`, tenantID, maxRows), nil
	case IntentTopState:
		return fmt.Sprintf(`
SELECT state, count() AS ping_count
FROM pings
WHERE tenant_id = '%s' AND is_duplicate = 0 AND state != ''
GROUP BY state
ORDER BY ping_count DESC
LIMIT %d`, tenantID, maxRows), nil
	case IntentComplianceExposure:
		return fmt.Sprintf(`
SELECT
  sum(exposure_usd) AS total_exposure_usd,
  countIf(risk_score >= 70) AS violation_count
FROM compliance_risk_scores
WHERE tenant_id = '%s'
LIMIT 1`, tenantID), nil
	default:
		return "", fmt.Errorf("nlsql.BuildTemplateSQL: no template for intent %s", intent)
	}
}
