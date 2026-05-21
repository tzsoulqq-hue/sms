package app

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	smsv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/contracts/sms/v1"
	smsinternalv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/sms/private/v1"
	"github.com/byte-v-forge/sms/internal/core"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *PostgresProviderConfigStore) ensureRouteProfileSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS sms_route_profiles (
  profile_key text PRIMARY KEY,
  display_name text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  selection_strategy integer NOT NULL DEFAULT 1,
  preferred_provider_key text NOT NULL DEFAULT '',
  default_application_key text NOT NULL DEFAULT '',
  default_country_iso2 text NOT NULL DEFAULT '',
  default_country_calling_code text NOT NULL DEFAULT '',
  default_max_price_currency text NOT NULL DEFAULT '',
  default_max_price_amount text NOT NULL DEFAULT '',
  routes jsonb NOT NULL DEFAULT '[]',
  labels jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sms_route_profiles_enabled
  ON sms_route_profiles(enabled, updated_at DESC);
`)
	return err
}

func (s *PostgresProviderConfigStore) UpsertRouteProfile(ctx context.Context, input *smsinternalv1.SmsRouteProfile) (*smsinternalv1.SmsRouteProfile, error) {
	profile, err := normalizeRouteProfileForSave(input)
	if err != nil {
		return nil, err
	}
	routes, err := json.Marshal(profile.GetRoutes())
	if err != nil {
		return nil, err
	}
	labels, err := json.Marshal(profile.GetLabels())
	if err != nil {
		return nil, err
	}
	target := profile.GetDefaultTarget()
	maxPrice := target.GetMaxPrice()
	row := s.pool.QueryRow(ctx, `
INSERT INTO sms_route_profiles (
  profile_key, display_name, enabled, selection_strategy, preferred_provider_key,
  default_application_key, default_country_iso2, default_country_calling_code,
  default_max_price_currency, default_max_price_amount, routes, labels
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (profile_key) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  enabled = EXCLUDED.enabled,
  selection_strategy = EXCLUDED.selection_strategy,
  preferred_provider_key = EXCLUDED.preferred_provider_key,
  default_application_key = EXCLUDED.default_application_key,
  default_country_iso2 = EXCLUDED.default_country_iso2,
  default_country_calling_code = EXCLUDED.default_country_calling_code,
  default_max_price_currency = EXCLUDED.default_max_price_currency,
  default_max_price_amount = EXCLUDED.default_max_price_amount,
  routes = EXCLUDED.routes,
  labels = EXCLUDED.labels,
  updated_at = now()
`+selectRouteProfileSQL(),
		profile.GetProfileKey(), profile.GetDisplayName(), profile.GetEnabled(), int32(profile.GetSelectionStrategy()), profile.GetPreferredProviderKey(),
		target.GetApplicationKey(), target.GetCountryIso2(), target.GetCountryCallingCode(),
		maxPrice.GetCurrencyCode(), maxPrice.GetAmountDecimal(), routes, labels,
	)
	return scanRouteProfile(row)
}

func (s *PostgresProviderConfigStore) GetRouteProfile(ctx context.Context, profileKey string) (*smsinternalv1.SmsRouteProfile, error) {
	profileKey = strings.TrimSpace(profileKey)
	if profileKey == "" {
		return nil, core.NewError(core.CodeValidationFailed, "profile_key is required", false)
	}
	row := s.pool.QueryRow(ctx, `SELECT `+routeProfileColumns()+` FROM sms_route_profiles WHERE profile_key = $1`, profileKey)
	return scanRouteProfile(row)
}

func (s *PostgresProviderConfigStore) ListRouteProfiles(ctx context.Context, includeDisabled bool) ([]*smsinternalv1.SmsRouteProfile, error) {
	rows, err := s.pool.Query(ctx, `
SELECT `+routeProfileColumns()+`
FROM sms_route_profiles
WHERE ($1 OR enabled)
ORDER BY updated_at DESC, profile_key ASC
`, includeDisabled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := []*smsinternalv1.SmsRouteProfile{}
	for rows.Next() {
		profile, err := scanRouteProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *PostgresProviderConfigStore) DeleteRouteProfile(ctx context.Context, profileKey string) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM sms_route_profiles WHERE profile_key = $1`, strings.TrimSpace(profileKey))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return core.NewError(core.CodeRouteNotFound, "sms route profile not found", false)
	}
	return nil
}

func normalizeRouteProfileForSave(input *smsinternalv1.SmsRouteProfile) (*smsinternalv1.SmsRouteProfile, error) {
	profile := cloneRouteProfile(input)
	profile.ProfileKey = strings.TrimSpace(profile.GetProfileKey())
	if profile.GetProfileKey() == "" {
		return nil, core.NewError(core.CodeValidationFailed, "profile_key is required", false)
	}
	profile.DisplayName = strings.TrimSpace(profile.GetDisplayName())
	if profile.GetDisplayName() == "" {
		profile.DisplayName = profile.GetProfileKey()
	}
	profile.PreferredProviderKey = normalizeProviderKey(profile.GetPreferredProviderKey())
	if profile.GetSelectionStrategy() == smsinternalv1.SmsRouteSelectionStrategy_SMS_ROUTE_SELECTION_STRATEGY_UNSPECIFIED {
		profile.SelectionStrategy = smsinternalv1.SmsRouteSelectionStrategy_SMS_ROUTE_SELECTION_STRATEGY_PRIORITY
	}
	if profile.GetDefaultTarget() == nil {
		profile.DefaultTarget = &smsv1.SmsTarget{}
	}
	normalizeTarget(profile.GetDefaultTarget())
	for index, route := range profile.GetRoutes() {
		normalizeRouteCandidate(route, index)
		if err := validateRouteCandidate(route); err != nil {
			return nil, err
		}
	}
	profile.Labels = normalizeLabels(profile.GetLabels())
	return profile, nil
}

