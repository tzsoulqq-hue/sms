package smsbower

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
)

func TestAcquireNumberV2MapsProviderSpecificOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assertQuery(t, query.Get("action"), "getNumberV2")
		assertQuery(t, query.Get("service"), "go")
		assertQuery(t, query.Get("country"), "6")
		assertQuery(t, query.Get("providerIds"), "1,2")
		assertQuery(t, query.Get("exceptProviderIds"), "9")
		assertQuery(t, query.Get("phoneException"), "")
		assertQuery(t, query.Get("ref"), "")
		assertQuery(t, query.Get("userID"), "u-1")
		_, _ = w.Write([]byte(`{
			"activationId": "abc",
			"phoneNumber": 628123456789,
			"activationCost": 0.12,
			"countryCode": "ID",
			"canGetAnotherSms": true,
			"activationTime": "2026-05-18 12:00:00",
			"activationOperator": "telkomsel"
		}`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, APIKey: "test-token"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := client.AcquireNumber(context.Background(), core.ProviderAcquireRequest{
		Route: core.Route{
			UpstreamServiceKey:        "go",
			ProviderCountryID:         "6",
			IncludeUpstreamProviderID: []string{"1", "2"},
			ExcludeUpstreamProviderID: []string{"9"},
			ProviderOptions:           map[string]string{"userID": "u-1"},
		},
		Target: core.Target{ApplicationKey: "gojek", CountryISO2: "ID", CountryCallingCode: "62"},
	})
	if err != nil {
		t.Fatalf("AcquireNumber() error = %v", err)
	}
	if resp.UpstreamActivationID != "abc" || resp.UpstreamOperator != "telkomsel" {
		t.Fatalf("activation = %#v", resp)
	}
	if !resp.CanRequestAdditionalCode {
		t.Fatal("CanRequestAdditionalCode = false, want true")
	}
	if resp.Price.AmountDecimal != "0.12" {
		t.Fatalf("price = %#v", resp.Price)
	}
	wantTime := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	if !resp.AcquiredAt.Equal(wantTime) {
		t.Fatalf("acquired_at = %s, want %s", resp.AcquiredAt, wantTime)
	}
}

func TestAcquireNumberUsesGetNumberWhenV2UnsupportedOptionsArePresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assertQuery(t, query.Get("action"), "getNumber")
		assertQuery(t, query.Get("service"), "go")
		assertQuery(t, query.Get("country"), "6")
		assertQuery(t, query.Get("phoneException"), "7900111")
		assertQuery(t, query.Get("ref"), "route-ref")
		_, _ = w.Write([]byte("ACCESS_NUMBER:abc:628123456789"))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, APIKey: "test-token", Ref: "default-ref"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := client.AcquireNumber(context.Background(), core.ProviderAcquireRequest{
		Route: core.Route{
			UpstreamServiceKey:    "go",
			ProviderCountryID:     "6",
			ExcludedPhonePrefixes: []string{"7900111"},
			ProviderOptions:       map[string]string{"ref": "route-ref"},
		},
		Target: core.Target{ApplicationKey: "gojek", CountryISO2: "ID", CountryCallingCode: "62"},
	})
	if err != nil {
		t.Fatalf("AcquireNumber() error = %v", err)
	}
	if resp.UpstreamActivationID != "abc" {
		t.Fatalf("activation id = %q, want abc", resp.UpstreamActivationID)
	}
	if resp.PhoneNumber.E164 != "+628123456789" {
		t.Fatalf("phone = %#v", resp.PhoneNumber)
	}
}

func TestPolicyRequiresTwoMinutesBeforeCancel(t *testing.T) {
	client, err := New(Config{Endpoint: "https://example.test/api", APIKey: "test-token"}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := client.Policy().CancelAllowedAfter; got != 2*time.Minute {
		t.Fatalf("CancelAllowedAfter = %s, want 2m", got)
	}
}

func assertQuery(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("query value = %q, want %q", got, want)
	}
}
