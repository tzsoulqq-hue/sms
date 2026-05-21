package app

import (
	"context"
	"strings"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type StaticRouteResolver struct {
	routes []core.Route
}

func NewStaticRouteResolver(routes []core.Route) *StaticRouteResolver {
	return &StaticRouteResolver{routes: append([]core.Route(nil), routes...)}
}

func (r *StaticRouteResolver) Resolve(_ context.Context, request core.RouteRequest) (core.Route, error) {
	for _, route := range r.routes {
		if !sameFoldOrEmpty(request.ProviderKey, route.ProviderKey) {
			continue
		}
		if !sameFoldOrEmpty(route.ApplicationKey, request.Target.ApplicationKey) {
			continue
		}
		if !sameFoldOrEmpty(route.CountryISO2, request.Target.CountryISO2) {
			continue
		}
		if route.CountryCallingCode != "" && request.Target.CountryCallingCode != "" && route.CountryCallingCode != request.Target.CountryCallingCode {
			continue
		}
		return route, nil
	}
	return core.Route{}, core.NewError(core.CodeRouteNotFound, "no sms route for target", false)
}

func sameFoldOrEmpty(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)
	return expected == "" || actual == "" || strings.EqualFold(expected, actual)
}
