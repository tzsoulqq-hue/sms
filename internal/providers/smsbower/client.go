package smsbower

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
	"github.com/byte-v-forge/sms/internal/providers/handlerapi"
	"github.com/byte-v-forge/sms/internal/providers/phone"
)

const (
	DefaultEndpoint = "https://smsbower.page/stubs/handler_api.php"
	ProviderKey     = "smsbower"
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
			ActivationTTL:      20 * time.Minute,
			PollInterval:       5 * time.Second,
			CancelAllowedAfter: 2 * time.Minute,
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
	service := firstNonEmpty(request.Route.UpstreamServiceKey, request.Target.ApplicationKey)
	if service == "" {
		return core.ProviderActivation{}, core.NewError(core.CodeValidationFailed, "smsbower service is required", false)
	}
	country, err := c.countryForRoute(ctx, request)
	if err != nil {
		return core.ProviderActivation{}, err
	}
	request.Route.UpstreamServiceKey = service
	request.Route.ProviderCountryID = country
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
	setMoney(params, "minPrice", request.Route.MinPrice)
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
		ActivationID       string          `json:"activationId"`
		PhoneNumber        json.RawMessage `json:"phoneNumber"`
		ActivationCost     json.RawMessage `json:"activationCost"`
		CountryCode        string          `json:"countryCode"`
		CanGetAnotherSMS   bool            `json:"canGetAnotherSms"`
		ActivationTime     json.RawMessage `json:"activationTime"`
		ActivationOperator string          `json:"activationOperator"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return core.ProviderActivation{}, core.NewError(core.CodeUpstreamRejected, "bad getNumberV2 json response", false)
	}
	if payload.ActivationID == "" {
		return core.ProviderActivation{}, core.NewError(core.CodeUpstreamRejected, "missing activationId in getNumberV2 response", false)
	}
	rawPhone := rawJSONScalar(payload.PhoneNumber)
	e164, national := phone.Normalize(rawPhone, request.Target.CountryISO2, request.Target.CountryCallingCode)
	return core.ProviderActivation{
		UpstreamActivationID:     payload.ActivationID,
		UpstreamOperator:         payload.ActivationOperator,
		PhoneNumber:              core.PhoneNumber{E164: e164, NationalNumber: national, CountryISO2: request.Target.CountryISO2, CountryCallingCode: request.Target.CountryCallingCode},
		Price:                    core.Money{AmountDecimal: rawJSONScalar(payload.ActivationCost)},
		AcquiredAt:               parseActivationTime(payload.ActivationTime),
		CanRequestAdditionalCode: payload.CanGetAnotherSMS,
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
	value := rawJSONScalar(raw)
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

func decodeJSONObject(result string, out any) error {
	if err := json.Unmarshal([]byte(result), out); err != nil {
		return core.NewError(core.CodeUpstreamRejected, fmt.Sprintf("bad json response: %v", err), false)
	}
	return nil
}
