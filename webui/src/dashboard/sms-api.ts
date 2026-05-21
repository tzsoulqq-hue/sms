import { api } from '@/dashboard/module-kit';
import type {
  CancelProviderActivationResponse,
  DeleteProviderConfigResponse,
  GetProviderBalanceResponse,
  ListActivationsResponse,
  ListProviderConfigsResponse,
  SmsProviderConfig,
  UpsertProviderConfigResponse
} from '@/proto/byte/v/forge/sms/internal/v1/sms_internal';

export const smsKeys = {
  configs: ['sms', 'provider-configs'] as const,
  activations: ['sms', 'activations'] as const,
  balance: (id: string) => ['sms', 'balance', id] as const
};

export function listSmsProviderConfigs() {
  return api<ListProviderConfigsResponse>('/api/sms/provider-configs?include_disabled=true');
}

export function saveSmsProviderConfig(config: SmsProviderConfig) {
  return api<UpsertProviderConfigResponse>('/api/sms/provider-configs', {
    method: 'POST',
    body: JSON.stringify({ config })
  });
}

export function deleteSmsProviderConfig(id: string) {
  return api<DeleteProviderConfigResponse>(`/api/sms/provider-configs/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export function getSmsProviderBalance(id: string) {
  return api<GetProviderBalanceResponse>(`/api/sms/provider-configs/${encodeURIComponent(id)}/balance`);
}

export function listSmsActivations() {
  return api<ListActivationsResponse>('/api/sms/activations?limit=200');
}

export function cancelSmsActivation(id: string) {
  return api<CancelProviderActivationResponse>(`/api/sms/activations/${encodeURIComponent(id)}/cancel`, {
    method: 'POST',
    body: JSON.stringify({})
  });
}
