package platforms

import (
	"context"
	"log/slog"

	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/models"
)

// MockPlatform logs would-be conversion fires without external calls.
type MockPlatform struct {
	logger *slog.Logger
}

// NewMockPlatform creates a mock ad platform.
func NewMockPlatform(logger *slog.Logger) *MockPlatform {
	return &MockPlatform{logger: logger}
}

// Name returns mock.
func (m *MockPlatform) Name() string {
	return "mock"
}

// Fire logs the payload and returns success.
func (m *MockPlatform) Fire(_ context.Context, req models.FireRequest) (models.FireResult, error) {
	m.logger.Info("mock attribution fire",
		"service", "ad-attribution",
		"tenant_id", req.TenantID,
		"call_id", req.CallID,
		"platform", req.Platform,
		"click_id", req.ClickID,
		"revenue", req.Revenue,
	)
	return models.FireResult{Success: true, HTTPStatus: 200}, nil
}

// MetaCAPIPlatform is a placeholder for Meta Conversions API.
type MetaCAPIPlatform struct{}

// NewMetaCAPIPlatform creates Meta CAPI client stub.
func NewMetaCAPIPlatform() *MetaCAPIPlatform {
	return &MetaCAPIPlatform{}
}

func (p *MetaCAPIPlatform) Name() string { return "meta" }

func (p *MetaCAPIPlatform) Fire(_ context.Context, _ models.FireRequest) (models.FireResult, error) {
	return models.FireResult{}, errNotImplemented("meta_capi")
}

// GoogleAdsPlatform is a placeholder for Google Ads conversions.
type GoogleAdsPlatform struct{}

// NewGoogleAdsPlatform creates Google Ads client stub.
func NewGoogleAdsPlatform() *GoogleAdsPlatform {
	return &GoogleAdsPlatform{}
}

func (p *GoogleAdsPlatform) Name() string { return "google" }

func (p *GoogleAdsPlatform) Fire(_ context.Context, _ models.FireRequest) (models.FireResult, error) {
	return models.FireResult{}, errNotImplemented("google_ads")
}

func errNotImplemented(name string) error {
	return &notImplementedError{name: name}
}

type notImplementedError struct {
	name string
}

func (e *notImplementedError) Error() string {
	return e.name + " provider not implemented in phase 3 step 4 skeleton"
}

// Registry selects platform implementation by resolved name.
func Registry(name string, logger *slog.Logger) Platform {
	switch name {
	case "mock", "":
		return NewMockPlatform(logger)
	case "meta", "meta_capi":
		return NewMetaCAPIPlatform()
	case "google", "google_ads":
		return NewGoogleAdsPlatform()
	default:
		return NewMockPlatform(logger)
	}
}
