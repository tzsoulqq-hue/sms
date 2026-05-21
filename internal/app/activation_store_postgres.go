package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresActivationStore struct {
	pool *pgxpool.Pool
}

func NewPostgresActivationStore(ctx context.Context, dsn string) (*PostgresActivationStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("SMS_PG_DSN or PG_DSN is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	store := &PostgresActivationStore{pool: pool}
	if err := store.EnsureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresActivationStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *PostgresActivationStore) EnsureSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS sms_activations (
  activation_id text PRIMARY KEY,
  request_id text NOT NULL DEFAULT '',
  provider_config_id text NOT NULL DEFAULT '',
  provider_key text NOT NULL DEFAULT '',
  upstream_activation_id text NOT NULL DEFAULT '',
  upstream_operator text NOT NULL DEFAULT '',
  target_application_key text NOT NULL DEFAULT '',
  target_country_iso2 text NOT NULL DEFAULT '',
  target_country_calling_code text NOT NULL DEFAULT '',
  target_max_price_currency text NOT NULL DEFAULT '',
  target_max_price_amount text NOT NULL DEFAULT '',
  phone_e164 text NOT NULL DEFAULT '',
  phone_national text NOT NULL DEFAULT '',
  phone_country_iso2 text NOT NULL DEFAULT '',
  phone_country_calling_code text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT '',
  price_currency text NOT NULL DEFAULT '',
  price_amount text NOT NULL DEFAULT '',
  acquired_at timestamptz,
  expires_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  cancel_allowed_at timestamptz,
  code_value text NOT NULL DEFAULT '',
  code_message_text text NOT NULL DEFAULT '',
  code_received_at timestamptz,
  can_request_additional_code boolean NOT NULL DEFAULT false,
  labels jsonb NOT NULL DEFAULT '{}',
  last_error_code text NOT NULL DEFAULT '',
  last_error_message text NOT NULL DEFAULT '',
  last_error_retryable boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sms_activations_status_updated
  ON sms_activations(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sms_activations_provider_upstream
  ON sms_activations(provider_key, upstream_activation_id);
`)
	return err
}

func (s *PostgresActivationStore) Save(ctx context.Context, activation core.Activation) error {
	labels, err := json.Marshal(activation.Labels)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO sms_activations (
  activation_id, request_id, provider_config_id, provider_key, upstream_activation_id, upstream_operator,
  target_application_key, target_country_iso2, target_country_calling_code, target_max_price_currency, target_max_price_amount,
  phone_e164, phone_national, phone_country_iso2, phone_country_calling_code,
  status, price_currency, price_amount, acquired_at, expires_at, updated_at, cancel_allowed_at,
  code_value, code_message_text, code_received_at, can_request_additional_code, labels,
  last_error_code, last_error_message, last_error_retryable
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
  $11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
  $21,$22,$23,$24,$25,$26,$27,$28,$29,$30
)
ON CONFLICT (activation_id) DO UPDATE SET
  request_id = EXCLUDED.request_id,
  provider_config_id = EXCLUDED.provider_config_id,
  provider_key = EXCLUDED.provider_key,
  upstream_activation_id = EXCLUDED.upstream_activation_id,
  upstream_operator = EXCLUDED.upstream_operator,
  target_application_key = EXCLUDED.target_application_key,
  target_country_iso2 = EXCLUDED.target_country_iso2,
  target_country_calling_code = EXCLUDED.target_country_calling_code,
  target_max_price_currency = EXCLUDED.target_max_price_currency,
  target_max_price_amount = EXCLUDED.target_max_price_amount,
  phone_e164 = EXCLUDED.phone_e164,
  phone_national = EXCLUDED.phone_national,
  phone_country_iso2 = EXCLUDED.phone_country_iso2,
  phone_country_calling_code = EXCLUDED.phone_country_calling_code,
  status = EXCLUDED.status,
  price_currency = EXCLUDED.price_currency,
  price_amount = EXCLUDED.price_amount,
  acquired_at = EXCLUDED.acquired_at,
  expires_at = EXCLUDED.expires_at,
  updated_at = EXCLUDED.updated_at,
  cancel_allowed_at = EXCLUDED.cancel_allowed_at,
  code_value = EXCLUDED.code_value,
  code_message_text = EXCLUDED.code_message_text,
  code_received_at = EXCLUDED.code_received_at,
  can_request_additional_code = EXCLUDED.can_request_additional_code,
  labels = EXCLUDED.labels,
  last_error_code = EXCLUDED.last_error_code,
  last_error_message = EXCLUDED.last_error_message,
  last_error_retryable = EXCLUDED.last_error_retryable
`, activationValues(activation, labels)...)
	return err
}

func (s *PostgresActivationStore) Get(ctx context.Context, activationID string) (core.Activation, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+activationColumns()+` FROM sms_activations WHERE activation_id = $1`, strings.TrimSpace(activationID))
	return scanActivation(row)
}

