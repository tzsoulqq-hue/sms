package app

import (
	"context"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
)

type ActivationService struct {
	store     core.ActivationStore
	routes    core.RouteResolver
	providers map[string]core.Provider
	clock     core.Clock
	ids       core.IDGenerator
}

func NewActivationService(
	store core.ActivationStore,
	routes core.RouteResolver,
	providers []core.Provider,
	clock core.Clock,
	ids core.IDGenerator,
) *ActivationService {
	index := make(map[string]core.Provider, len(providers))
	for _, provider := range providers {
		index[provider.Key()] = provider
	}
	if clock == nil {
		clock = SystemClock{}
	}
	if ids == nil {
		ids = RandomIDGenerator{}
	}
	return &ActivationService{
		store:     store,
		routes:    routes,
		providers: index,
		clock:     clock,
		ids:       ids,
	}
}

func (s *ActivationService) AcquireNumber(ctx context.Context, cmd core.AcquireNumberCommand) (core.Activation, error) {
	if cmd.Target.ApplicationKey == "" {
		return core.Activation{}, core.NewError(core.CodeValidationFailed, "application_key is required", false)
	}
	route, err := s.routes.Resolve(ctx, core.RouteRequest{
		Target:           cmd.Target,
		ProviderKey:      cmd.ProviderKey,
		ProviderConfigID: cmd.ProviderConfigID,
	})
	if err != nil {
		return core.Activation{}, err
	}
	cmd.Target = withRouteTargetDefaults(cmd.Target, route)
	provider, err := s.provider(route.ProviderKey)
	if err != nil {
		return core.Activation{}, err
	}
	if cmd.RequestID == "" {
		cmd.RequestID = s.ids.NewID("req_")
	}

	providerActivation, err := provider.AcquireNumber(ctx, core.ProviderAcquireRequest{
		RequestID:     cmd.RequestID,
		Route:         route,
		Target:        cmd.Target,
		LeaseDuration: cmd.LeaseDuration,
	})
	if err != nil {
		return core.Activation{}, err
	}

	now := s.clock.Now()
	acquiredAt := providerActivation.AcquiredAt
	if acquiredAt.IsZero() {
		acquiredAt = now
	}
	policy := provider.Policy().WithDefaults()
	ttl := policy.ActivationTTL
	if cmd.LeaseDuration > 0 && cmd.LeaseDuration < ttl {
		ttl = cmd.LeaseDuration
	}
	expiresAt := providerActivation.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = acquiredAt.Add(ttl)
	} else if cmd.LeaseDuration > 0 {
		requestExpiresAt := acquiredAt.Add(cmd.LeaseDuration)
		if requestExpiresAt.Before(expiresAt) {
			expiresAt = requestExpiresAt
		}
	}
	var cancelAllowedAt time.Time
	if policy.CancelAllowedAfter > 0 {
		cancelAllowedAt = acquiredAt.Add(policy.CancelAllowedAfter)
	}
	activation := core.Activation{
		ID:                       s.ids.NewID("act_"),
		RequestID:                cmd.RequestID,
		ProviderConfigID:         route.ProviderOptions["provider_config_id"],
		ProviderKey:              provider.Key(),
		UpstreamActivationID:     providerActivation.UpstreamActivationID,
		UpstreamOperator:         providerActivation.UpstreamOperator,
		Target:                   cmd.Target,
		PhoneNumber:              providerActivation.PhoneNumber,
		Status:                   core.StatusPendingCode,
		Price:                    providerActivation.Price,
		AcquiredAt:               acquiredAt,
		ExpiresAt:                expiresAt,
		UpdatedAt:                now,
		CancelAllowedAt:          cancelAllowedAt,
		CanRequestAdditionalCode: providerActivation.CanRequestAdditionalCode,
		Labels:                   cloneMap(cmd.Labels),
	}
	if err := s.store.Save(ctx, activation); err != nil {
		return core.Activation{}, err
	}
	return activation, nil
}

