package connectors

import (
	"fmt"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]PlatformConnector)
)

// Register adds a platform connector to the webhook registry.
func Register(connector PlatformConnector) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[connector.Platform()] = connector
}

// Get returns the connector for a platform name.
func Get(platform string) (PlatformConnector, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	connector, ok := registry[platform]
	if !ok {
		return nil, fmt.Errorf("connectors.Get: unknown platform %q", platform)
	}
	return connector, nil
}

// WebhookPlatforms lists platforms served via /webhooks/{platform}/{token}.
func WebhookPlatforms() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

func init() {
	Register(&RingbaConnector{})
	Register(&RetreaverConnector{})
	Register(&TrackDriveConnector{})
	Register(&CallGridConnector{})
}
