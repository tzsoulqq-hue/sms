import type { DecimalMoney } from '@/proto/byte/v/forge/contracts/sms/v1/sms';
import { SmsRouteSelectionStrategy, type SmsProviderConfig, type SmsProviderPolicy, type SmsRouteCandidate, type SmsRouteProfile } from '@/proto/byte/v/forge/sms/internal/v1/sms_internal';

export function newSmsProviderConfig(): SmsProviderConfig {
  return {
    provider_config_id: '',
    provider_key: '5sim',
    display_name: '',
    enabled: true,
    api_endpoint: '',
    credential_secret_ref: '',
    proxy_ref: '',
    default_target: undefined,
    capabilities: undefined,
    upstream_service_key: '',
    provider_country_id: '',
    credential_secret: '',
    http_proxy: '',
    credential_secret_set: false,
    policy: defaultSmsProviderPolicy('5sim'),
    labels: {},
    created_at: undefined,
    updated_at: undefined
  };
}

export function newSmsRouteProfile(): SmsRouteProfile {
  return {
    profile_key: '',
    display_name: '',
    enabled: true,
    selection_strategy: SmsRouteSelectionStrategy.SMS_ROUTE_SELECTION_STRATEGY_PRIORITY,
    preferred_provider_key: '',
    default_target: { application_key: '', country_iso2: '', country_calling_code: '', max_price: undefined },
    routes: [newSmsRouteCandidate()],
    labels: {},
    created_at: undefined,
    updated_at: undefined
  };
}

export function newSmsRouteCandidate(providerKey = 'smsbower'): SmsRouteCandidate {
  return {
    route_id: '',
    enabled: true,
    priority: 10,
    provider_config_id: '',
    provider_key: providerKey,
    upstream_service_key: '',
    provider_country_id: '',
    target: { application_key: '', country_iso2: '', country_calling_code: '', max_price: undefined },
    min_price: undefined,
    max_price: undefined,
    provider_options: {}
  };
}

export function moneyText(money?: DecimalMoney) {
  if (!money?.amount_decimal) return '-';
  return [money.currency_code, money.amount_decimal].filter(Boolean).join(' ');
}

export function defaultSmsProviderPolicy(providerType: string): SmsProviderPolicy {
  if (providerType === 'smsbower') {
    return { activation_ttl: '1500s', poll_interval: '5s', cancel_allowed_after: undefined, early_cancel_retry_after: '120s', cancel_allowed_until: undefined };
  }
  if (providerType === 'herosms') {
    return { activation_ttl: '1200s', poll_interval: '5s', cancel_allowed_after: '120s', early_cancel_retry_after: undefined, cancel_allowed_until: undefined };
  }
  return { activation_ttl: '1200s', poll_interval: '5s', cancel_allowed_after: undefined, early_cancel_retry_after: undefined, cancel_allowed_until: undefined };
}

export function durationSeconds(value?: string) {
  if (!value) return 0;
  const parsed = Number(String(value).replace(/s$/, ''));
  return Number.isFinite(parsed) ? Math.max(0, Math.round(parsed)) : 0;
}

export function secondsDuration(seconds: number) {
  const normalized = Math.max(0, Math.round(seconds || 0));
  return normalized > 0 ? `${normalized}s` : undefined;
}

export function strategyText(strategy?: SmsRouteSelectionStrategy) {
  const labels: Record<string, string> = {
    [SmsRouteSelectionStrategy.SMS_ROUTE_SELECTION_STRATEGY_PRIORITY]: '按优先级',
    [SmsRouteSelectionStrategy.SMS_ROUTE_SELECTION_STRATEGY_LOWEST_PRICE]: '最低价',
    [SmsRouteSelectionStrategy.SMS_ROUTE_SELECTION_STRATEGY_SPECIFIED_PROVIDER]: '指定Provider'
  };
  return labels[strategy || SmsRouteSelectionStrategy.SMS_ROUTE_SELECTION_STRATEGY_PRIORITY] || '-';
}

export function remainingText(expiresAt?: string) {
  if (!expiresAt) return '-';
  const seconds = Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000));
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${String(seconds % 60).padStart(2, '0')}`;
}

export function statusText(status?: string) {
  const labels: Record<string, string> = {
    SMS_ACTIVATION_STATUS_PENDING_CODE: '等待验证码',
    SMS_ACTIVATION_STATUS_MESSAGE_SENT: '已触发短信',
    SMS_ACTIVATION_STATUS_CODE_RECEIVED: '已收到',
    SMS_ACTIVATION_STATUS_ADDITIONAL_CODE_REQUESTED: '重发中',
    SMS_ACTIVATION_STATUS_COMPLETED: '已完成',
    SMS_ACTIVATION_STATUS_CANCELED: '已取消',
    SMS_ACTIVATION_STATUS_EXPIRED: '已过期',
    SMS_ACTIVATION_STATUS_FAILED: '失败'
  };
  return labels[status || ''] || status || '-';
}

export function canCancelStatus(status?: string) {
  return !['SMS_ACTIVATION_STATUS_COMPLETED', 'SMS_ACTIVATION_STATUS_CANCELED', 'SMS_ACTIVATION_STATUS_EXPIRED', 'SMS_ACTIVATION_STATUS_FAILED'].includes(status || '');
}