func (s *ActivationService) GetActivation(ctx context.Context, activationID string) (core.Activation, error) {
	activation, err := s.store.Get(ctx, activationID)
	if err != nil {
		return core.Activation{}, err
	}
	if err := s.expireIfNeeded(ctx, &activation); err != nil {
		return core.Activation{}, err
	}
	return activation, nil
}

func (s *ActivationService) CheckCode(ctx context.Context, activationID string) (core.Activation, *core.SMSCode, error) {
	activation, err := s.store.Get(ctx, activationID)
	if err != nil {
		return core.Activation{}, nil, err
	}
	if err := s.expireIfNeeded(ctx, &activation); err != nil {
		return core.Activation{}, nil, err
	}
	provider, err := s.provider(activation.ProviderKey)
	if err != nil {
		return core.Activation{}, nil, err
	}
	result, err := provider.GetStatus(ctx, activation.UpstreamActivationID)
	if err != nil {
		activation.LastError = asCoreError(err)
		activation.UpdatedAt = s.clock.Now()
		_ = s.store.Update(ctx, activation)
		return activation, nil, err
	}
	activation.UpdatedAt = s.clock.Now()
	switch result.Status {
	case core.StatusCodeReceived:
		receivedAt := result.ReceivedAt
		if receivedAt.IsZero() {
			receivedAt = activation.UpdatedAt
		}
		code := &core.SMSCode{Value: result.Code, MessageText: result.MessageText, ReceivedAt: receivedAt}
		activation.Code = code
		activation.Status = core.StatusCodeReceived
		if err := s.store.Update(ctx, activation); err != nil {
			return core.Activation{}, nil, err
		}
		return activation, code, nil
	case core.StatusCanceled, core.StatusFailed, core.StatusExpired:
		activation.Status = result.Status
	}
	if err := s.store.Update(ctx, activation); err != nil {
		return core.Activation{}, nil, err
	}
	return activation, activation.Code, nil
}

func (s *ActivationService) WaitForCode(ctx context.Context, activationID string, timeout, pollInterval time.Duration) (core.Activation, *core.SMSCode, error) {
	activation, err := s.store.Get(ctx, activationID)
	if err != nil {
		return core.Activation{}, nil, err
	}
	provider, err := s.provider(activation.ProviderKey)
	if err != nil {
		return core.Activation{}, nil, err
	}
	policy := provider.Policy().WithDefaults()
	if timeout <= 0 {
		timeout = time.Until(activation.ExpiresAt)
	}
	if pollInterval <= 0 {
		pollInterval = policy.PollInterval
	}
	deadline := s.clock.Now().Add(timeout)
	for {
		current, code, err := s.CheckCode(ctx, activationID)
		if err != nil {
			return current, nil, err
		}
		if code != nil && code.Value != "" {
			return current, code, nil
		}
		if !s.clock.Now().Before(deadline) {
			return current, nil, core.NewError(core.CodeTimeout, "sms code wait timed out", true)
		}
		select {
		case <-ctx.Done():
			return current, nil, core.NewError(core.CodeTimeout, ctx.Err().Error(), true)
		case <-time.After(pollInterval):
		}
	}
}

func (s *ActivationService) MarkMessageSent(ctx context.Context, activationID, requestID string) (core.Activation, error) {
	return s.applyAction(ctx, activationID, requestID, core.ActionMarkMessageSent, core.StatusMessageSent)
}

func (s *ActivationService) RequestAdditionalCode(ctx context.Context, activationID, requestID string) (core.Activation, error) {
	return s.applyAction(ctx, activationID, requestID, core.ActionRequestAdditional, core.StatusAdditionalCodeRequested)
}

func (s *ActivationService) CompleteActivation(ctx context.Context, activationID, requestID string) (core.Activation, error) {
	return s.applyAction(ctx, activationID, requestID, core.ActionCompleteActivation, core.StatusCompleted)
}

