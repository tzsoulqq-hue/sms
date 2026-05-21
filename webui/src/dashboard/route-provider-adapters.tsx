import { Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/dashboard/module-kit';
import type { SmsProviderRouteOptions, SmsRouteCandidate, SmsRouteOption } from '@/proto/byte/v/forge/sms/internal/v1/sms_internal';
import { routeAdapters, type RouteField } from './provider-route-adapters';

type Props = {
  route: SmsRouteCandidate;
  options?: SmsProviderRouteOptions;
  onChange: (route: SmsRouteCandidate) => void;
};

export function ProviderRouteFields({ route, options, onChange }: Props) {
  const adapter = routeAdapters[route.provider_key];
  return (
    <div className="grid grid-cols-2 gap-2">
      {(adapter?.fields || []).map((field) => (
        <FieldInput key={`${field.scope}-${field.key}`} field={field} route={route} options={options} onChange={onChange} />
      ))}
    </div>
  );
}

function FieldInput({ field, route, options, onChange }: Props & { field: RouteField }) {
  const value = readField(route, field);
  const choices = field.options ? options?.[field.options] || [] : [];
  return (
    <div className="grid gap-1">
      <Label>{field.label}</Label>
      {field.options ? (
        <Select value={value || undefined} onValueChange={(next) => onChange(writeChoice(route, field, choices.find((item) => item.value === next)))}>
          <SelectTrigger><SelectValue placeholder={choices.length ? '选择' : '无可选项'} /></SelectTrigger>
          <SelectContent>{choices.map((item) => <SelectItem key={item.value} value={item.value}>{item.label || item.value}</SelectItem>)}</SelectContent>
        </Select>
      ) : <Input value={value} onChange={(event) => onChange(writeField(route, field, event.target.value))} />}
    </div>
  );
}

function readField(route: SmsRouteCandidate, field: RouteField) {
  if (field.scope === 'option') return route.provider_options?.[field.key] || '';
  if (field.scope === 'maxPrice') return route.max_price?.amount_decimal || '';
  return route[field.key] || '';
}

function writeChoice(route: SmsRouteCandidate, field: RouteField, choice?: SmsRouteOption): SmsRouteCandidate {
  if (!choice) return route;
  const next = writeField(route, field, choice.value);
  if (field.options === 'countries') {
    next.target = {
      application_key: next.target?.application_key || '',
      country_iso2: choice.metadata?.country_iso2 || next.target?.country_iso2 || '',
      country_calling_code: choice.metadata?.country_calling_code || next.target?.country_calling_code || '',
      max_price: next.target?.max_price
    };
  }
  if (field.options === 'services' && choice.metadata?.application_key) {
    next.target = { application_key: choice.metadata.application_key, country_iso2: next.target?.country_iso2 || '', country_calling_code: next.target?.country_calling_code || '', max_price: next.target?.max_price };
  }
  return next;
}

function writeField(route: SmsRouteCandidate, field: RouteField, value: string): SmsRouteCandidate {
  if (field.scope === 'option') {
    return { ...route, provider_options: { ...(route.provider_options || {}), [field.key]: value } };
  }
  if (field.scope === 'maxPrice') {
    return { ...route, max_price: { currency_code: route.max_price?.currency_code || 'USD', amount_decimal: value } };
  }
  if (field.key === 'upstream_service_key') return { ...route, upstream_service_key: value };
  return { ...route, provider_country_id: value };
}
