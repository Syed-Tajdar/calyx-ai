package cost

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BudgetGuard tracks per-tenant daily token spend in memory (dev/MVP).
type BudgetGuard struct {
	mu      sync.Mutex
	limit   int
	spent   map[string]int
	resetAt time.Time
}

// NewBudgetGuard creates a token budget guard.
func NewBudgetGuard(dailyLimit int) *BudgetGuard {
	return &BudgetGuard{
		limit:   dailyLimit,
		spent:   map[string]int{},
		resetAt: nextUTCReset(),
	}
}

// Allow checks whether tenant can spend tokens.
func (b *BudgetGuard) Allow(_ context.Context, tenantID string, tokens int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if time.Now().UTC().After(b.resetAt) {
		b.spent = map[string]int{}
		b.resetAt = nextUTCReset()
	}
	next := b.spent[tenantID] + tokens
	if next > b.limit {
		return fmt.Errorf("budget_guard: tenant %s exceeded daily token budget", tenantID)
	}
	b.spent[tenantID] = next
	return nil
}

func nextUTCReset() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
