package core

import (
	"context"
	"time"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID(prefix string) string
}

type RouteResolver interface {
	Resolve(ctx context.Context, target Target) (Route, error)
}

type ActivationStore interface {
	Save(ctx context.Context, activation Activation) error
	Get(ctx context.Context, activationID string) (Activation, error)
	Update(ctx context.Context, activation Activation) error
}

type Provider interface {
	Key() string
	Policy() ProviderPolicy
	AcquireNumber(ctx context.Context, request ProviderAcquireRequest) (ProviderActivation, error)
	GetStatus(ctx context.Context, upstreamActivationID string) (ProviderCodeResult, error)
	SetStatus(ctx context.Context, upstreamActivationID string, action ProviderAction) error
	GetBalance(ctx context.Context) (Money, error)
}
