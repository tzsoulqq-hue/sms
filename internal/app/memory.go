package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/byte-v-forge/sms/internal/core"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type MemoryActivationStore struct {
	mu          sync.RWMutex
	activations map[string]core.Activation
}

func NewMemoryActivationStore() *MemoryActivationStore {
	return &MemoryActivationStore{activations: make(map[string]core.Activation)}
}

func (s *MemoryActivationStore) Save(_ context.Context, activation core.Activation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activations[activation.ID] = activation
	return nil
}

func (s *MemoryActivationStore) Get(_ context.Context, activationID string) (core.Activation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	activation, ok := s.activations[activationID]
	if !ok {
		return core.Activation{}, core.NewError(core.CodeActivationNotFound, "activation not found", false)
	}
	return activation, nil
}

func (s *MemoryActivationStore) Update(_ context.Context, activation core.Activation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.activations[activation.ID]; !ok {
		return core.NewError(core.CodeActivationNotFound, "activation not found", false)
	}
	s.activations[activation.ID] = activation
	return nil
}

type StaticRouteResolver struct {
	routes []core.Route
}

func NewStaticRouteResolver(routes []core.Route) *StaticRouteResolver {
	return &StaticRouteResolver{routes: append([]core.Route(nil), routes...)}
}

func (r *StaticRouteResolver) Resolve(_ context.Context, target core.Target) (core.Route, error) {
	for _, route := range r.routes {
		if !sameFoldOrEmpty(route.ApplicationKey, target.ApplicationKey) {
			continue
		}
		if !sameFoldOrEmpty(route.CountryISO2, target.CountryISO2) {
			continue
		}
		if route.CountryCallingCode != "" && target.CountryCallingCode != "" && route.CountryCallingCode != target.CountryCallingCode {
			continue
		}
		return route, nil
	}
	return core.Route{}, core.NewError(core.CodeRouteNotFound, "no sms route for target", false)
}

func sameFoldOrEmpty(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)
	return expected == "" || actual == "" || strings.EqualFold(expected, actual)
}
