package smsbower

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
	"github.com/byte-v-forge/sms/internal/providers/handlerapi"
	"github.com/byte-v-forge/sms/internal/providers/phone"
)

const (
	DefaultEndpoint                   = "https://smsbower.page/stubs/handler_api.php"
	ProviderKey                       = "smsbower"
	defaultMinimumStockForLowestPrice = 5
)

type Config struct {
	Endpoint string
	APIKey   string
	Ref      string
	UserID   string
}

type Client struct {
	api    *handlerapi.Client
	ref    string
	userID string
	policy core.ProviderPolicy
}

func New(config Config, httpClient handlerapi.HTTPDoer) (*Client, error) {
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	api, err := handlerapi.New(endpoint, config.APIKey, httpClient)
	if err != nil {
		return nil, err
	}
	return &Client{
		api:    api,
		ref:    config.Ref,
		userID: config.UserID,
		policy: core.ProviderPolicy{
			ActivationTTL:         25 * time.Minute,
			PollInterval:          5 * time.Second,
			EarlyCancelRetryAfter: 2 * time.Minute,
		},
	}, nil
}

func (c *Client) Key() string {
	return ProviderKey
}

func (c *Client) Policy() core.ProviderPolicy {
	return c.policy
}

func (c *Client) AcquireNumber(ctx context.Context, request core.ProviderAcquireRequest) (core.ProviderActivation, error) {
	service, err := c.serviceForRoute(ctx, request)
	if err != nil {
		return core.ProviderActivation{}, err
	}
	country, err := c.countryForRoute(ctx, request)
	if err != nil {
		return core.ProviderActivation{}, err
	}
	request.Route.UpstreamServiceKey = service
	request.Route.ProviderCountryID = country
	request.Route.IncludeUpstreamProviderID = c.effectiveProviderIDs(ctx, request)
	activation, err := c.acquireNumberOnce(ctx, request)
	if err == nil || len(request.Route.IncludeUpstreamProviderID) == 0 || !isNoNumberError(err) {
		return activation, err
	}
	request.Route.IncludeUpstreamProviderID = nil
	return c.acquireNumberOnce(ctx, request)
}

func (c *Client) acquireNumberOnce(ctx context.Context, request core.ProviderAcquireRequest) (core.ProviderActivation, error) {
	if c.requiresGetNumber(request) {
		params := c.acquireParams(request, false)
		result, err := c.api.Do(ctx, "getNumber", params)
		if err != nil {
			return core.ProviderActivation{}, err
		}
		activation, err := parseAccessNumber(result, request)
		if err == nil {
			return activation, nil
		}
		return core.ProviderActivation{}, handlerapi.MapTextError(result)
	}

	params := c.acquireParams(request, true)
	result, err := c.api.Do(ctx, "getNumberV2", params)
	if err != nil {
		return core.ProviderActivation{}, err
	}
	activation, err := c.parseGetNumberV2(result, request)
	if err == nil {
		return activation, nil
	}
	if isProviderTextError(result) {
		return core.ProviderActivation{}, handlerapi.MapTextError(result)
	}
	return core.ProviderActivation{}, err
}

func isNoNumberError(err error) bool {
	var coreErr *core.Error
	if errors.As(err, &coreErr) {
		return coreErr.Code == core.CodeNoNumberAvailable
	}
	return strings.Contains(strings.ToLower(err.Error()), "no upstream number available")
}

func (c *Client) effectiveProviderIDs(ctx context.Context, request core.ProviderAcquireRequest) []string {
	if len(request.Route.IncludeUpstreamProviderID) > 0 {
		return request.Route.IncludeUpstreamProviderID
	}
	if optionBool(request.Route.ProviderOptions, "disable_market_provider_selection") {
		return nil
	}
	threshold := optionInt(request.Route.ProviderOptions, "min_stock_threshold", defaultMinimumStockForLowestPrice)
	if threshold <= 0 {
		return nil
	}
	offers, err := c.ListPriceOffers(ctx, request.Route.UpstreamServiceKey, request.Route.ProviderCountryID)
	if err != nil {
		return nil
	}
	return selectProviderIDsByStock(offers, threshold, request.Target.MinPrice, request.Target.MaxPrice, request.Route.MinPrice, request.Route.MaxPrice)
}

