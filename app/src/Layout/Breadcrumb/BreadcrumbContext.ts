import { createContext } from 'react';

export interface BreadcrumbItem {
  label: string;
  href?: string;
}

export interface BreadcrumbContextProps {
  items: BreadcrumbItem[];
  setItems: (items: BreadcrumbItem[]) => void;
}

export const BreadcrumbContext = createContext<BreadcrumbContextProps | undefined>(undefined);
