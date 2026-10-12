package writeback

import (
	"errors"
	"strings"
)

var (
	ErrInvalidAmount = errors.New("writeback: amount must be a positive number")
	// ErrPlatformUnsupported: no Writer is registered for the integration's platform.
	ErrPlatformUnsupported = errors.New("writeback: write-back is not supported for this platform")
	// ErrNotRingba is the pre-registry name, kept so errors.Is keeps matching.
	ErrNotRingba          = ErrPlatformUnsupported
	ErrNotLive            = errors.New("writeback: integration must be Live (sync_mode=real)")
	ErrTenantDisabled     = errors.New("writeback: writeback_enabled is false for this integration")
	ErrAlreadyConverted   = errors.New("writeback: call already converted")
	ErrNotConverted       = errors.New("writeback: call is not converted")
	ErrAlreadyProcessing  = errors.New("writeback: convert already in progress for this call")
	ErrAlreadyVoiding     = errors.New("writeback: void already in progress for this call")
	ErrCallNotFound       = errors.New("writeback: call not found on this tenant's Ringba account")
	ErrAccountMismatch    = errors.New("writeback: call does not belong to this tenant's Ringba account")
	ErrIntegrationMissing = errors.New("writeback: integration not found")
	ErrGloballyDisabled   = errors.New("writeback: write-back is disabled")
	ErrFireRejected       = errors.New("writeback: Ringba rejected the fire")
	// ErrReadBackMismatch: the write was accepted but the read-back shows a contradicting value
	// (neither what was written nor the value before). A read-back that simply has not caught
	// up yet is pending, not this.
	ErrReadBackMismatch = errors.New("writeback: read-back contradicts the write")
	// ErrInvalidCallID: the call id is not in the platform's id format (Retreaver: UUID).
	ErrInvalidCallID = errors.New("writeback: invalid call id for this platform")
)

func IsInvalidAmount(err error) bool { return errors.Is(err, ErrInvalidAmount) }

func HTTPStatus(err error) int {
	if err == nil {
		return 200
	}
	switch {
	case errors.Is(err, ErrInvalidAmount), errors.Is(err, ErrInvalidCallID):
		return 400
	case errors.Is(err, ErrAlreadyConverted), errors.Is(err, ErrAlreadyProcessing),
		errors.Is(err, ErrAlreadyVoiding), errors.Is(err, ErrNotConverted):
		return 409
	case errors.Is(err, ErrTenantDisabled), errors.Is(err, ErrGloballyDisabled), errors.Is(err, ErrNotLive),
		errors.Is(err, ErrPlatformUnsupported), errors.Is(err, ErrAccountMismatch):
		return 403
	case errors.Is(err, ErrCallNotFound), errors.Is(err, ErrIntegrationMissing):
		return 404
	case errors.Is(err, ErrFireRejected), errors.Is(err, ErrReadBackMismatch):
		return 502
	default:
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			return 404
		}
		if strings.Contains(msg, "required") {
			return 400
		}
		return 500
	}
}