func selectProviderIDsByStock(offers []PriceOffer, threshold int, bounds ...core.Money) []string {
	minPrice, hasMinPrice := firstParsedMoney(moneyAt(bounds, 0), moneyAt(bounds, 2))
	maxPrice, hasMaxPrice := firstParsedMoney(moneyAt(bounds, 1), moneyAt(bounds, 3))
	eligible := make([]PriceOffer, 0, len(offers))
	for _, offer := range offers {
		if !isAllowedQuality(offer.Quality) {
			continue
		}
		price, ok := parseOfferPrice(offer)
		if !ok {
			continue
		}
		if hasMinPrice && price < minPrice {
			continue
		}
		if hasMaxPrice && price > maxPrice {
			continue
		}
		eligible = append(eligible, offer)
	}
	offers = eligible
	groups := map[string]*priceGroup{}
	for _, offer := range offers {
		providerID := strings.TrimSpace(offer.ProviderID)
		if providerID == "" || offer.AvailableCount <= 0 {
			continue
		}
		priceText := strings.TrimSpace(offer.Price.AmountDecimal)
		price, ok := parseOfferPrice(offer)
		if !ok {
			continue
		}
		group := groups[priceText]
		if group == nil {
			group = &priceGroup{price: price, priceText: priceText}
			groups[priceText] = group
		}
		group.count += offer.AvailableCount
		if isGoldQuality(offer.Quality) {
			group.goldCount += offer.AvailableCount
			group.goldProviderIDs = append(group.goldProviderIDs, providerID)
		} else {
			group.silverCount += offer.AvailableCount
			group.silverProviderIDs = append(group.silverProviderIDs, providerID)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	ordered := make([]*priceGroup, 0, len(groups))
	for _, group := range groups {
		ordered = append(ordered, group)
	}
	sortPriceGroups(ordered)
	selected := ordered[0]
	for _, group := range ordered {
		if group.count >= threshold {
			selected = group
			break
		}
	}
	return selected.providerIDs()
}

func parseOfferPrice(offer PriceOffer) (float64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(offer.Price.AmountDecimal), 64)
	if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
		return 0, false
	}
	return parsed, true
}

func isAllowedQuality(value string) bool {
	return isGoldQuality(value) || isSilverQuality(value)
}

func isGoldQuality(value string) bool {
	return normalizeQuality(value) == "gold"
}

func isSilverQuality(value string) bool {
	return normalizeQuality(value) == "silver"
}

func normalizeQuality(value string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if r >= 'a' && r <= 'z' {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func moneyAt(values []core.Money, index int) core.Money {
	if index < 0 || index >= len(values) {
		return core.Money{}
	}
	return values[index]
}

func firstParsedMoney(values ...core.Money) (float64, bool) {
	for _, value := range values {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value.AmountDecimal), 64)
		if err == nil && !math.IsInf(parsed, 0) && !math.IsNaN(parsed) {
			return parsed, true
		}
	}
	return 0, false
}

func sortPriceGroups(groups []*priceGroup) {
	for i := 1; i < len(groups); i++ {
		current := groups[i]
		j := i - 1
		for j >= 0 && priceGroupLess(current, groups[j]) {
			groups[j+1] = groups[j]
			j--
		}
		groups[j+1] = current
	}
}

type priceGroup struct {
	price             float64
	priceText         string
	count             int
	goldCount         int
	silverCount       int
	goldProviderIDs   []string
	silverProviderIDs []string
}

func (g *priceGroup) providerIDs() []string {
	if g == nil {
		return nil
	}
	if len(g.goldProviderIDs) > 0 {
		return g.goldProviderIDs
	}
	return g.silverProviderIDs
}

