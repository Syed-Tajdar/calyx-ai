package dedup

import (
	"time"

	"github.com/Syed-Tajdar/calyx-ai/packages/pingdedup"
	"github.com/redis/go-redis/v9"
)

type (
	Checker  = pingdedup.Checker
	Content  = pingdedup.Content
	Decision = pingdedup.Decision
)

const (
	PublishNew     = pingdedup.PublishNew
	PublishChanged = pingdedup.PublishChanged
	Unchanged      = pingdedup.Unchanged
)

func NewChecker(client *redis.Client, ttl time.Duration) *Checker {
	return pingdedup.NewChecker(client, ttl)
}

func Hash(c Content) string {
	return pingdedup.Hash(c)
}
