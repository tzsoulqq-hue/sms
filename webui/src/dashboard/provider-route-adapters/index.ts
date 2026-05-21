import { fivesimRouteAdapter } from './fivesim';
import { herosmsRouteAdapter } from './herosms';
import { smsbowerRouteAdapter } from './smsbower';
import type { RouteAdapter } from './types';

export const routeAdapters: Record<string, RouteAdapter> = {
  [fivesimRouteAdapter.providerKey]: fivesimRouteAdapter,
  [herosmsRouteAdapter.providerKey]: herosmsRouteAdapter,
  [smsbowerRouteAdapter.providerKey]: smsbowerRouteAdapter
};

export type { RouteAdapter, RouteField } from './types';
