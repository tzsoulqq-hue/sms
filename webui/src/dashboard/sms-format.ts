import type { DecimalMoney } from '@/proto/byte/v/forge/contracts/sms/v1/sms';
import type { SmsProviderConfig } from '@/proto/byte/v/forge/sms/internal/v1/sms_internal';

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
    labels: {},
    created_at: undefined,
    updated_at: undefined
  };
}

export function moneyText(money?: DecimalMoney) {
  if (!money?.amount_decimal) return '-';
  return [money.currency_code, money.amount_decimal].filter(Boolean).join(' ');
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
