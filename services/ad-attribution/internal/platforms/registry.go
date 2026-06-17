package platforms

import "log/slog"

// RegistryByProvider is an alias for Registry (providerconfig provider_id).
func RegistryByProvider(providerID string, logger *slog.Logger) Platform {
	return Registry(providerID, logger)
}
