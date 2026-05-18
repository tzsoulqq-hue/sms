package handlerapi

import (
	"errors"
	"testing"

	"github.com/byte-v-forge/sms/internal/core"
)

func TestMapTextErrorMatchesHandlerAPIProviderCodes(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		code      core.ErrorCode
		retryable bool
	}{
		{name: "bad key", text: "BAD_KEY", code: core.CodeUpstreamRejected},
		{name: "bad action", text: "BAD_ACTION", code: core.CodeUnsupportedOperation},
		{name: "wrong max price with minimum", text: "WRONG_MAX_PRICE:0.12", code: core.CodePriceLimitExceeded},
		{name: "no balance", text: "NO_BALANCE", code: core.CodeInsufficientBalance},
		{name: "no numbers", text: "NO_NUMBERS", code: core.CodeNoNumberAvailable, retryable: true},
		{name: "early cancel", text: "EARLY_CANCEL_DENIED", code: core.CodeCancelNotAllowed, retryable: true},
		{name: "wrong prefix", text: "WRONG_EXCEPTION_PHONE", code: core.CodeValidationFailed},
		{name: "temporary sql failure", text: "ERROR_SQL", code: core.CodeSupplyUnavailable, retryable: true},
		{name: "blocked account", text: "BANNED:'2026-05-18 12:00:00'", code: core.CodeSupplyUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapTextError(tt.text)
			var smsErr *core.Error
			if !errors.As(err, &smsErr) {
				t.Fatalf("error = %T, want *core.Error", err)
			}
			if smsErr.Code != tt.code || smsErr.Retryable != tt.retryable {
				t.Fatalf("mapped error = %#v, want code=%s retryable=%v", smsErr, tt.code, tt.retryable)
			}
		})
	}
}
