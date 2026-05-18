package app

import (
	"context"
	"testing"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

type sequenceID struct {
	values []string
}

func (s *sequenceID) NewID(_ string) string {
	value := s.values[0]
	s.values = s.values[1:]
	return value
}

type fakeProvider struct {
	policy             core.ProviderPolicy
	providerActivation core.ProviderActivation
	setStatuses        []core.ProviderAction
	status             core.ProviderCodeResult
}

func (p *fakeProvider) Key() string {
	return "fake"
}

func (p *fakeProvider) Policy() core.ProviderPolicy {
	return p.policy
}

func (p *fakeProvider) AcquireNumber(context.Context, core.ProviderAcquireRequest) (core.ProviderActivation, error) {
	if p.providerActivation.UpstreamActivationID != "" {
		return p.providerActivation, nil
	}
	return core.ProviderActivation{
		UpstreamActivationID: "upstream-1",
		PhoneNumber: core.PhoneNumber{
			E164:               "+628123456789",
			NationalNumber:     "8123456789",
			CountryISO2:        "ID",
			CountryCallingCode: "62",
		},
		CanRequestAdditionalCode: true,
	}, nil
}

func (p *fakeProvider) GetStatus(context.Context, string) (core.ProviderCodeResult, error) {
	return p.status, nil
}

func (p *fakeProvider) SetStatus(_ context.Context, _ string, action core.ProviderAction) error {
	p.setStatuses = append(p.setStatuses, action)
	return nil
}

func (p *fakeProvider) GetBalance(context.Context) (core.Money, error) {
	return core.Money{}, nil
}

func TestCancelHonorsProviderMinAge(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)}
	provider := &fakeProvider{policy: core.ProviderPolicy{
		ActivationTTL:      20 * time.Minute,
		CancelAllowedAfter: 2 * time.Minute,
	}}
	service := NewActivationService(
		NewMemoryActivationStore(),
		NewStaticRouteResolver([]core.Route{{
			ProviderKey:        "fake",
			ApplicationKey:     "gojek",
			UpstreamServiceKey: "go",
			CountryISO2:        "ID",
		}}),
		[]core.Provider{provider},
		clock,
		&sequenceID{values: []string{"req-1", "act-1"}},
	)

	activation, err := service.AcquireNumber(ctx, core.AcquireNumberCommand{
		Target: core.Target{ApplicationKey: "gojek", CountryISO2: "ID", CountryCallingCode: "62"},
	})
	if err != nil {
		t.Fatalf("AcquireNumber() error = %v", err)
	}
	wantCancelAllowedAt := activation.AcquiredAt.Add(2 * time.Minute)
	if !activation.CancelAllowedAt.Equal(wantCancelAllowedAt) {
		t.Fatalf("cancel_allowed_at = %s, want %s", activation.CancelAllowedAt, wantCancelAllowedAt)
	}

	_, err = service.CancelActivation(ctx, activation.ID, "cancel-1")
	if err == nil {
		t.Fatal("CancelActivation() expected early cancel error")
	}
	smsErr, ok := err.(*core.Error)
	if !ok || smsErr.Code != core.CodeCancelNotAllowed || !smsErr.Retryable {
		t.Fatalf("CancelActivation() error = %#v", err)
	}
	if got := len(provider.setStatuses); got != 0 {
		t.Fatalf("upstream SetStatus calls = %d, want 0", got)
	}

	clock.Advance(2 * time.Minute)
	canceled, err := service.CancelActivation(ctx, activation.ID, "cancel-2")
	if err != nil {
		t.Fatalf("CancelActivation() after min age error = %v", err)
	}
	if canceled.Status != core.StatusCanceled {
		t.Fatalf("status = %s, want %s", canceled.Status, core.StatusCanceled)
	}
	if got := provider.setStatuses; len(got) != 1 || got[0] != core.ActionCancelActivation {
		t.Fatalf("upstream SetStatus calls = %#v", got)
	}
}

func TestAcquireHonorsProviderExpiration(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)}
	providerExpiresAt := clock.now.Add(15 * time.Minute)
	provider := &fakeProvider{
		policy: core.ProviderPolicy{ActivationTTL: 20 * time.Minute},
		providerActivation: core.ProviderActivation{
			UpstreamActivationID: "upstream-1",
			PhoneNumber:          core.PhoneNumber{E164: "+447350690992", NationalNumber: "7350690992", CountryISO2: "GB", CountryCallingCode: "44"},
			AcquiredAt:           clock.now,
			ExpiresAt:            providerExpiresAt,
		},
	}
	service := NewActivationService(
		NewMemoryActivationStore(),
		NewStaticRouteResolver([]core.Route{{ProviderKey: "fake", ApplicationKey: "facebook", CountryISO2: "GB"}}),
		[]core.Provider{provider},
		clock,
		&sequenceID{values: []string{"req-1", "act-1"}},
	)
	activation, err := service.AcquireNumber(ctx, core.AcquireNumberCommand{
		Target: core.Target{ApplicationKey: "facebook", CountryISO2: "GB", CountryCallingCode: "44"},
	})
	if err != nil {
		t.Fatalf("AcquireNumber() error = %v", err)
	}
	if !activation.ExpiresAt.Equal(providerExpiresAt) {
		t.Fatalf("expires_at = %s, want %s", activation.ExpiresAt, providerExpiresAt)
	}
}

func TestExpiredActivationCannotCancel(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)}
	provider := &fakeProvider{policy: core.ProviderPolicy{ActivationTTL: time.Minute}}
	service := NewActivationService(
		NewMemoryActivationStore(),
		NewStaticRouteResolver([]core.Route{{ProviderKey: "fake", ApplicationKey: "telegram", CountryISO2: "US"}}),
		[]core.Provider{provider},
		clock,
		&sequenceID{values: []string{"req-1", "act-1"}},
	)
	activation, err := service.AcquireNumber(ctx, core.AcquireNumberCommand{
		Target: core.Target{ApplicationKey: "telegram", CountryISO2: "US", CountryCallingCode: "1"},
	})
	if err != nil {
		t.Fatalf("AcquireNumber() error = %v", err)
	}
	clock.Advance(time.Minute)

	expired, err := service.CancelActivation(ctx, activation.ID, "cancel")
	if err == nil {
		t.Fatal("CancelActivation() expected expiration error")
	}
	smsErr, ok := err.(*core.Error)
	if !ok || smsErr.Code != core.CodeActivationExpired {
		t.Fatalf("CancelActivation() error = %#v", err)
	}
	if expired.Status != core.StatusExpired {
		t.Fatalf("status = %s, want %s", expired.Status, core.StatusExpired)
	}
	if got := len(provider.setStatuses); got != 0 {
		t.Fatalf("upstream SetStatus calls = %d, want 0", got)
	}
}
