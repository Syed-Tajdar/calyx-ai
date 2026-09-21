package writeback

import (
	"errors"
	"strings"
)

var (
	ErrInvalidAmount      = errors.New("writeback: amount must be a positive number")
	ErrNotRingba          = errors.New("writeback: only ringba is supported")
	ErrNotLive            = errors.New("writeback: integration must be Live (sync_mode=real)")
	ErrTenantDisabled     = errors.New("writeback: writeback_enabled is false for this integration")
	ErrAlreadyConverted   = errors.New("writeback: call already converted")
	ErrNotConverted       = errors.New("writeback: call is not converted")
	ErrAlreadyProcessing  = errors.New("writeback: convert already in progress for this call")
	ErrAlreadyVoiding     = errors.New("writeback: void already in progress for this call")
	ErrCallNotFound       = errors.New("writeback: call not found on this tenant's Ringba account")
	ErrAccountMismatch    = errors.New("writeback: call does not belong to this tenant's Ringba account")
	ErrIntegrationMissing = errors.New("writeback: integration not found")
	ErrFireRejected       = errors.New("writeback: Ringba rejected the fire")
)

func IsInvalidAmount(err error) bool { return errors.Is(err, ErrInvalidAmount) }

func HTTPStatus(err error) int {
	if err == nil {
		return 200
	}
	switch {
	case errors.Is(err, ErrInvalidAmount):
		return 400
	case errors.Is(err, ErrAlreadyConverted), errors.Is(err, ErrAlreadyProcessing),
		errors.Is(err, ErrAlreadyVoiding), errors.Is(err, ErrNotConverted):
		return 409
	case errors.Is(err, ErrTenantDisabled), errors.Is(err, ErrNotLive), errors.Is(err, ErrNotRingba),
		errors.Is(err, ErrAccountMismatch):
		return 403
	case errors.Is(err, ErrCallNotFound), errors.Is(err, ErrIntegrationMissing):
		return 404
	case errors.Is(err, ErrFireRejected):
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
