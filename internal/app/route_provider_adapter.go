package app

import (
	"context"

	smsinternalv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/sms/private/v1"
	"github.com/byte-v-forge/sms/internal/core"
	"github.com/byte-v-forge/sms/internal/providers/fivesim"
	"github.com/byte-v-forge/sms/internal/providers/herosms"
	"github.com/byte-v-forge/sms/internal/providers/smsbower"
)

type routeProviderAdapter interface {
	NormalizeRouteCandidate(*smsinternalv1.SmsRouteCandidate)
	ApplyRouteCandidate(*smsinternalv1.SmsRouteCandidate, *core.Route)
}

type routeOptionProvider interface {
	Key() string
	ListRouteOptions(context.Context) (*smsinternalv1.SmsProviderRouteOptions, error)
}

func routeAdapterForProvider(providerKey string) routeProviderAdapter {
	switch normalizeProviderKey(providerKey) {
	case fivesim.ProviderKey:
		return fivesim.RouteAdapter{}
	case herosms.ProviderKey:
		return herosms.RouteAdapter{}
	case smsbower.ProviderKey:
		return smsbower.RouteAdapter{}
	default:
		return nil
	}
}

func applyRouteCandidate(candidate *smsinternalv1.SmsRouteCandidate, route *core.Route) {
	if adapter := routeAdapterForProvider(candidate.GetProviderKey()); adapter != nil {
		adapter.ApplyRouteCandidate(candidate, route)
	}
}

func listRouteOptions(ctx context.Context, provider core.Provider, config *smsinternalv1.SmsProviderConfig) (*smsinternalv1.SmsProviderRouteOptions, error) {
	options := &smsinternalv1.SmsProviderRouteOptions{ProviderKey: normalizeProviderKey(config.GetProviderKey())}
	if providerOptions, ok := provider.(routeOptionProvider); ok {
		return providerOptions.ListRouteOptions(ctx)
	}
	return options, nil
}
