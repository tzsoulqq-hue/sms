package fivesim

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/byte-v-forge/sms/internal/core"
)

type ProductOffer struct {
	ApplicationKey     string
	UpstreamServiceKey string
	Category           string
	CountryID          string
	Operator           string
	Price              core.Money
	AvailableCount     int
}

type Country struct {
	CountryID          string
	Name               string
	CountryISO2        string
	CountryCallingCode string
}

type PriceOffer struct {
	CountryID          string
	UpstreamServiceKey string
	Operator           string
	Price              core.Money
	AvailableCount     int
	SuccessRate        float64
}

func (c *Client) ListProducts(ctx context.Context, countryID, operator string) ([]ProductOffer, error) {
	if operator == "" {
		operator = c.defaultOperator
	}
	var raw map[string]struct {
		Category string          `json:"Category"`
		Qty      int             `json:"Qty"`
		Price    json.RawMessage `json:"Price"`
	}
	path := "/v1/guest/products/" + url.PathEscape(countryID) + "/" + url.PathEscape(operator)
	if err := c.getJSON(ctx, path, nil, false, &raw); err != nil {
		return nil, err
	}
	offers := make([]ProductOffer, 0, len(raw))
	for product, offer := range raw {
		offers = append(offers, ProductOffer{
			ApplicationKey:     product,
			UpstreamServiceKey: product,
			Category:           offer.Category,
			CountryID:          countryID,
			Operator:           operator,
			Price:              core.Money{CurrencyCode: c.currencyCode, AmountDecimal: rawJSONScalar(offer.Price)},
			AvailableCount:     offer.Qty,
		})
	}
	return offers, nil
}

func (c *Client) ListCountries(ctx context.Context) ([]Country, error) {
	var raw map[string]struct {
		ISO    map[string]int `json:"iso"`
		Prefix map[string]int `json:"prefix"`
		Name   string         `json:"text_en"`
	}
	if err := c.getJSON(ctx, "/v1/guest/countries", nil, false, &raw); err != nil {
		return nil, err
	}
	countries := make([]Country, 0, len(raw))
	for countryID, item := range raw {
		countries = append(countries, Country{
			CountryID:          countryID,
			Name:               item.Name,
			CountryISO2:        firstMapKey(item.ISO),
			CountryCallingCode: trimPlus(firstMapKey(item.Prefix)),
		})
	}
	return countries, nil
}

func (c *Client) ListPriceOffers(ctx context.Context, product, countryID string) ([]PriceOffer, error) {
	params := url.Values{}
	if product != "" {
		params.Set("product", product)
	}
	if countryID != "" {
		params.Set("country", countryID)
	}
	var raw map[string]map[string]map[string]struct {
		Cost  json.RawMessage `json:"cost"`
		Count int             `json:"count"`
		Rate  float64         `json:"rate"`
	}
	if err := c.getJSON(ctx, "/v1/guest/prices", params, false, &raw); err != nil {
		return nil, err
	}
	var offers []PriceOffer
	for country, byProduct := range raw {
		for productKey, byOperator := range byProduct {
			for operator, offer := range byOperator {
				offers = append(offers, PriceOffer{
					CountryID:          country,
					UpstreamServiceKey: productKey,
					Operator:           operator,
					Price:              core.Money{CurrencyCode: c.currencyCode, AmountDecimal: rawJSONScalar(offer.Cost)},
					AvailableCount:     offer.Count,
					SuccessRate:        offer.Rate,
				})
			}
		}
	}
	return offers, nil
}

func firstMapKey(values map[string]int) string {
	for key := range values {
		return key
	}
	return ""
}

func trimPlus(value string) string {
	if len(value) > 0 && value[0] == '+' {
		return value[1:]
	}
	return value
}
