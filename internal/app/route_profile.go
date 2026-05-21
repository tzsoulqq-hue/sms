package app

import (
	"math"
	"strconv"
	"strings"

	smsinternalv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/sms/private/v1"
	"github.com/byte-v-forge/sms/internal/core"
)

func selectRouteCandidate(profile *smsinternalv1.SmsRouteProfile, request core.RouteRequest) (*smsinternalv1.SmsRouteCandidate, core.Target, error) {
	if profile == nil || !profile.GetEnabled() {
		return nil, core.Target{}, core.NewError(core.CodeRouteNotFound, "sms route profile is disabled or missing", false)
	}
	baseTarget := mergeRouteTarget(request.Target, targetFromProto(profile.GetDefaultTarget()))
	var best *smsinternalv1.SmsRouteCandidate
	bestTarget := core.Target{}
	bestRank := routeRank{priority: math.MaxInt32, price: math.Inf(1)}
	for _, candidate := range profile.GetRoutes() {
		target := mergeRouteTarget(targetFromProto(candidate.GetTarget()), baseTarget)
		if !routeCandidateMatches(profile, candidate, target, request) {
			continue
		}
		rank := rankRouteCandidate(profile.GetSelectionStrategy(), candidate, target)
		if rank.betterThan(bestRank) {
			best = candidate
			bestTarget = target
			bestRank = rank
		}
	}
	if best == nil {
		return nil, core.Target{}, core.NewError(core.CodeRouteNotFound, "sms route profile has no matching route", false)
	}
	return best, bestTarget, nil
}

func routeCandidateMatches(profile *smsinternalv1.SmsRouteProfile, candidate *smsinternalv1.SmsRouteCandidate, target core.Target, request core.RouteRequest) bool {
	if candidate == nil || !candidate.GetEnabled() || strings.TrimSpace(candidate.GetProviderKey()) == "" {
		return false
	}
	if request.ProviderKey != "" && !strings.EqualFold(candidate.GetProviderKey(), request.ProviderKey) {
		return false
	}
	if profile.GetSelectionStrategy() == smsinternalv1.SmsRouteSelectionStrategy_SMS_ROUTE_SELECTION_STRATEGY_SPECIFIED_PROVIDER {
		preferred := strings.TrimSpace(profile.GetPreferredProviderKey())
		if preferred != "" && !strings.EqualFold(candidate.GetProviderKey(), preferred) {
			return false
		}
	}
	return targetMatchesRequest(target, request.Target)
}

func targetMatchesRequest(target core.Target, request core.Target) bool {
	if request.ApplicationKey != "" && target.ApplicationKey != "" && !strings.EqualFold(target.ApplicationKey, request.ApplicationKey) {
		return false
	}
	if request.CountryISO2 != "" && target.CountryISO2 != "" && !strings.EqualFold(target.CountryISO2, request.CountryISO2) {
		return false
	}
	if request.CountryCallingCode != "" && target.CountryCallingCode != "" && target.CountryCallingCode != request.CountryCallingCode {
		return false
	}
	return true
}

type routeRank struct {
	priority int
	price    float64
}

func rankRouteCandidate(strategy smsinternalv1.SmsRouteSelectionStrategy, candidate *smsinternalv1.SmsRouteCandidate, target core.Target) routeRank {
	price := routePrice(candidate, target)
	priority := int(candidate.GetPriority())
	if priority <= 0 {
		priority = math.MaxInt32
	}
	if strategy == smsinternalv1.SmsRouteSelectionStrategy_SMS_ROUTE_SELECTION_STRATEGY_LOWEST_PRICE {
		return routeRank{priority: pricePriority(price), price: float64(priority)}
	}
	return routeRank{priority: priority, price: price}
}

func pricePriority(price float64) int {
	if math.IsInf(price, 1) || price < 0 {
		return math.MaxInt32
	}
	return int(math.Round(price * 1000000))
}

func (rank routeRank) betterThan(other routeRank) bool {
	if rank.priority != other.priority {
		return rank.priority < other.priority
	}
	return rank.price < other.price
}

func routePrice(candidate *smsinternalv1.SmsRouteCandidate, target core.Target) float64 {
	for _, money := range []core.Money{moneyFromProto(candidate.GetMaxPrice()), target.MaxPrice} {
		if price, err := strconv.ParseFloat(strings.TrimSpace(money.AmountDecimal), 64); err == nil {
			return price
		}
	}
	return math.Inf(1)
}
