import type { ReactNode, ComponentType } from 'react';

export type RouteConfig = {
  path: string;
  element?: ReactNode;
  component?: ComponentType<Record<string, unknown>>;
  children?: RouteConfig[];
};
