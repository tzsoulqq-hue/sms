package fivesim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListProductsParsesGuestProducts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/guest/products/england/any" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"facebook": {"Category": "activation", "Qty": 133, "Price": 21},
			"1day": {"Category": "hosting", "Qty": 14, "Price": 80}
		}`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, Token: "test-token", CurrencyCode: "RUB"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	offers, err := client.ListProducts(context.Background(), "england", "any")
	if err != nil {
		t.Fatalf("ListProducts() error = %v", err)
	}
	if len(offers) != 2 {
		t.Fatalf("offers = %#v", offers)
	}
	var facebook ProductOffer
	for _, offer := range offers {
		if offer.UpstreamServiceKey == "facebook" {
			facebook = offer
		}
	}
	if facebook.AvailableCount != 133 || facebook.Price.AmountDecimal != "21" || facebook.Category != "activation" {
		t.Fatalf("facebook offer = %#v", facebook)
	}
}

func TestListCountriesParsesGuestCountries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/guest/countries" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"england": {
				"iso": {"gb": 1},
				"prefix": {"+44": 1},
				"text_en": "United Kingdom"
			}
		}`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, Token: "test-token"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	countries, err := client.ListCountries(context.Background())
	if err != nil {
		t.Fatalf("ListCountries() error = %v", err)
	}
	if len(countries) != 1 {
		t.Fatalf("countries = %#v", countries)
	}
	country := countries[0]
	if country.CountryID != "england" || country.CountryISO2 != "gb" || country.CountryCallingCode != "44" {
		t.Fatalf("country = %#v", country)
	}
}

func TestListPriceOffersParsesGuestPrices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/guest/prices" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("country") != "england" || r.URL.Query().Get("product") != "facebook" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{
			"england": {
				"facebook": {
					"vodafone": {"cost": 4, "count": 1260, "rate": 99.99}
				}
			}
		}`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, Token: "test-token", CurrencyCode: "RUB"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	offers, err := client.ListPriceOffers(context.Background(), "facebook", "england")
	if err != nil {
		t.Fatalf("ListPriceOffers() error = %v", err)
	}
	if len(offers) != 1 {
		t.Fatalf("offers = %#v", offers)
	}
	offer := offers[0]
	if offer.Operator != "vodafone" || offer.Price.AmountDecimal != "4" || offer.AvailableCount != 1260 || offer.SuccessRate != 99.99 {
		t.Fatalf("offer = %#v", offer)
	}
}
