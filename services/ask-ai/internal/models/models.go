package models

// AskRequest is the inbound question payload.
type AskRequest struct {
	TenantID string `json:"tenant_id"`
	Question string `json:"question"`
	TraceID  string `json:"trace_id"`
}

// AskResponse is returned to the API layer.
type AskResponse struct {
	QueryID   string `json:"query_id"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Intent    string `json:"intent"`
	CacheHit  bool   `json:"cache_hit"`
	RowCount  int    `json:"row_count"`
	TokensUsed int   `json:"tokens_used"`
}

// HistoryRecord is a prior Q&A row.
type HistoryRecord struct {
	ID         string `json:"id"`
	Question   string `json:"question"`
	Answer     string `json:"answer"`
	Intent     string `json:"intent"`
	CacheHit   bool   `json:"cache_hit"`
	RowCount   int    `json:"row_count"`
	TokensUsed int    `json:"tokens_used"`
	CreatedAt  string `json:"created_at"`
}

// QueryResult holds validated SQL execution output.
type QueryResult struct {
	Rows    []map[string]any
	RowCount int
}
