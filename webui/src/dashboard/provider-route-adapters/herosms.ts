import type { RouteAdapter } from './types';

export const herosmsRouteAdapter: RouteAdapter = {
  providerKey: 'herosms',
  fields: [
    { key: 'upstream_service_key', label: 'Service', scope: 'route', options: 'services' },
    { key: 'provider_country_id', label: 'Country', scope: 'route', options: 'countries' },
    { key: 'amount_decimal', label: 'Max Price', scope: 'maxPrice' }
  ]
};
