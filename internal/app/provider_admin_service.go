package app

import (
	"context"
	"strings"
	"time"

	smsinternalv1 "github.com/byte-v-forge/sms/gen/go/byte/v/forge/sms/private/v1"
	"github.com/byte-v-forge/sms/internal/core"
	"google.golang.org/protobuf/proto"
)

type ProviderAdminService struct {
	configs      ProviderConfigStore
	activations  *ActivationService
	activationDB ActivationListStore
	timeout      time.Duration
}

func NewProviderAdminService(configs ProviderConfigStore, activations *ActivationService, activationDB ActivationListStore, timeout time.Duration) *ProviderAdminService {
	return &ProviderAdminService{configs: configs, activations: activations, activationDB: activationDB, timeout: timeout}
}

func (s *ProviderAdminService) UpsertProviderConfig(ctx context.Context, config *smsinternalv1.SmsProviderConfig) (*smsinternalv1.SmsProviderConfig, error) {
	saved, err := s.configs.UpsertProviderConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	return RedactProviderConfig(saved), nil
}

func (s *ProviderAdminService) GetProviderConfig(ctx context.Context, providerConfigID string) (*smsinternalv1.SmsProviderConfig, error) {
	config, err := s.configs.GetProviderConfig(ctx, providerConfigID)
	if err != nil {
		return nil, err
	}
	return RedactProviderConfig(config), nil
}

func (s *ProviderAdminService) ListProviderConfigs(ctx context.Context, includeDisabled bool, providerKey string) ([]*smsinternalv1.SmsProviderConfig, error) {
	configs, err := s.configs.ListProviderConfigs(ctx, includeDisabled, providerKey)
	if err != nil {
		return nil, err
	}
	for index, config := range configs {
		configs[index] = RedactProviderConfig(config)
	}
	return configs, nil
}

func (s *ProviderAdminService) DeleteProviderConfig(ctx context.Context, providerConfigID string) error {
	return s.configs.DeleteProviderConfig(ctx, providerConfigID)
}

func (s *ProviderAdminService) GetProviderBalance(ctx context.Context, providerConfigID string) (core.Money, error) {
	config, err := s.configs.GetProviderConfig(ctx, providerConfigID)
	if err != nil {
		return core.Money{}, err
	}
	if !config.GetEnabled() {
		return core.Money{}, core.NewError(core.CodeValidationFailed, "sms provider config is disabled", false)
	}
	provider, err := providerFromConfig(config, s.timeout)
	if err != nil {
		return core.Money{}, err
	}
	return provider.GetBalance(ctx)
}

func (s *ProviderAdminService) ListActivations(ctx context.Context, includeFinal bool, limit int) ([]core.Activation, error) {
	if s.activationDB == nil {
		return nil, core.NewError(core.CodeUnsupportedOperation, "sms activation list is not available", false)
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return s.activationDB.List(ctx, includeFinal, limit)
}

func (s *ProviderAdminService) CancelActivation(ctx context.Context, activationID string, requestID string) (core.Activation, error) {
	if strings.TrimSpace(activationID) == "" {
		return core.Activation{}, core.NewError(core.CodeValidationFailed, "activation_id is required", false)
	}
	if strings.TrimSpace(requestID) == "" {
		requestID = RandomIDGenerator{}.NewID("req_")
	}
	return s.activations.CancelActivation(ctx, activationID, requestID)
}

func RedactProviderConfig(config *smsinternalv1.SmsProviderConfig) *smsinternalv1.SmsProviderConfig {
	if config == nil {
		return nil
	}
	redacted := proto.Clone(config).(*smsinternalv1.SmsProviderConfig)
	redacted.CredentialSecretSet = strings.TrimSpace(config.GetCredentialSecret()) != "" || config.GetCredentialSecretSet()
	redacted.CredentialSecret = ""
	return redacted
}
