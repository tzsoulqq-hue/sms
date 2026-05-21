import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { Save, Trash2 } from 'lucide-react';
import { Button, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/dashboard/module-kit';
import type { SmsProviderConfig } from '@/proto/byte/v/forge/sms/internal/v1/sms_internal';
import { newSmsProviderConfig } from './sms-format';

type FormProps = {
  config: SmsProviderConfig | null;
  saving?: boolean;
  deleting?: boolean;
  onSave: (config: SmsProviderConfig) => void;
  onDelete: (id: string) => void;
};

export function ProviderConfigForm({ config, saving, deleting, onSave, onDelete }: FormProps) {
  const [draft, setDraft] = useState<SmsProviderConfig>(() => config || newSmsProviderConfig());
  useEffect(() => setDraft(config || newSmsProviderConfig()), [config]);
  const providerType = draft.provider_key || '5sim';

  function patch(next: Partial<SmsProviderConfig>) {
    setDraft((current) => ({ ...current, ...next }));
  }

  function patchProviderType(value: string) {
    patch({ provider_key: value, provider_config_id: value, display_name: providerLabel(value), enabled: true });
  }

  function save() {
    onSave({
      ...draft,
      provider_config_id: providerType,
      provider_key: providerType,
      display_name: providerLabel(providerType),
      enabled: true,
      api_endpoint: '',
      credential_secret_ref: '',
      proxy_ref: '',
      default_target: undefined,
      upstream_service_key: '',
      provider_country_id: '',
      http_proxy: '',
      labels: {}
    });
  }

  return (
    <div className="flex min-h-0 flex-col gap-3 border-l border-border/70 p-3">
      <div className="text-sm font-semibold">Provider配置</div>
      <div className="grid gap-2">
        <Field label="Provider Type">
          <Select value={providerType} onValueChange={patchProviderType}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="5sim">5sim</SelectItem>
              <SelectItem value="smsbower">SMSBower</SelectItem>
              <SelectItem value="herosms">HeroSMS</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <Field label="API Key"><Input type="password" placeholder={draft.credential_secret_set ? '留空则保留现有密钥' : ''} value={draft.credential_secret} onChange={(e) => patch({ credential_secret: e.target.value })} /></Field>
      </div>
      <div className="mt-auto flex gap-2">
        <Button className="flex-1" disabled={saving} onClick={save}><Save className="size-4" />保存</Button>
        <Button variant="outline" size="icon" disabled={!draft.provider_config_id || deleting} onClick={() => onDelete(draft.provider_config_id)}><Trash2 className="size-4" /></Button>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <div className="grid gap-1"><Label>{label}</Label>{children}</div>;
}

function providerLabel(providerType: string) {
  const labels: Record<string, string> = { '5sim': '5sim', herosms: 'HeroSMS', smsbower: 'SMSBower' };
  return labels[providerType] || providerType;
}
