import type { RouteAdapter } from './types';

export const smsbowerRouteAdapter: RouteAdapter = {
  providerKey: 'smsbower',
  fields: [
    { key: 'upstream_service_key', label: 'Service', scope: 'route', options: 'services' },
    { key: 'provider_country_id', label: 'Country', scope: 'route', options: 'countries' },
    { key: 'amount_decimal', label: 'Min Price', scope: 'minPrice' },
    { key: 'amount_decimal', label: 'Max Price', scope: 'maxPrice' },
    { key: 'ref', label: 'Ref', scope: 'option' },
    { key: 'include_provider_ids', label: 'Include Providers', scope: 'option' },
    { key: 'exclude_provider_ids', label: 'Exclude Providers', scope: 'option' },
    { key: 'phone_exception_prefixes', label: 'Blocked Prefixes', scope: 'option' }
  ]
};
