package core

import (
	"fmt"
	"time"
)

type Money struct {
	CurrencyCode  string
	AmountDecimal string
}

type PhoneNumber struct {
	E164               string
	NationalNumber     string
	CountryISO2        string
	CountryCallingCode string
}

type Target struct {
	ApplicationKey     string
	CountryISO2        string
	CountryCallingCode string
	MinPrice           Money
	MaxPrice           Money
}

type ActivationStatus string

const (
	StatusPendingCode             ActivationStatus = "pending_code"
	StatusMessageSent             ActivationStatus = "message_sent"
	StatusCodeReceived            ActivationStatus = "code_received"
	StatusAdditionalCodeRequested ActivationStatus = "additional_code_requested"
	StatusCompleted               ActivationStatus = "completed"
	StatusCanceled                ActivationStatus = "canceled"
	StatusExpired                 ActivationStatus = "expired"
	StatusFailed                  ActivationStatus = "failed"
)

func (s ActivationStatus) IsFinal() bool {
	switch s {
	case StatusCompleted, StatusCanceled, StatusExpired, StatusFailed:
		return true
	default:
		return false
	}
}

type ErrorCode string

const (
	CodeValidationFailed           ErrorCode = "validation_failed"
	CodeRouteNotFound              ErrorCode = "route_not_found"
	CodeActivationNotFound         ErrorCode = "activation_not_found"
	CodeActivationAlreadyFinalized ErrorCode = "activation_already_finalized"
	CodeNoNumberAvailable          ErrorCode = "no_number_available"
	CodePriceLimitExceeded         ErrorCode = "price_limit_exceeded"
	CodeRateLimited                ErrorCode = "rate_limited"
	CodeSupplyUnavailable          ErrorCode = "supply_unavailable"
	CodeUpstreamRejected           ErrorCode = "upstream_rejected"
	CodeTimeout                    ErrorCode = "timeout"
	CodeUnsupportedOperation       ErrorCode = "unsupported_operation"
	CodeActivationExpired          ErrorCode = "activation_expired"
	CodeCancelNotAllowed           ErrorCode = "cancel_not_allowed"
	CodeInsufficientBalance        ErrorCode = "insufficient_balance"
	CodeInternal                   ErrorCode = "internal"
)

type Error struct {
	Code      ErrorCode
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code ErrorCode, message string, retryable bool) *Error {
	return &Error{Code: code, Message: message, Retryable: retryable}
}

type SMSCode struct {
	Value       string
	MessageText string
	ReceivedAt  time.Time
}

type Activation struct {
	ID                       string
	RequestID                string
	ProviderConfigID         string
	ProviderKey              string
	UpstreamActivationID     string
	UpstreamOperator         string
	Target                   Target
	PhoneNumber              PhoneNumber
	Status                   ActivationStatus
	Price                    Money
	AcquiredAt               time.Time
	ExpiresAt                time.Time
	UpdatedAt                time.Time
	CancelAllowedAt          time.Time
	Code                     *SMSCode
	CanRequestAdditionalCode bool
	Labels                   map[string]string
	LastError                *Error
}

func (a Activation) IsExpired(now time.Time) bool {
	return !a.ExpiresAt.IsZero() && !a.Status.IsFinal() && !now.Before(a.ExpiresAt)
}

type ProviderAction string

const (
	ActionMarkMessageSent    ProviderAction = "mark_message_sent"
	ActionRequestAdditional  ProviderAction = "request_additional_code"
	ActionCompleteActivation ProviderAction = "complete_activation"
	ActionCancelActivation   ProviderAction = "cancel_activation"
)

type ProviderPolicy struct {
	ActivationTTL         time.Duration
	PollInterval          time.Duration
	CancelAllowedAfter    time.Duration
	EarlyCancelRetryAfter time.Duration
	CancelAllowedUntil    time.Duration
}

func (p ProviderPolicy) WithDefaults() ProviderPolicy {
	if p.ActivationTTL <= 0 {
		p.ActivationTTL = 20 * time.Minute
	}
	if p.PollInterval <= 0 {
		p.PollInterval = 5 * time.Second
	}
	return p
}

type Route struct {
	ProviderKey               string
	ApplicationKey            string
	UpstreamServiceKey        string
	CountryISO2               string
	CountryCallingCode        string
	ProviderCountryID         string
	MinPrice                  Money
	MaxPrice                  Money
	IncludeUpstreamProviderID []string
	ExcludeUpstreamProviderID []string
	ExcludedPhonePrefixes     []string
	ProviderOptions           map[string]string
}

type RouteRequest struct {
	ProfileKey       string
	Target           Target
	ProviderKey      string
	ProviderConfigID string
}

type AcquireNumberCommand struct {
	RequestID        string
	ProfileKey       string
	ProviderKey      string
	ProviderConfigID string
	Target           Target
	LeaseDuration    time.Duration
	Labels           map[string]string
}

type ProviderAcquireRequest struct {
	RequestID     string
	Route         Route
	Target        Target
	LeaseDuration time.Duration
}

type ProviderActivation struct {
	UpstreamActivationID     string
	UpstreamOperator         string
	PhoneNumber              PhoneNumber
	Price                    Money
	AcquiredAt               time.Time
	ExpiresAt                time.Time
	CanRequestAdditionalCode bool
}

type ProviderCodeResult struct {
	Status      ActivationStatus
	Code        string
	MessageText string
	ReceivedAt  time.Time
}
