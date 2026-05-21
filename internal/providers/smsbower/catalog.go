package smsbower

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/byte-v-forge/sms/internal/core"
	"github.com/byte-v-forge/sms/internal/providers/handlerapi"
)

type ApplicationOffer struct {
	ApplicationKey     string
	UpstreamServiceKey string
	DisplayName        string
}

type Country struct {
	CountryID string
	Name      string
}

type PriceOffer struct {
	CountryID          string
	UpstreamServiceKey string
	ProviderID         string
	Price              core.Money
	AvailableCount     int
}

func (c *Client) ListApplications(ctx context.Context) ([]ApplicationOffer, error) {
	result, err := c.api.Do(ctx, "getServicesList", nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Status   string `json:"status"`
		Services []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"services"`
	}
	if err := decodeJSONObject(result, &payload); err != nil {
		return nil, err
	}
	if payload.Status != "" && payload.Status != "success" {
		return nil, handlerapi.MapTextError(payload.Status)
	}
	offers := make([]ApplicationOffer, 0, len(payload.Services))
	for _, service := range payload.Services {
		offers = append(offers, ApplicationOffer{
			ApplicationKey:     service.Code,
			UpstreamServiceKey: service.Code,
			DisplayName:        service.Name,
		})
	}
	return offers, nil
}

func (c *Client) ListCountries(ctx context.Context) ([]Country, error) {
	result, err := c.api.Do(ctx, "getCountries", nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]struct {
		ID  json.RawMessage `json:"id"`
		Rus string          `json:"rus"`
		Eng string          `json:"eng"`
		Chn string          `json:"chn"`
	}
	if err := decodeJSONObject(result, &raw); err != nil {
		return nil, err
	}
	countries := make([]Country, 0, len(raw))
	for key, item := range raw {
		id := rawJSONScalar(item.ID)
		if id == "" || id == "0" {
			id = key
		}
		name := firstNonEmpty(item.Eng, item.Chn, item.Rus, key)
		countries = append(countries, Country{CountryID: id, Name: name})
	}
	return countries, nil
}

func (c *Client) ListPriceOffers(ctx context.Context, serviceKey, countryID string) ([]PriceOffer, error) {
	params := url.Values{}
	params.Set("service", serviceKey)
	params.Set("country", countryID)
	result, err := c.api.Do(ctx, "getPricesV3", params)
	if err != nil {
		return nil, err
	}
	var raw map[string]map[string]map[string]struct {
		Count      int             `json:"count"`
		Price      json.RawMessage `json:"price"`
		ProviderID json.RawMessage `json:"provider_id"`
	}
	if err := decodeJSONObject(result, &raw); err != nil {
		return nil, err
	}
	var offers []PriceOffer
	for cID, byService := range raw {
		for svc, byProvider := range byService {
			for providerID, offer := range byProvider {
				offers = append(offers, PriceOffer{
					CountryID:          cID,
					UpstreamServiceKey: svc,
					ProviderID:         firstNonEmpty(rawJSONScalar(offer.ProviderID), providerID),
					Price:              core.Money{AmountDecimal: rawJSONScalar(offer.Price)},
					AvailableCount:     offer.Count,
				})
			}
		}
	}
	return offers, nil
}
