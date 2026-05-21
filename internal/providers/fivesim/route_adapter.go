package fivesim

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
	if product := strings.TrimSpace(candidate.GetUpstreamServiceKey()); product != "" {
		route.UpstreamServiceKey = product
	}
	if country := strings.TrimSpace(candidate.GetProviderCountryId()); country != "" {
		route.ProviderCountryID = country
	}
	route.MinPrice = moneyFromProto(candidate.GetMinPrice())
	if candidate.GetMaxPrice() != nil {
		route.MaxPrice = moneyFromProto(candidate.GetMaxPrice())
	}
	mergeOptions(route, candidate.GetProviderOptions())
	if operator := strings.TrimSpace(candidate.GetProviderOptions()["operator"]); operator != "" {
		route.IncludeUpstreamProviderID = []string{operator}
	}
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

func moneyFromProto(value *smsv1.DecimalMoney) core.Money {
	if value == nil {
		return core.Money{}
	}
	return core.Money{CurrencyCode: value.GetCurrencyCode(), AmountDecimal: value.GetAmountDecimal()}
}

func (c *Client) ListRouteOptions(ctx context.Context) (*smsinternalv1.SmsProviderRouteOptions, error) {
	countries, err := c.ListCountries(ctx)
	options := &smsinternalv1.SmsProviderRouteOptions{ProviderKey: ProviderKey}
	for _, country := range countries {
		options.Countries = append(options.Countries, &smsinternalv1.SmsRouteOption{
			Value: country.CountryID,
			Label: firstNonEmpty(country.Name, country.CountryID),
			Metadata: map[string]string{
				"country_iso2":         country.CountryISO2,
				"country_calling_code": country.CountryCallingCode,
			},
		})
	}
	offers, offerErr := c.ListPriceOffers(ctx, "", "")
	addedServices := map[string]bool{}
	addedOperators := map[string]bool{"any": true}
	options.Operators = []*smsinternalv1.SmsRouteOption{{Value: "any", Label: "Any"}}
	for _, offer := range offers {
		if service := strings.TrimSpace(offer.UpstreamServiceKey); service != "" && !addedServices[service] {
			addedServices[service] = true
			options.Services = append(options.Services, &smsinternalv1.SmsRouteOption{
				Value:          service,
				Label:          service,
				Price:          &smsv1.DecimalMoney{CurrencyCode: offer.Price.CurrencyCode, AmountDecimal: offer.Price.AmountDecimal},
				AvailableCount: int32(offer.AvailableCount),
			})
		}
		if operator := strings.TrimSpace(offer.Operator); operator != "" && !addedOperators[operator] {
			addedOperators[operator] = true
			options.Operators = append(options.Operators, &smsinternalv1.SmsRouteOption{Value: operator, Label: operator})
		}
	}
	if err != nil {
		return options, err
	}
	if offerErr != nil {
		return options, offerErr
	}
	return options, nil
}
