package smsbower

import (
	"context"
	"strings"

	smsv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/contracts/sms/v1"
	smsinternalv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/sms/private/v1"
	"github.com/byte-v-forge/sms/internal/core"
)

type RouteAdapter struct{}

func (RouteAdapter) NormalizeRouteCandidate(route *smsinternalv1.SmsRouteCandidate) {
	route.UpstreamServiceKey = strings.TrimSpace(route.GetUpstreamServiceKey())
	route.ProviderCountryId = strings.TrimSpace(route.GetProviderCountryId())
	route.ProviderOptions = normalizeOptions(route.GetProviderOptions())
}

func (RouteAdapter) ApplyRouteCandidate(candidate *smsinternalv1.SmsRouteCandidate, route *core.Route) {
	if service := strings.TrimSpace(candidate.GetUpstreamServiceKey()); service != "" {
		route.UpstreamServiceKey = service
	}
	if country := strings.TrimSpace(candidate.GetProviderCountryId()); country != "" {
		route.ProviderCountryID = country
	}
	route.MinPrice = moneyFromProto(candidate.GetMinPrice())
	if candidate.GetMaxPrice() != nil {
		route.MaxPrice = moneyFromProto(candidate.GetMaxPrice())
	}
	mergeOptions(route, candidate.GetProviderOptions())
	route.IncludeUpstreamProviderID = splitCSV(candidate.GetProviderOptions()["include_provider_ids"])
	route.ExcludeUpstreamProviderID = splitCSV(candidate.GetProviderOptions()["exclude_provider_ids"])
	route.ExcludedPhonePrefixes = splitCSV(candidate.GetProviderOptions()["phone_exception_prefixes"])
}

func mergeOptions(route *core.Route, options map[string]string) {
	if route.ProviderOptions == nil {
		route.ProviderOptions = map[string]string{}
	}
	for key, value := range options {
		route.ProviderOptions[key] = value
	}
}

func normalizeOptions(options map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range options {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func moneyFromProto(value *smsv1.DecimalMoney) core.Money {
	if value == nil {
		return core.Money{}
	}
	return core.Money{CurrencyCode: value.GetCurrencyCode(), AmountDecimal: value.GetAmountDecimal()}
}

func (c *Client) ListRouteOptions(ctx context.Context) (*smsinternalv1.SmsProviderRouteOptions, error) {
	applications, appErr := c.ListApplications(ctx)
	countries, countryErr := c.ListCountries(ctx)
	options := &smsinternalv1.SmsProviderRouteOptions{ProviderKey: ProviderKey}
	for _, app := range applications {
		options.Services = append(options.Services, &smsinternalv1.SmsRouteOption{Value: app.UpstreamServiceKey, Label: firstNonEmpty(app.DisplayName, app.UpstreamServiceKey)})
	}
	for _, country := range countries {
		options.Countries = append(options.Countries, &smsinternalv1.SmsRouteOption{Value: country.CountryID, Label: firstNonEmpty(country.Name, country.CountryID)})
	}
	if appErr != nil {
		return options, appErr
	}
	if countryErr != nil {
		return options, countryErr
	}
	return options, nil
}
