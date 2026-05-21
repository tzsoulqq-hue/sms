import type { RouteAdapter } from './types';

export const fivesimRouteAdapter: RouteAdapter = {
  providerKey: '5sim',
  fields: [
    { key: 'upstream_service_key', label: 'Product', scope: 'route', options: 'services' },
    { key: 'provider_country_id', label: 'Country', scope: 'route', options: 'countries' },
    { key: 'operator', label: 'Operator', scope: 'option', options: 'operators' },
    { key: 'amount_decimal', label: 'Max Price', scope: 'maxPrice' },
    { key: 'reuse', label: 'Reuse', scope: 'option' },
    { key: 'voice', label: 'Voice', scope: 'option' },
    { key: 'ref', label: 'Ref', scope: 'option' }
  ]
};
