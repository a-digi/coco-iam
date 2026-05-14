import { useState, type ReactNode } from 'react';
import { BreadcrumbContext, type BreadcrumbItem } from './BreadcrumbContext';

export const BreadcrumbProvider = ({ children }: { children: ReactNode }) => {
  const [items, setItems] = useState<BreadcrumbItem[]>([]);

  return (
    <BreadcrumbContext.Provider value={{ items, setItems }}>
      {children}
    </BreadcrumbContext.Provider>
  );
};
