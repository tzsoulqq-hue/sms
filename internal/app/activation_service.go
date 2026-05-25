package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
)

const (
	cancelRequestedLabel = "cancel_requested"
	cancelRequestIDLabel = "cancel_request_id"
)

type cancelRequestedActivationStore interface {
	ListCancelRequested(context.Context, time.Time, int) ([]core.Activation, error)
}

type ActivationService struct {
	store     core.ActivationStore
	routes    core.RouteResolver
	providers map[string]core.Provider
	clock     core.Clock
	ids       core.IDGenerator
}

type profileRouteCandidateResolver interface {
	ResolveProfileCandidates(context.Context, core.RouteRequest) ([]core.Route, error)
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
	if cmd.ProfileKey == "" && cmd.Target.ApplicationKey == "" {
		return core.Activation{}, core.NewError(core.CodeValidationFailed, "profile_key or application_key is required", false)
	}
	if resolver, ok := s.routes.(profileRouteCandidateResolver); ok && cmd.ProfileKey != "" {
		return s.acquireNumberFromProfileCandidates(ctx, cmd, resolver)
	}
	route, err := s.routes.Resolve(ctx, core.RouteRequest{
		ProfileKey:       cmd.ProfileKey,
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
	policy := providerPolicyForUpstreamActivation(provider, providerActivation.UpstreamActivationID).WithDefaults()
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

func (s *ActivationService) acquireNumberFromProfileCandidates(ctx context.Context, cmd core.AcquireNumberCommand, resolver profileRouteCandidateResolver) (core.Activation, error) {
	routes, err := resolver.ResolveProfileCandidates(ctx, core.RouteRequest{
		ProfileKey:       cmd.ProfileKey,
		Target:           cmd.Target,
		ProviderKey:      cmd.ProviderKey,
		ProviderConfigID: cmd.ProviderConfigID,
	})
	if err != nil {
		return core.Activation{}, err
	}
	if len(routes) == 0 {
		return core.Activation{}, core.NewError(core.CodeRouteNotFound, "sms route profile has no matching route", false)
	}
	var lastErr error
	for _, route := range routes {
		attempt := cmd
		attempt.Target = withRouteTargetDefaults(attempt.Target, route)
		activation, err := s.acquireNumberWithRoute(ctx, attempt, route)
		if err == nil {
			return activation, nil
		}
		lastErr = err
		if !shouldTryNextRouteOnAcquireError(err) {
			return core.Activation{}, err
		}
	}
	if lastErr != nil {
		return core.Activation{}, lastErr
	}
	return core.Activation{}, core.NewError(core.CodeNoNumberAvailable, "no upstream number available", true)
}

func (s *ActivationService) acquireNumberWithRoute(ctx context.Context, cmd core.AcquireNumberCommand, route core.Route) (core.Activation, error) {
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
	policy := providerPolicyForUpstreamActivation(provider, providerActivation.UpstreamActivationID).WithDefaults()
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

func shouldTryNextRouteOnAcquireError(err error) bool {
	var smsErr *core.Error
	if !errors.As(err, &smsErr) || smsErr == nil {
		return false
	}
	switch smsErr.Code {
	case core.CodeNoNumberAvailable, core.CodeSupplyUnavailable, core.CodeRouteNotFound, core.CodePriceLimitExceeded:
		return smsErr.Retryable || smsErr.Code == core.CodeNoNumberAvailable || smsErr.Code == core.CodePriceLimitExceeded
	default:
		return false
	}
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
	bindActivationProviderConfig(provider, activation)
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
	case core.StatusPendingCode, core.StatusAdditionalCodeRequested:
		activation.Status = result.Status
		activation.Code = nil
	case core.StatusCanceled, core.StatusFailed, core.StatusExpired:
		activation.Status = result.Status
	}
	if err := s.store.Update(ctx, activation); err != nil {
		return core.Activation{}, nil, err
	}
	return activation, nil, nil
}

func (s *ActivationService) WaitForCode(ctx context.Context, activationID string, timeout, pollInterval time.Duration, issuedAfterUnix int64) (core.Activation, *core.SMSCode, error) {
	activation, err := s.store.Get(ctx, activationID)
	if err != nil {
		return core.Activation{}, nil, err
	}
	provider, err := s.provider(activation.ProviderKey)
	if err != nil {
		return core.Activation{}, nil, err
	}
	bindActivationProviderConfig(provider, activation)
	policy := providerPolicyForActivation(ctx, provider, activation).WithDefaults()
	if timeout <= 0 {
		timeout = time.Until(activation.ExpiresAt)
	}
	if pollInterval <= 0 {
		pollInterval = policy.PollInterval
	}
	var issuedAfter time.Time
	if issuedAfterUnix > 0 {
		issuedAfter = time.Unix(issuedAfterUnix, 0)
	}
	deadline := s.clock.Now().Add(timeout)
	for {
		current, code, err := s.CheckCode(ctx, activationID)
		if err != nil {
			return current, nil, err
		}
		if code != nil && code.Value != "" {
			if issuedAfter.IsZero() || !code.ReceivedAt.Before(issuedAfter) {
				return current, code, nil
			}
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
	return s.cancelLoadedActivation(ctx, activation, requestID)
}

func (s *ActivationService) cancelLoadedActivation(ctx context.Context, activation core.Activation, requestID string) (core.Activation, error) {
	provider, err := s.provider(activation.ProviderKey)
	if err != nil {
		return core.Activation{}, err
	}
	bindActivationProviderConfig(provider, activation)
	policy := providerPolicyForActivation(ctx, provider, activation).WithDefaults()
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
	if policy.CancelAllowedAfter > 0 && age < policy.CancelAllowedAfter {
		return s.queueCancelRetry(ctx, activation, requestID, activation.AcquiredAt.Add(policy.CancelAllowedAfter))
	}
	if policy.CancelAllowedUntil > 0 && age > policy.CancelAllowedUntil {
		return activation, core.NewError(core.CodeCancelNotAllowed, "activation is too old to cancel", false)
	}
	if err := provider.SetStatus(ctx, activation.UpstreamActivationID, core.ActionCancelActivation); err != nil {
		smsErr := asCoreError(err)
		if shouldQueueEarlyCancelRetry(smsErr, policy) {
			return s.queueCancelRetry(ctx, activation, requestID, earlyCancelRetryAt(activation, policy, now))
		}
		activation.LastError = smsErr
		activation.UpdatedAt = now
		activation.Labels = clearCancelRequested(activation.Labels)
		_ = s.store.Update(ctx, activation)
		return activation, err
	}
	activation.Status = core.StatusCanceled
	activation.UpdatedAt = now
	activation.LastError = nil
	activation.CancelAllowedAt = time.Time{}
	activation.Labels = clearCancelRequested(activation.Labels)
	if err := s.store.Update(ctx, activation); err != nil {
		return core.Activation{}, err
	}
	return activation, nil
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
	bindActivationProviderConfig(provider, activation)
	if err := provider.SetStatus(ctx, activation.UpstreamActivationID, action); err != nil {
		activation.LastError = asCoreError(err)
		activation.UpdatedAt = s.clock.Now()
		_ = s.store.Update(ctx, activation)
		return activation, err
	}
	activation.Status = next
	activation.UpdatedAt = s.clock.Now()
	if action == core.ActionMarkMessageSent || action == core.ActionRequestAdditional {
		activation.Code = nil
	}
	if err := s.store.Update(ctx, activation); err != nil {
		return core.Activation{}, err
	}
	return activation, nil
}

func bindActivationProviderConfig(provider core.Provider, activation core.Activation) {
	configured, ok := provider.(*ConfiguredProvider)
	if !ok {
		return
	}
	configured.BindActivationConfig(activation.UpstreamActivationID, activation.ProviderConfigID)
}

func providerPolicyForActivation(ctx context.Context, provider core.Provider, activation core.Activation) core.ProviderPolicy {
	if configured, ok := provider.(*ConfiguredProvider); ok {
		return configured.LoadPolicyForActivation(ctx, activation.UpstreamActivationID, activation.ProviderConfigID)
	}
	return providerPolicyForUpstreamActivation(provider, activation.UpstreamActivationID)
}

func providerPolicyForUpstreamActivation(provider core.Provider, upstreamActivationID string) core.ProviderPolicy {
	if configured, ok := provider.(*ConfiguredProvider); ok {
		return configured.PolicyForActivation(upstreamActivationID)
	}
	return provider.Policy()
}

func shouldQueueEarlyCancelRetry(err *core.Error, policy core.ProviderPolicy) bool {
	return err != nil && err.Code == core.CodeCancelNotAllowed && err.Retryable && policy.EarlyCancelRetryAfter > 0
}

func earlyCancelRetryAt(activation core.Activation, policy core.ProviderPolicy, now time.Time) time.Time {
	if !activation.AcquiredAt.IsZero() && policy.EarlyCancelRetryAfter > 0 {
		retryAt := activation.AcquiredAt.Add(policy.EarlyCancelRetryAfter)
		if retryAt.After(now) {
			return retryAt
		}
	}
	delay := policy.PollInterval
	if delay <= 0 {
		delay = 5 * time.Second
	}
	return now.Add(delay)
}

func (s *ActivationService) queueCancelRetry(ctx context.Context, activation core.Activation, requestID string, retryAt time.Time) (core.Activation, error) {
	if retryAt.IsZero() {
		retryAt = s.clock.Now().Add(5 * time.Second)
	}
	if !retryAt.After(s.clock.Now()) {
		retryAt = s.clock.Now().Add(5 * time.Second)
	}
	activation.UpdatedAt = s.clock.Now()
	activation.CancelAllowedAt = retryAt
	activation.LastError = nil
	activation.Labels = markCancelRequested(activation.Labels, requestID)
	if err := s.store.Update(ctx, activation); err != nil {
		return core.Activation{}, err
	}
	s.scheduleCancelAttempt(activation.ID, retryAt)
	return activation, nil
}

func markCancelRequested(labels map[string]string, requestID string) map[string]string {
	out := cloneMap(labels)
	if out == nil {
		out = map[string]string{}
	}
	out[cancelRequestedLabel] = "true"
	if requestID != "" {
		out[cancelRequestIDLabel] = requestID
	}
	return out
}

func clearCancelRequested(labels map[string]string) map[string]string {
	out := cloneMap(labels)
	if out == nil {
		return nil
	}
	delete(out, cancelRequestedLabel)
	delete(out, cancelRequestIDLabel)
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *ActivationService) scheduleCancelAttempt(activationID string, retryAt time.Time) {
	delay := time.Until(retryAt)
	if delay < time.Second {
		delay = time.Second
	}
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.CancelActivation(ctx, activationID, s.ids.NewID("req_")); err != nil {
			log.Printf("scheduled sms activation cancel failed: activation_id=%s error=%v", activationID, err)
		}
	}()
}

func (s *ActivationService) StartCancelScheduler(ctx context.Context, interval time.Duration) {
	if _, ok := s.store.(cancelRequestedActivationStore); !ok {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		s.runDueCancelRequests(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runDueCancelRequests(ctx)
			}
		}
	}()
}

func (s *ActivationService) runDueCancelRequests(ctx context.Context) {
	store, ok := s.store.(cancelRequestedActivationStore)
	if !ok {
		return
	}
	activations, err := store.ListCancelRequested(ctx, s.clock.Now(), 100)
	if err != nil {
		log.Printf("list scheduled sms activation cancels failed: %v", err)
		return
	}
	for _, activation := range activations {
		if _, err := s.cancelLoadedActivation(ctx, activation, s.ids.NewID("req_")); err != nil {
			log.Printf("scheduled sms activation cancel failed: activation_id=%s error=%v", activation.ID, err)
		}
	}
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
	if target.ApplicationKey == "" {
		target.ApplicationKey = route.ApplicationKey
	}
	if target.CountryISO2 == "" {
		target.CountryISO2 = route.CountryISO2
	}
	if target.CountryCallingCode == "" {
		target.CountryCallingCode = route.CountryCallingCode
	}
	if target.MinPrice.AmountDecimal == "" && target.MinPrice.CurrencyCode == "" {
		target.MinPrice = route.MinPrice
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
