package grpcadapter

import (
	"errors"
	"time"

	smsv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/contracts/sms/v1"
	"github.com/byte-v-forge/sms/internal/core"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toProtoActivation(activation core.Activation) *smsv1.SmsActivation {
	return &smsv1.SmsActivation{
		ActivationId: activation.ID,
		RequestId:    activation.RequestID,
		Target: &smsv1.SmsTarget{
			ApplicationKey:     activation.Target.ApplicationKey,
			CountryIso2:        activation.Target.CountryISO2,
			CountryCallingCode: activation.Target.CountryCallingCode,
			MaxPrice:           toProtoMoney(activation.Target.MaxPrice),
		},
		PhoneNumber: &smsv1.PhoneNumber{
			E164Number:         activation.PhoneNumber.E164,
			NationalNumber:     activation.PhoneNumber.NationalNumber,
			CountryIso2:        activation.PhoneNumber.CountryISO2,
			CountryCallingCode: activation.PhoneNumber.CountryCallingCode,
		},
		Status:                   toProtoStatus(activation.Status),
		Price:                    toProtoMoney(activation.Price),
		AcquiredAt:               toProtoTime(activation.AcquiredAt),
		ExpiresAt:                toProtoTime(activation.ExpiresAt),
		UpdatedAt:                toProtoTime(activation.UpdatedAt),
		LastError:                toProtoError(activation.LastError),
		CanRequestAdditionalCode: activation.CanRequestAdditionalCode,
		CancelAllowedAt:          toProtoTime(activation.CancelAllowedAt),
		Labels:                   activation.Labels,
	}
}

func toProtoCode(code *core.SMSCode) *smsv1.SmsCode {
	if code == nil {
		return nil
	}
	return &smsv1.SmsCode{Value: code.Value, ReceivedAt: toProtoTime(code.ReceivedAt)}
}

func toProtoMoney(money core.Money) *smsv1.DecimalMoney {
	if money.CurrencyCode == "" && money.AmountDecimal == "" {
		return nil
	}
	return &smsv1.DecimalMoney{CurrencyCode: money.CurrencyCode, AmountDecimal: money.AmountDecimal}
}

func fromProtoTarget(target *smsv1.SmsTarget) core.Target {
	if target == nil {
		return core.Target{}
	}
	var maxPrice core.Money
	if target.GetMaxPrice() != nil {
		maxPrice = core.Money{
			CurrencyCode:  target.GetMaxPrice().GetCurrencyCode(),
			AmountDecimal: target.GetMaxPrice().GetAmountDecimal(),
		}
	}
	return core.Target{
		ApplicationKey:     target.GetApplicationKey(),
		CountryISO2:        target.GetCountryIso2(),
		CountryCallingCode: target.GetCountryCallingCode(),
		MaxPrice:           maxPrice,
	}
}

func toProtoError(err error) *smsv1.SmsError {
	if err == nil {
		return nil
	}
	var smsErr *core.Error
	if !errors.As(err, &smsErr) {
		smsErr = core.NewError(core.CodeInternal, err.Error(), false)
	}
	if smsErr == nil {
		return nil
	}
	return &smsv1.SmsError{
		Code:      toProtoErrorCode(smsErr.Code),
		Message:   smsErr.Message,
		Retryable: smsErr.Retryable,
	}
}

func toProtoStatus(status core.ActivationStatus) smsv1.SmsActivationStatus {
	switch status {
	case core.StatusPendingCode:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_PENDING_CODE
	case core.StatusMessageSent:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_MESSAGE_SENT
	case core.StatusCodeReceived:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_CODE_RECEIVED
	case core.StatusAdditionalCodeRequested:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_ADDITIONAL_CODE_REQUESTED
	case core.StatusCompleted:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_COMPLETED
	case core.StatusCanceled:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_CANCELED
	case core.StatusExpired:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_EXPIRED
	case core.StatusFailed:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_FAILED
	default:
		return smsv1.SmsActivationStatus_SMS_ACTIVATION_STATUS_UNSPECIFIED
	}
}

func toProtoErrorCode(code core.ErrorCode) smsv1.SmsErrorCode {
	switch code {
	case core.CodeValidationFailed:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_VALIDATION_FAILED
	case core.CodeRouteNotFound:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_ROUTE_NOT_FOUND
	case core.CodeActivationNotFound:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_ACTIVATION_NOT_FOUND
	case core.CodeActivationAlreadyFinalized:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_ACTIVATION_ALREADY_FINALIZED
	case core.CodeNoNumberAvailable:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_NO_NUMBER_AVAILABLE
	case core.CodePriceLimitExceeded:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_PRICE_LIMIT_EXCEEDED
	case core.CodeRateLimited:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_RATE_LIMITED
	case core.CodeSupplyUnavailable:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_SUPPLY_UNAVAILABLE
	case core.CodeUpstreamRejected:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_UPSTREAM_REJECTED
	case core.CodeTimeout:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_TIMEOUT
	case core.CodeUnsupportedOperation:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_UNSUPPORTED_OPERATION
	case core.CodeActivationExpired:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_ACTIVATION_EXPIRED
	case core.CodeCancelNotAllowed:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_CANCEL_NOT_ALLOWED
	case core.CodeInsufficientBalance:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_INSUFFICIENT_BALANCE
	case core.CodeInternal:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_INTERNAL
	default:
		return smsv1.SmsErrorCode_SMS_ERROR_CODE_UNSPECIFIED
	}
}

func protoDuration(value *durationpb.Duration) time.Duration {
	if value == nil {
		return 0
	}
	return value.AsDuration()
}

func toProtoTime(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}