func (s *ActivationService) CancelActivation(ctx context.Context, activationID, requestID string) (core.Activation, error) {
	activation, err := s.store.Get(ctx, activationID)
	if err != nil {
		return core.Activation{}, err
	}
	provider, err := s.provider(activation.ProviderKey)
	if err != nil {
		return core.Activation{}, err
	}
	policy := provider.Policy().WithDefaults()
	now := s.clock.Now()
	if activation.Status.IsFinal() {
		return activation, core.NewError(core.CodeActivationAlreadyFinalized, "activation already finalized", false)
	}
	if activation.IsExpired(now) {
		activation.Status = core.StatusExpired
		activation.UpdatedAt = now
		_ = s.store.Update(ctx, activation)
		return activation, core.NewError(core.CodeActivationExpired, "activation expired", false)
	}
	age := now.Sub(activation.AcquiredAt)
	if age < policy.CancelAllowedAfter {
		return activation, core.NewError(core.CodeCancelNotAllowed, "activation is too new to cancel", true)
	}
	if policy.CancelAllowedUntil > 0 && age > policy.CancelAllowedUntil {
		return activation, core.NewError(core.CodeCancelNotAllowed, "activation is too old to cancel", false)
	}
	return s.applyAction(ctx, activationID, requestID, core.ActionCancelActivation, core.StatusCanceled)
}

func (s *ActivationService) applyAction(ctx context.Context, activationID, _ string, action core.ProviderAction, next core.ActivationStatus) (core.Activation, error) {
	activation, err := s.store.Get(ctx, activationID)
	if err != nil {
		return core.Activation{}, err
	}
	if err := s.expireIfNeeded(ctx, &activation); err != nil {
		return activation, err
	}
	if activation.Status.IsFinal() {
		return activation, core.NewError(core.CodeActivationAlreadyFinalized, "activation already finalized", false)
	}
	provider, err := s.provider(activation.ProviderKey)
	if err != nil {
		return core.Activation{}, err
	}
	if err := provider.SetStatus(ctx, activation.UpstreamActivationID, action); err != nil {
		activation.LastError = asCoreError(err)
		activation.UpdatedAt = s.clock.Now()
		_ = s.store.Update(ctx, activation)
		return activation, err
	}
	activation.Status = next
	activation.UpdatedAt = s.clock.Now()
	if err := s.store.Update(ctx, activation); err != nil {
		return core.Activation{}, err
	}
	return activation, nil
}

func (s *ActivationService) expireIfNeeded(ctx context.Context, activation *core.Activation) error {
	now := s.clock.Now()
	if !activation.IsExpired(now) {
		return nil
	}
	activation.Status = core.StatusExpired
	activation.UpdatedAt = now
	if err := s.store.Update(ctx, *activation); err != nil {
		return err
	}
	return core.NewError(core.CodeActivationExpired, "activation expired", false)
}

func (s *ActivationService) provider(key string) (core.Provider, error) {
	provider, ok := s.providers[key]
	if !ok {
		return nil, core.NewError(core.CodeRouteNotFound, "sms provider not registered", false)
	}
	return provider, nil
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func withRouteTargetDefaults(target core.Target, route core.Route) core.Target {
	if target.CountryISO2 == "" {
		target.CountryISO2 = route.CountryISO2
	}
	if target.CountryCallingCode == "" {
		target.CountryCallingCode = route.CountryCallingCode
	}
	if target.MaxPrice.AmountDecimal == "" && target.MaxPrice.CurrencyCode == "" {
		target.MaxPrice = route.MaxPrice
	}
	return target
}

func asCoreError(err error) *core.Error {
	if err == nil {
		return nil
	}
	if smsErr, ok := err.(*core.Error); ok {
		return smsErr
	}
	return core.NewError(core.CodeInternal, err.Error(), false)
}
