package nlsql

import (
	"context"
	"fmt"
	"time"

	chreader "github.com/Syed-Tajdar/calyx-ai/services/ask-ai/pkg/clickhouse"
)

// Executor runs validated read-only SQL.
type Executor struct {
	reader  *chreader.Reader
	timeout time.Duration
	maxRows int
}

// NewExecutor creates a ClickHouse query executor.
func NewExecutor(reader *chreader.Reader, timeout time.Duration, maxRows int) *Executor {
	return &Executor{reader: reader, timeout: timeout, maxRows: maxRows}
}

// Execute runs validated SQL with timeout.
func (e *Executor) Execute(ctx context.Context, sql string) ([]map[string]any, int, error) {
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	rows, err := e.reader.QueryJSON(execCtx, sql)
	if err != nil {
		return nil, 0, fmt.Errorf("nlsql.Execute: %w", err)
	}
	if len(rows) > e.maxRows {
		rows = rows[:e.maxRows]
	}
	return rows, len(rows), nil
}