func normalizeRouteCandidate(route *smsinternalv1.SmsRouteCandidate, index int) {
	route.RouteId = strings.TrimSpace(route.GetRouteId())
	if route.GetRouteId() == "" {
		route.RouteId = "route-" + strconv.Itoa(index+1)
	}
	route.ProviderConfigId = strings.TrimSpace(route.GetProviderConfigId())
	route.ProviderKey = normalizeProviderKey(route.GetProviderKey())
	if route.GetTarget() == nil {
		route.Target = &smsv1.SmsTarget{}
	}
	normalizeTarget(route.GetTarget())
	normalizeMoney(route.GetMinPrice())
	normalizeMoney(route.GetMaxPrice())
	route.ProviderOptions = normalizeLabels(route.GetProviderOptions())
	if adapter := routeAdapterForProvider(route.GetProviderKey()); adapter != nil {
		adapter.NormalizeRouteCandidate(route)
	}
}

func validateRouteCandidate(route *smsinternalv1.SmsRouteCandidate) error {
	if !route.GetEnabled() {
		return nil
	}
	if route.GetProviderKey() == "" {
		return core.NewError(core.CodeValidationFailed, "route provider_key is required", false)
	}
	if route.GetProviderKey() != "" && !supportedProviderKey(route.GetProviderKey()) {
		return core.NewError(core.CodeUnsupportedOperation, "unsupported sms route provider", false)
	}
	return nil
}

func scanRouteProfile(row pgx.Row) (*smsinternalv1.SmsRouteProfile, error) {
	var profile routeProfileRecord
	if err := row.Scan(
		&profile.profileKey, &profile.displayName, &profile.enabled, &profile.selectionStrategy, &profile.preferredProviderKey,
		&profile.defaultApplicationKey, &profile.defaultCountryISO2, &profile.defaultCountryCalling,
		&profile.defaultMaxPriceCurrency, &profile.defaultMaxPriceAmount, &profile.routes, &profile.labels,
		&profile.createdAt, &profile.updatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.NewError(core.CodeRouteNotFound, "sms route profile not found", false)
		}
		return nil, err
	}
	return profile.toProto(), nil
}

type routeProfileRecord struct {
	profileKey              string
	displayName             string
	enabled                 bool
	selectionStrategy       int32
	preferredProviderKey    string
	defaultApplicationKey   string
	defaultCountryISO2      string
	defaultCountryCalling   string
	defaultMaxPriceCurrency string
	defaultMaxPriceAmount   string
	routes                  []byte
	labels                  []byte
	createdAt               time.Time
	updatedAt               time.Time
}

func (r routeProfileRecord) toProto() *smsinternalv1.SmsRouteProfile {
	routes := []*smsinternalv1.SmsRouteCandidate{}
	_ = json.Unmarshal(r.routes, &routes)
	labels := map[string]string{}
	_ = json.Unmarshal(r.labels, &labels)
	return &smsinternalv1.SmsRouteProfile{
		ProfileKey:           r.profileKey,
		DisplayName:          r.displayName,
		Enabled:              r.enabled,
		SelectionStrategy:    smsinternalv1.SmsRouteSelectionStrategy(r.selectionStrategy),
		PreferredProviderKey: r.preferredProviderKey,
		DefaultTarget: &smsv1.SmsTarget{
			ApplicationKey:     r.defaultApplicationKey,
			CountryIso2:        r.defaultCountryISO2,
			CountryCallingCode: r.defaultCountryCalling,
			MaxPrice: &smsv1.DecimalMoney{
				CurrencyCode:  r.defaultMaxPriceCurrency,
				AmountDecimal: r.defaultMaxPriceAmount,
			},
		},
		Routes:    routes,
		Labels:    labels,
		CreatedAt: timestamppb.New(r.createdAt),
		UpdatedAt: timestamppb.New(r.updatedAt),
	}
}

func routeProfileColumns() string {
	return `profile_key, display_name, enabled, selection_strategy, preferred_provider_key,
default_application_key, default_country_iso2, default_country_calling_code,
default_max_price_currency, default_max_price_amount, routes, labels,
created_at, updated_at`
}

func selectRouteProfileSQL() string {
	return ` RETURNING ` + routeProfileColumns()
}

func cloneRouteProfile(input *smsinternalv1.SmsRouteProfile) *smsinternalv1.SmsRouteProfile {
	if input == nil {
		return &smsinternalv1.SmsRouteProfile{}
	}
	return proto.Clone(input).(*smsinternalv1.SmsRouteProfile)
}

func normalizeMoney(money *smsv1.DecimalMoney) {
	if money == nil {
		return
	}
	money.CurrencyCode = strings.ToUpper(strings.TrimSpace(money.GetCurrencyCode()))
	money.AmountDecimal = strings.TrimSpace(money.GetAmountDecimal())
}