func priceGroupLess(a, b *priceGroup) bool {
	if a.price != b.price {
		return a.price < b.price
	}
	if (a.goldCount > 0) != (b.goldCount > 0) {
		return a.goldCount > 0
	}
	if a.count != b.count {
		return a.count > b.count
	}
	return a.priceText < b.priceText
}

func (c *Client) GetStatus(ctx context.Context, upstreamActivationID string) (core.ProviderCodeResult, error) {
	params := url.Values{}
	params.Set("id", upstreamActivationID)
	result, err := c.api.Do(ctx, "getStatus", params)
	if err != nil {
		return core.ProviderCodeResult{}, err
	}
	return parseStatus(result)
}

func (c *Client) SetStatus(ctx context.Context, upstreamActivationID string, action core.ProviderAction) error {
	status, expected, err := statusForAction(action)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Set("id", upstreamActivationID)
	params.Set("status", status)
	result, err := c.api.Do(ctx, "setStatus", params)
	if err != nil {
		return err
	}
	if result != expected {
		return handlerapi.MapTextError(result)
	}
	return nil
}

func (c *Client) GetBalance(ctx context.Context) (core.Money, error) {
	result, err := c.api.Do(ctx, "getBalance", nil)
	if err != nil {
		return core.Money{}, err
	}
	const prefix = "ACCESS_BALANCE:"
	if !strings.HasPrefix(result, prefix) {
		return core.Money{}, handlerapi.MapTextError(result)
	}
	return core.Money{AmountDecimal: strings.TrimPrefix(result, prefix)}, nil
}

func (c *Client) serviceForRoute(ctx context.Context, request core.ProviderAcquireRequest) (string, error) {
	explicit := strings.TrimSpace(request.Route.UpstreamServiceKey)
	target := strings.TrimSpace(request.Target.ApplicationKey)
	if explicit != "" && explicit != target {
		return explicit, nil
	}
	candidate := firstNonEmpty(explicit, target)
	if candidate == "" {
		return "", core.NewError(core.CodeValidationFailed, "smsbower service is required", false)
	}
	applications, err := c.ListApplications(ctx)
	if err != nil {
		return "", err
	}
	if service := matchService(candidate, applications); service != "" {
		return service, nil
	}
	return "", core.NewError(core.CodeRouteNotFound, "smsbower service not found for sms target", false)
}

func matchService(candidate string, applications []ApplicationOffer) string {
	normalized := normalizeApplicationAlias(candidate)
	for _, app := range applications {
		if normalizeApplicationAlias(app.UpstreamServiceKey) == normalized || normalizeApplicationAlias(app.ApplicationKey) == normalized {
			return app.UpstreamServiceKey
		}
	}
	for _, app := range applications {
		display := normalizeApplicationAlias(app.DisplayName)
		if display != "" && (display == normalized || strings.Contains(display, normalized)) {
			return app.UpstreamServiceKey
		}
	}
	return ""
}

