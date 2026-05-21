package herosms

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
}

func moneyFromProto(value *smsv1.DecimalMoney) core.Money {
	if value == nil {
		return core.Money{}
	}
	return core.Money{CurrencyCode: value.GetCurrencyCode(), AmountDecimal: value.GetAmountDecimal()}
}

func (c *Client) ListRouteOptions(context.Context) (*smsinternalv1.SmsProviderRouteOptions, error) {
	return &smsinternalv1.SmsProviderRouteOptions{ProviderKey: ProviderKey}, nil
}
