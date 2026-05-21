export type RouteOptionKey = 'services' | 'countries' | 'operators' | 'upstream_providers';

type BaseRouteField = {
  label: string;
  options?: RouteOptionKey;
};

export type RouteField =
  | (BaseRouteField & { key: 'upstream_service_key' | 'provider_country_id'; scope: 'route' })
  | (BaseRouteField & { key: string; scope: 'option' })
  | (BaseRouteField & { key: 'amount_decimal'; scope: 'maxPrice'; options?: undefined });

export type RouteAdapter = {
  providerKey: string;
  fields: RouteField[];
};