func normalizeApplicationAlias(value string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func (c *Client) countryForRoute(ctx context.Context, request core.ProviderAcquireRequest) (string, error) {
	if country := strings.TrimSpace(request.Route.ProviderCountryID); country != "" {
		return country, nil
	}
	aliases := countryAliases(request.Target)
	if len(aliases) == 0 {
		return "", core.NewError(core.CodeValidationFailed, "smsbower provider country id is required", false)
	}
	countries, err := c.ListCountries(ctx)
	if err != nil {
		return "", err
	}
	for _, country := range countries {
		if matchesCountryAlias(country.Name, aliases) {
			return country.CountryID, nil
		}
	}
	return "", core.NewError(core.CodeRouteNotFound, "smsbower country not found for sms target", false)
}

func countryAliases(target core.Target) map[string]bool {
	aliases := map[string]bool{}
	switch strings.ToUpper(strings.TrimSpace(target.CountryISO2)) {
	case "ID":
		aliases["indonesia"] = true
	}
	switch strings.TrimPrefix(strings.TrimSpace(target.CountryCallingCode), "+") {
	case "62":
		aliases["indonesia"] = true
	}
	if len(aliases) == 0 {
		return nil
	}
	return aliases
}

func normalizeCountryName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func matchesCountryAlias(name string, aliases map[string]bool) bool {
	normalized := normalizeCountryName(name)
	if aliases[normalized] {
		return true
	}
	for alias := range aliases {
		if strings.Contains(normalized, alias) {
			return true
		}
	}
	return false
}

func (c *Client) acquireParams(request core.ProviderAcquireRequest, v2 bool) url.Values {
	params := url.Values{}
	params.Set("service", request.Route.UpstreamServiceKey)
	params.Set("country", request.Route.ProviderCountryID)
	setMoney(params, "maxPrice", firstMoney(request.Target.MaxPrice, request.Route.MaxPrice))
	setMoney(params, "minPrice", firstMoney(request.Target.MinPrice, request.Route.MinPrice))
	if len(request.Route.IncludeUpstreamProviderID) > 0 {
		params.Set("providerIds", strings.Join(request.Route.IncludeUpstreamProviderID, ","))
	}
	if len(request.Route.ExcludeUpstreamProviderID) > 0 {
		params.Set("exceptProviderIds", strings.Join(request.Route.ExcludeUpstreamProviderID, ","))
	}
	if !v2 && len(request.Route.ExcludedPhonePrefixes) > 0 {
		params.Set("phoneException", strings.Join(request.Route.ExcludedPhonePrefixes, ","))
	}
	ref := firstNonEmpty(request.Route.ProviderOptions["ref"], c.ref)
	if !v2 && ref != "" {
		params.Set("ref", ref)
	}
	userID := firstNonEmpty(request.Route.ProviderOptions["userID"], request.Route.ProviderOptions["user_id"], c.userID)
	if userID != "" {
		params.Set("userID", userID)
	}
	return params
}

func (c *Client) requiresGetNumber(request core.ProviderAcquireRequest) bool {
	if len(request.Route.ExcludedPhonePrefixes) > 0 {
		return true
	}
	return firstNonEmpty(request.Route.ProviderOptions["ref"], c.ref) != ""
}

func parseAccessNumber(result string, request core.ProviderAcquireRequest) (core.ProviderActivation, error) {
	parts := strings.SplitN(result, ":", 3)
	if len(parts) != 3 || parts[0] != "ACCESS_NUMBER" {
		return core.ProviderActivation{}, core.NewError(core.CodeUpstreamRejected, "bad getNumber text response", false)
	}
	e164, national := phone.Normalize(parts[2], request.Target.CountryISO2, request.Target.CountryCallingCode)
	return core.ProviderActivation{
		UpstreamActivationID: parts[1],
		PhoneNumber:          core.PhoneNumber{E164: e164, NationalNumber: national, CountryISO2: request.Target.CountryISO2, CountryCallingCode: request.Target.CountryCallingCode},
		AcquiredAt:           time.Now().UTC(),
	}, nil
}

func (c *Client) parseGetNumberV2(result string, request core.ProviderAcquireRequest) (core.ProviderActivation, error) {
	var payload struct {
		ActivationID       int64   `json:"activationId"`
		PhoneNumber        string  `json:"phoneNumber"`
		ActivationCost     string  `json:"activationCost"`
		CountryCode        string  `json:"countryCode"`
		CanGetAnotherSMS   string  `json:"canGetAnotherSms"`
		ActivationTime     string  `json:"activationTime"`
		ActivationOperator *string `json:"activationOperator"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return core.ProviderActivation{}, core.NewError(core.CodeUpstreamRejected, "bad getNumberV2 json response", false)
	}
	if payload.ActivationID <= 0 {
		return core.ProviderActivation{}, core.NewError(core.CodeUpstreamRejected, "missing activationId in getNumberV2 response", false)
	}
	activationID := strconv.FormatInt(payload.ActivationID, 10)
	e164, national := phone.Normalize(payload.PhoneNumber, request.Target.CountryISO2, request.Target.CountryCallingCode)
	return core.ProviderActivation{
		UpstreamActivationID:     activationID,
		UpstreamOperator:         stringOrEmpty(payload.ActivationOperator),
		PhoneNumber:              core.PhoneNumber{E164: e164, NationalNumber: national, CountryISO2: request.Target.CountryISO2, CountryCallingCode: request.Target.CountryCallingCode},
		Price:                    core.Money{AmountDecimal: strings.TrimSpace(payload.ActivationCost)},
		AcquiredAt:               parseActivationTimeText(payload.ActivationTime),
		CanRequestAdditionalCode: payload.CanGetAnotherSMS == "1",
	}, nil
}

func parseStatus(result string) (core.ProviderCodeResult, error) {
	switch {
	case strings.HasPrefix(result, "STATUS_OK:"):
		return core.ProviderCodeResult{
			Status:     core.StatusCodeReceived,
			Code:       strings.Trim(strings.TrimSpace(strings.TrimPrefix(result, "STATUS_OK:")), "'\""),
			ReceivedAt: time.Now().UTC(),
		}, nil
	case result == "STATUS_WAIT_CODE":
		return core.ProviderCodeResult{Status: core.StatusPendingCode}, nil
	case strings.HasPrefix(result, "STATUS_WAIT_RETRY:"):
		return core.ProviderCodeResult{
			Status: core.StatusAdditionalCodeRequested,
			Code:   strings.TrimSpace(strings.TrimPrefix(result, "STATUS_WAIT_RETRY:")),
		}, nil
	case result == "STATUS_CANCEL":
		return core.ProviderCodeResult{Status: core.StatusCanceled}, nil
	default:
		return core.ProviderCodeResult{}, handlerapi.MapTextError(result)
	}
}

func statusForAction(action core.ProviderAction) (status string, expected string, err error) {
	switch action {
	case core.ActionMarkMessageSent:
		return "1", "ACCESS_READY", nil
	case core.ActionRequestAdditional:
		return "3", "ACCESS_RETRY_GET", nil
	case core.ActionCompleteActivation:
		return "6", "ACCESS_ACTIVATION", nil
	case core.ActionCancelActivation:
		return "8", "ACCESS_CANCEL", nil
	default:
		return "", "", core.NewError(core.CodeUnsupportedOperation, "unsupported smsbower status action", false)
	}
}

func isProviderTextError(result string) bool {
	return !strings.HasPrefix(strings.TrimSpace(result), "{")
}

func setMoney(params url.Values, key string, money core.Money) {
	if money.AmountDecimal != "" {
		params.Set(key, money.AmountDecimal)
	}
}

func firstMoney(values ...core.Money) core.Money {
	for _, value := range values {
		if value.AmountDecimal != "" {
			return value
		}
	}
	return core.Money{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func optionInt(options map[string]string, key string, fallback int) int {
	if options == nil {
		return fallback
	}
	value := strings.TrimSpace(options[key])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func optionBool(options map[string]string, key string) bool {
	if options == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(options[key])) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func rawJSONScalar(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	var floatValue float64
	if err := json.Unmarshal(raw, &floatValue); err == nil {
		return strconv.FormatFloat(floatValue, 'f', -1, 64)
	}
	return strings.Trim(string(raw), "\"")
}

func parseActivationTime(raw json.RawMessage) time.Time {
	return parseActivationTimeText(rawJSONScalar(raw))
}

func parseActivationTimeText(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().UTC()
	}
	if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
		if unix > 1_000_000_000_000 {
			return time.UnixMilli(unix).UTC()
		}
		return time.Unix(unix, 0).UTC()
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func decodeJSONObject(result string, out any) error {
	if err := json.Unmarshal([]byte(result), out); err != nil {
		return core.NewError(core.CodeUpstreamRejected, fmt.Sprintf("bad json response: %v", err), false)
	}
	return nil
}
