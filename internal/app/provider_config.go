package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	smsv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/contracts/sms/v1"
	smsinternalv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/sms/private/v1"
	"github.com/byte-v-forge/sms/internal/core"
	"github.com/byte-v-forge/sms/internal/providers/fivesim"
	"github.com/byte-v-forge/sms/internal/providers/herosms"
	"github.com/byte-v-forge/sms/internal/providers/smsbower"
)

type ProviderConfigStore interface {
	UpsertProviderConfig(context.Context, *smsinternalv1.SmsProviderConfig) (*smsinternalv1.SmsProviderConfig, error)
	GetProviderConfig(context.Context, string) (*smsinternalv1.SmsProviderConfig, error)
	ListProviderConfigs(context.Context, bool, string) ([]*smsinternalv1.SmsProviderConfig, error)
	DeleteProviderConfig(context.Context, string) error
	GetEnabledProviderConfig(context.Context, string, core.Target) (*smsinternalv1.SmsProviderConfig, error)
}

type ActivationListStore interface {
	List(context.Context, bool, int) ([]core.Activation, error)
}

type ProviderConfigRouteResolver struct {
	configs ProviderConfigStore
}

func NewProviderConfigRouteResolver(configs ProviderConfigStore) *ProviderConfigRouteResolver {
	return &ProviderConfigRouteResolver{configs: configs}
}

func (r *ProviderConfigRouteResolver) Resolve(ctx context.Context, request core.RouteRequest) (core.Route, error) {
	var config *smsinternalv1.SmsProviderConfig
	var err error
	if strings.TrimSpace(request.ProviderConfigID) != "" {
		config, err = r.configs.GetProviderConfig(ctx, request.ProviderConfigID)
		if err == nil {
			err = validateRequestedProviderConfig(config, request)
		}
	} else {
		config, err = r.configs.GetEnabledProviderConfig(ctx, request.ProviderKey, request.Target)
	}
	if err != nil {
		return core.Route{}, err
	}
	return routeFromProviderConfig(config, request.Target), nil
}

type ConfiguredProvider struct {
	key        string
	configs    ProviderConfigStore
	timeout    time.Duration
	activation sync.Map
}

func NewConfiguredProvider(key string, configs ProviderConfigStore, timeout time.Duration) *ConfiguredProvider {
	return &ConfiguredProvider{key: normalizeProviderKey(key), configs: configs, timeout: timeout}
}

func (p *ConfiguredProvider) Key() string {
	return p.key
}

func (p *ConfiguredProvider) Policy() core.ProviderPolicy {
	return core.ProviderPolicy{ActivationTTL: 20 * time.Minute, PollInterval: 5 * time.Second, CancelAllowedAfter: 2 * time.Minute}
}

func (p *ConfiguredProvider) AcquireNumber(ctx context.Context, request core.ProviderAcquireRequest) (core.ProviderActivation, error) {
	config, err := p.configForRoute(ctx, request.Route, request.Target)
	if err != nil {
		return core.ProviderActivation{}, err
	}
	provider, err := providerFromConfig(config, p.timeout)
	if err != nil {
		return core.ProviderActivation{}, err
	}
	activation, err := provider.AcquireNumber(ctx, request)
	if err == nil && activation.UpstreamActivationID != "" {
		p.activation.Store(activation.UpstreamActivationID, config.GetProviderConfigId())
	}
	return activation, err
}

func (p *ConfiguredProvider) GetStatus(ctx context.Context, upstreamActivationID string) (core.ProviderCodeResult, error) {
	provider, err := p.providerForActivation(ctx, upstreamActivationID)
	if err != nil {
		return core.ProviderCodeResult{}, err
	}
	return provider.GetStatus(ctx, upstreamActivationID)
}

func (p *ConfiguredProvider) SetStatus(ctx context.Context, upstreamActivationID string, action core.ProviderAction) error {
	provider, err := p.providerForActivation(ctx, upstreamActivationID)
	if err != nil {
		return err
	}
	return provider.SetStatus(ctx, upstreamActivationID, action)
}

func (p *ConfiguredProvider) GetBalance(ctx context.Context) (core.Money, error) {
	config, err := p.configs.GetEnabledProviderConfig(ctx, p.key, core.Target{})
	if err != nil {
		return core.Money{}, err
	}
	provider, err := providerFromConfig(config, p.timeout)
	if err != nil {
		return core.Money{}, err
	}
	return provider.GetBalance(ctx)
}

func (p *ConfiguredProvider) configForRoute(ctx context.Context, route core.Route, target core.Target) (*smsinternalv1.SmsProviderConfig, error) {
	configID := strings.TrimSpace(route.ProviderOptions["provider_config_id"])
	if configID != "" {
		return p.configs.GetProviderConfig(ctx, configID)
	}
	return p.configs.GetEnabledProviderConfig(ctx, p.key, target)
}

func (p *ConfiguredProvider) providerForActivation(ctx context.Context, upstreamActivationID string) (core.Provider, error) {
	if value, ok := p.activation.Load(upstreamActivationID); ok {
		if configID, ok := value.(string); ok && strings.TrimSpace(configID) != "" {
			config, err := p.configs.GetProviderConfig(ctx, configID)
			if err != nil {
				return nil, err
			}
			return providerFromConfig(config, p.timeout)
		}
	}
	config, err := p.configs.GetEnabledProviderConfig(ctx, p.key, core.Target{})
	if err != nil {
		return nil, err
	}
	return providerFromConfig(config, p.timeout)
}