func (s *PostgresActivationStore) Update(ctx context.Context, activation core.Activation) error {
	labels, err := json.Marshal(activation.Labels)
	if err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `
UPDATE sms_activations SET
  request_id = $2,
  provider_config_id = $3,
  provider_key = $4,
  upstream_activation_id = $5,
  upstream_operator = $6,
  target_application_key = $7,
  target_country_iso2 = $8,
  target_country_calling_code = $9,
  target_max_price_currency = $10,
  target_max_price_amount = $11,
  phone_e164 = $12,
  phone_national = $13,
  phone_country_iso2 = $14,
  phone_country_calling_code = $15,
  status = $16,
  price_currency = $17,
  price_amount = $18,
  acquired_at = $19,
  expires_at = $20,
  updated_at = $21,
  cancel_allowed_at = $22,
  code_value = $23,
  code_message_text = $24,
  code_received_at = $25,
  can_request_additional_code = $26,
  labels = $27,
  last_error_code = $28,
  last_error_message = $29,
  last_error_retryable = $30
WHERE activation_id = $1
`, activationValues(activation, labels)...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return core.NewError(core.CodeActivationNotFound, "activation not found", false)
	}
	return nil
}

func (s *PostgresActivationStore) List(ctx context.Context, includeFinal bool, limit int) ([]core.Activation, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
SELECT `+activationColumns()+`
FROM sms_activations
WHERE $1 OR status NOT IN ('completed', 'canceled', 'expired', 'failed')
ORDER BY updated_at DESC, activation_id ASC
LIMIT $2
`, includeFinal, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []core.Activation{}
	for rows.Next() {
		activation, err := scanActivation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, activation)
	}
	return out, rows.Err()
}

func activationValues(activation core.Activation, labels []byte) []any {
	codeValue, codeMessage, codeReceivedAt := codeFields(activation.Code)
	errorCode, errorMessage, errorRetryable := errorFields(activation.LastError)
	return []any{
		activation.ID,
		activation.RequestID,
		activation.ProviderConfigID,
		activation.ProviderKey,
		activation.UpstreamActivationID,
		activation.UpstreamOperator,
		activation.Target.ApplicationKey,
		activation.Target.CountryISO2,
		activation.Target.CountryCallingCode,
		activation.Target.MaxPrice.CurrencyCode,
		activation.Target.MaxPrice.AmountDecimal,
		activation.PhoneNumber.E164,
		activation.PhoneNumber.NationalNumber,
		activation.PhoneNumber.CountryISO2,
		activation.PhoneNumber.CountryCallingCode,
		string(activation.Status),
		activation.Price.CurrencyCode,
		activation.Price.AmountDecimal,
		timeOrNil(activation.AcquiredAt),
		timeOrNil(activation.ExpiresAt),
		timeOrNil(activation.UpdatedAt),
		timeOrNil(activation.CancelAllowedAt),
		codeValue,
		codeMessage,
		codeReceivedAt,
		activation.CanRequestAdditionalCode,
		labels,
		errorCode,
		errorMessage,
		errorRetryable,
	}
}

func activationColumns() string {
	return `activation_id, request_id, provider_config_id, provider_key, upstream_activation_id, upstream_operator,
target_application_key, target_country_iso2, target_country_calling_code, target_max_price_currency, target_max_price_amount,
phone_e164, phone_national, phone_country_iso2, phone_country_calling_code,
status, price_currency, price_amount, acquired_at, expires_at, updated_at, cancel_allowed_at,
code_value, code_message_text, code_received_at, can_request_additional_code, labels,
last_error_code, last_error_message, last_error_retryable`
}

func scanActivation(row pgx.Row) (core.Activation, error) {
	var record activationRecord
	if err := row.Scan(
		&record.id, &record.requestID, &record.providerConfigID, &record.providerKey, &record.upstreamActivationID, &record.upstreamOperator,
		&record.targetApplicationKey, &record.targetCountryISO2, &record.targetCallingCode, &record.targetMaxPriceCurrency, &record.targetMaxPriceAmount,
		&record.phoneE164, &record.phoneNational, &record.phoneCountryISO2, &record.phoneCallingCode,
		&record.status, &record.priceCurrency, &record.priceAmount, &record.acquiredAt, &record.expiresAt, &record.updatedAt, &record.cancelAllowedAt,
		&record.codeValue, &record.codeMessageText, &record.codeReceivedAt, &record.canRequestAdditionalCode, &record.labels,
		&record.lastErrorCode, &record.lastErrorMessage, &record.lastErrorRetryable,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Activation{}, core.NewError(core.CodeActivationNotFound, "activation not found", false)
		}
		return core.Activation{}, err
	}
	return record.toCore(), nil
}

type activationRecord struct {
	id                       string
	requestID                string
	providerConfigID         string
	providerKey              string
	upstreamActivationID     string
	upstreamOperator         string
	targetApplicationKey     string
	targetCountryISO2        string
	targetCallingCode        string
	targetMaxPriceCurrency   string
	targetMaxPriceAmount     string
	phoneE164                string
	phoneNational            string
	phoneCountryISO2         string
	phoneCallingCode         string
	status                   string
	priceCurrency            string
	priceAmount              string
	acquiredAt               sql.NullTime
	expiresAt                sql.NullTime
	updatedAt                time.Time
	cancelAllowedAt          sql.NullTime
	codeValue                string
	codeMessageText          string
	codeReceivedAt           sql.NullTime
	canRequestAdditionalCode bool
	labels                   []byte
	lastErrorCode            string
	lastErrorMessage         string
	lastErrorRetryable       bool
}

func (r activationRecord) toCore() core.Activation {
	labels := map[string]string{}
	_ = json.Unmarshal(r.labels, &labels)
	return core.Activation{
		ID:                   r.id,
		RequestID:            r.requestID,
		ProviderConfigID:     r.providerConfigID,
		ProviderKey:          r.providerKey,
		UpstreamActivationID: r.upstreamActivationID,
		UpstreamOperator:     r.upstreamOperator,
		Target: core.Target{
			ApplicationKey:     r.targetApplicationKey,
			CountryISO2:        r.targetCountryISO2,
			CountryCallingCode: r.targetCallingCode,
			MaxPrice: core.Money{
				CurrencyCode:  r.targetMaxPriceCurrency,
				AmountDecimal: r.targetMaxPriceAmount,
			},
		},
		PhoneNumber: core.PhoneNumber{
			E164:               r.phoneE164,
			NationalNumber:     r.phoneNational,
			CountryISO2:        r.phoneCountryISO2,
			CountryCallingCode: r.phoneCallingCode,
		},
		Status:                   core.ActivationStatus(r.status),
		Price:                    core.Money{CurrencyCode: r.priceCurrency, AmountDecimal: r.priceAmount},
		AcquiredAt:               nullableTime(r.acquiredAt),
		ExpiresAt:                nullableTime(r.expiresAt),
		UpdatedAt:                r.updatedAt,
		CancelAllowedAt:          nullableTime(r.cancelAllowedAt),
		Code:                     codeFromFields(r.codeValue, r.codeMessageText, r.codeReceivedAt),
		CanRequestAdditionalCode: r.canRequestAdditionalCode,
		Labels:                   labels,
		LastError:                errorFromFields(r.lastErrorCode, r.lastErrorMessage, r.lastErrorRetryable),
	}
}

func timeOrNil(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullableTime(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func codeFields(code *core.SMSCode) (string, string, any) {
	if code == nil {
		return "", "", nil
	}
	return code.Value, code.MessageText, timeOrNil(code.ReceivedAt)
}

func codeFromFields(value string, messageText string, receivedAt sql.NullTime) *core.SMSCode {
	if value == "" && messageText == "" && !receivedAt.Valid {
		return nil
	}
	return &core.SMSCode{Value: value, MessageText: messageText, ReceivedAt: nullableTime(receivedAt)}
}

func errorFields(err *core.Error) (string, string, bool) {
	if err == nil {
		return "", "", false
	}
	return string(err.Code), err.Message, err.Retryable
}

func errorFromFields(code string, message string, retryable bool) *core.Error {
	if code == "" && message == "" {
		return nil
	}
	return &core.Error{Code: core.ErrorCode(code), Message: message, Retryable: retryable}
}
