import { useContext, useEffect } from 'react';
import { BreadcrumbContext, type BreadcrumbItem } from './BreadcrumbContext';

export const useBreadcrumb = () => {
  const ctx = useContext(BreadcrumbContext);
  if (!ctx) throw new Error('useBreadcrumb must be used inside BreadcrumbProvider');
  return ctx;
};

/** Convenience hook — call once per page to declare breadcrumb items. */
export const useBreadcrumbItems = (items: BreadcrumbItem[]) => {
  const { setItems } = useBreadcrumb();
  useEffect(() => {
    setItems(items);
    return () => setItems([]);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
};
