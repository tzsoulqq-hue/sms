package fivesim

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
)

func TestAcquireNumberBuysActivationWithBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/user/buy/activation/england/vodafone/facebook" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		assertHeader(t, r.Header.Get("Authorization"), "Bearer test-token")
		query := r.URL.Query()
		assertQuery(t, query.Get("maxPrice"), "21")
		assertQuery(t, query.Get("reuse"), "1")
		assertQuery(t, query.Get("voice"), "")
		assertQuery(t, query.Get("ref"), "route-ref")
		_, _ = w.Write([]byte(`{
			"id": 11631253,
			"created_at": "2018-10-13T08:13:38.809469028Z",
			"phone": "+447350690992",
			"operator": "vodafone",
			"product": "facebook",
			"price": 21,
			"status": "PENDING",
			"expires": "2018-10-13T08:28:38.809469028Z",
			"sms": [],
			"forwarding": false,
			"forwarding_number": "",
			"country": "england"
		}`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, Token: "test-token", DefaultOperator: "any", CurrencyCode: "RUB"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	activation, err := client.AcquireNumber(context.Background(), core.ProviderAcquireRequest{
		Route: core.Route{
			ProviderCountryID:         "england",
			UpstreamServiceKey:        "facebook",
			IncludeUpstreamProviderID: []string{"vodafone"},
			ProviderOptions:           map[string]string{"reuse": "true", "ref": "route-ref"},
		},
		Target: core.Target{
			ApplicationKey:     "facebook",
			CountryISO2:        "GB",
			CountryCallingCode: "44",
			MaxPrice:           core.Money{AmountDecimal: "21"},
		},
	})
	if err != nil {
		t.Fatalf("AcquireNumber() error = %v", err)
	}
	if activation.UpstreamActivationID != "11631253" || activation.UpstreamOperator != "vodafone" {
		t.Fatalf("activation = %#v", activation)
	}
	if activation.PhoneNumber.E164 != "+447350690992" || activation.PhoneNumber.NationalNumber != "7350690992" {
		t.Fatalf("phone = %#v", activation.PhoneNumber)
	}
	if activation.Price.CurrencyCode != "RUB" || activation.Price.AmountDecimal != "21" {
		t.Fatalf("price = %#v", activation.Price)
	}
	if activation.ExpiresAt.IsZero() {
		t.Fatal("ExpiresAt is zero")
	}
}

func TestGetStatusReturnsLatestSMSCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/user/check/11631253" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id": 11631253,
			"status": "RECEIVED",
			"sms": [
				{"date": "2018-10-13T08:19:38Z", "text": "old", "code": "111"},
				{"date": "2018-10-13T08:20:38Z", "text": "Facebook: 09363", "code": "09363"}
			]
		}`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, Token: "test-token"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := client.GetStatus(context.Background(), "11631253")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if result.Status != core.StatusCodeReceived || result.Code != "09363" || result.MessageText != "Facebook: 09363" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSetStatusUsesFinishAndCancelEndpoints(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte(`{"id": 1, "status": "FINISHED"}`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, Token: "test-token"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := client.SetStatus(context.Background(), "1", core.ActionMarkMessageSent); err != nil {
		t.Fatalf("SetStatus(mark) error = %v", err)
	}
	if err := client.SetStatus(context.Background(), "1", core.ActionRequestAdditional); err != nil {
		t.Fatalf("SetStatus(additional) error = %v", err)
	}
	if err := client.SetStatus(context.Background(), "1", core.ActionCompleteActivation); err != nil {
		t.Fatalf("SetStatus(complete) error = %v", err)
	}
	if err := client.SetStatus(context.Background(), "1", core.ActionCancelActivation); err != nil {
		t.Fatalf("SetStatus(cancel) error = %v", err)
	}
	if len(paths) != 2 || paths[0] != "/v1/user/finish/1" || paths[1] != "/v1/user/cancel/1" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestGetBalanceParsesProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/user/profile" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"balance": 100.5}`))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, Token: "test-token", CurrencyCode: "RUB"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	balance, err := client.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBalance() error = %v", err)
	}
	if balance.CurrencyCode != "RUB" || balance.AmountDecimal != "100.5" {
		t.Fatalf("balance = %#v", balance)
	}
}

func TestMap5SimErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		code core.ErrorCode
	}{
		{name: "no phones", body: "no free phones", code: core.CodeNoNumberAvailable},
		{name: "balance", body: "not enough user balance", code: core.CodeInsufficientBalance},
		{name: "has sms", body: "order has sms", code: core.CodeCancelNotAllowed},
		{name: "not found", body: "order not found", code: core.CodeActivationNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapError(http.StatusBadRequest, tt.body)
			var smsErr *core.Error
			if !errors.As(err, &smsErr) {
				t.Fatalf("error = %T, want *core.Error", err)
			}
			if smsErr.Code != tt.code {
				t.Fatalf("code = %s, want %s", smsErr.Code, tt.code)
			}
		})
	}
}

func TestParseTimeSupports5SimNanoTimestamps(t *testing.T) {
	got := parseTime("2018-10-13T08:13:38.809469028Z")
	want := time.Date(2018, 10, 13, 8, 13, 38, 809469028, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("time = %s, want %s", got, want)
	}
}

func assertQuery(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("query value = %q, want %q", got, want)
	}
}

func assertHeader(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("header = %q, want %q", got, want)
	}
}
