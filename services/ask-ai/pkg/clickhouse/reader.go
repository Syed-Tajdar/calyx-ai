package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/shopspring/decimal"
)

// Reader executes read-only ClickHouse queries.
type Reader struct {
	conn clickhouse.Conn
}

// NewReader connects to ClickHouse.
func NewReader(addr, database, username, password string) (*Reader, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:        []string{addr},
		Auth:        clickhouse.Auth{Database: database, Username: username, Password: password},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse.NewReader: %w", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("clickhouse.NewReader ping: %w", err)
	}
	return &Reader{conn: conn}, nil
}

// Ping checks connectivity.
func (r *Reader) Ping(ctx context.Context) error {
	return r.conn.Ping(ctx)
}

// Close closes the connection.
func (r *Reader) Close() error {
	return r.conn.Close()
}

// QueryJSON runs a SELECT and returns rows as maps.
func (r *Reader) QueryJSON(ctx context.Context, sql string) ([]map[string]any, error) {
	rows, err := r.conn.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("clickhouse.QueryJSON: %w", err)
	}
	defer rows.Close()

	cols := rows.Columns()
	colTypes := rows.ColumnTypes()
	out := make([]map[string]any, 0)

	for rows.Next() {
		scan := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i, colType := range colTypes {
			dbType := colType.DatabaseTypeName()
			if isDecimalType(dbType) {
				var v decimal.Decimal
				scan[i] = &v
			} else {
				switch colType.ScanType().Kind() {
				case reflect.String:
					var v string
					scan[i] = &v
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					var v int64
					scan[i] = &v
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					var v uint64
					scan[i] = &v
				case reflect.Float32, reflect.Float64:
					var v float64
					scan[i] = &v
				default:
					var v string
					scan[i] = &v
				}
			}
			ptrs[i] = scan[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("clickhouse.QueryJSON scan: %w", err)
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			val := deref(scan[i])
			row[col] = val
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	// Normalize numeric types for JSON friendliness.
	normalized, err := normalizeRows(out)
	if err != nil {
		return out, nil
	}
	return normalized, nil
}

func isDecimalType(dbType string) bool {
	return strings.HasPrefix(dbType, "Decimal")
}

func deref(ptr any) any {
	switch v := ptr.(type) {
	case *string:
		return *v
	case *int64:
		return *v
	case *uint64:
		return *v
	case *float64:
		return *v
	case *decimal.Decimal:
		f, _ := v.Float64()
		return f
	default:
		return fmt.Sprint(ptr)
	}
}

func normalizeRows(rows []map[string]any) ([]map[string]any, error) {
	b, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
