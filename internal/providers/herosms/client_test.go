package herosms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
)

func TestAcquireNumberParsesAccessNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("action"); got != "getNumber" {
			t.Fatalf("action = %q, want getNumber", got)
		}
		if got := r.URL.Query().Get("service"); got != "go" {
			t.Fatalf("service = %q, want go", got)
		}
		_, _ = w.Write([]byte("ACCESS_NUMBER:123:628123456789"))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, APIKey: "test-token"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := client.AcquireNumber(context.Background(), core.ProviderAcquireRequest{
		Route:  core.Route{UpstreamServiceKey: "go", ProviderCountryID: "6"},
		Target: core.Target{ApplicationKey: "gojek", CountryISO2: "ID", CountryCallingCode: "62"},
	})
	if err != nil {
		t.Fatalf("AcquireNumber() error = %v", err)
	}
	if resp.UpstreamActivationID != "123" {
		t.Fatalf("activation id = %q", resp.UpstreamActivationID)
	}
	if resp.PhoneNumber.E164 != "+628123456789" || resp.PhoneNumber.NationalNumber != "8123456789" {
		t.Fatalf("phone = %#v", resp.PhoneNumber)
	}
}

func TestSetStatusCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("status"); got != "8" {
			t.Fatalf("status = %q, want 8", got)
		}
		_, _ = w.Write([]byte("ACCESS_CANCEL"))
	}))
	defer server.Close()

	client, err := New(Config{Endpoint: server.URL, APIKey: "test-token"}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := client.SetStatus(context.Background(), "123", core.ActionCancelActivation); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
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