func providerFromConfig(config *smsinternalv1.SmsProviderConfig, timeout time.Duration) (core.Provider, error) {
	if config == nil {
		return nil, core.NewError(core.CodeRouteNotFound, "sms provider config not found", false)
	}
	client, err := httpClientFromConfig(config, timeout)
	if err != nil {
		return nil, err
	}
	switch normalizeProviderKey(config.GetProviderKey()) {
	case fivesim.ProviderKey:
		return fivesim.New(fivesim.Config{
			Endpoint:     config.GetApiEndpoint(),
			Token:        config.GetCredentialSecret(),
			CurrencyCode: firstLabel(config.GetLabels(), "currency", "currency_code"),
		}, client)
	case herosms.ProviderKey:
		return herosms.New(herosms.Config{Endpoint: config.GetApiEndpoint(), APIKey: config.GetCredentialSecret()}, client)
	case smsbower.ProviderKey:
		return smsbower.New(smsbower.Config{
			Endpoint: config.GetApiEndpoint(),
			APIKey:   config.GetCredentialSecret(),
			Ref:      firstLabel(config.GetLabels(), "ref"),
			UserID:   firstLabel(config.GetLabels(), "userID", "user_id"),
		}, client)
	default:
		return nil, core.NewError(core.CodeUnsupportedOperation, fmt.Sprintf("unsupported sms provider %q", config.GetProviderKey()), false)
	}
}

func routeFromProviderConfig(config *smsinternalv1.SmsProviderConfig, requestTarget core.Target) core.Route {
	defaultTarget := targetFromProto(config.GetDefaultTarget())
	target := mergeRouteTarget(requestTarget, defaultTarget)
	options := cloneMap(config.GetLabels())
	if options == nil {
		options = map[string]string{}
	}
	options["provider_config_id"] = config.GetProviderConfigId()
	return core.Route{
		ProviderKey:        normalizeProviderKey(config.GetProviderKey()),
		ApplicationKey:     target.ApplicationKey,
		UpstreamServiceKey: firstNonEmptyString(config.GetUpstreamServiceKey(), target.ApplicationKey),
		CountryISO2:        target.CountryISO2,
		CountryCallingCode: target.CountryCallingCode,
		ProviderCountryID:  config.GetProviderCountryId(),
		MaxPrice:           target.MaxPrice,
		ProviderOptions:    options,
	}
}

func httpClientFromConfig(config *smsinternalv1.SmsProviderConfig, timeout time.Duration) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy := strings.TrimSpace(config.GetHttpProxy()); proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, core.NewError(core.CodeValidationFailed, "invalid sms provider proxy", false)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func moneyFromProto(value *smsv1.DecimalMoney) core.Money {
	if value == nil {
		return core.Money{}
	}
	return core.Money{CurrencyCode: value.GetCurrencyCode(), AmountDecimal: value.GetAmountDecimal()}
}

func targetFromProto(value *smsv1.SmsTarget) core.Target {
	if value == nil {
		return core.Target{}
	}
	return core.Target{
		ApplicationKey:     strings.TrimSpace(value.GetApplicationKey()),
		CountryISO2:        strings.ToUpper(strings.TrimSpace(value.GetCountryIso2())),
		CountryCallingCode: strings.TrimPrefix(strings.TrimSpace(value.GetCountryCallingCode()), "+"),
		MaxPrice:           moneyFromProto(value.GetMaxPrice()),
	}
}

func mergeRouteTarget(primary core.Target, fallback core.Target) core.Target {
	if primary.ApplicationKey == "" {
		primary.ApplicationKey = fallback.ApplicationKey
	}
	if primary.CountryISO2 == "" {
		primary.CountryISO2 = fallback.CountryISO2
	}
	if primary.CountryCallingCode == "" {
		primary.CountryCallingCode = fallback.CountryCallingCode
	}
	if primary.MaxPrice.AmountDecimal == "" && primary.MaxPrice.CurrencyCode == "" {
		primary.MaxPrice = fallback.MaxPrice
	}
	return primary
}

func firstLabel(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(labels[key]); value != "" {
			return value
		}
	}
	return ""
}

func normalizeProviderKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func validateRequestedProviderConfig(config *smsinternalv1.SmsProviderConfig, request core.RouteRequest) error {
	if config == nil || !config.GetEnabled() {
		return core.NewError(core.CodeRouteNotFound, "sms provider config is disabled or missing", false)
	}
	if providerKey := normalizeProviderKey(request.ProviderKey); providerKey != "" && providerKey != normalizeProviderKey(config.GetProviderKey()) {
		return core.NewError(core.CodeRouteNotFound, "sms provider config does not match requested provider", false)
	}
	if targetMatchScore(config.GetDefaultTarget(), request.Target) < 0 {
		return core.NewError(core.CodeRouteNotFound, "sms provider config does not match requested target", false)
	}
	return nil
}
