import { createContext, useContext } from 'react';

export interface SidebarContextProps {
  open: boolean;
  toggle: () => void;
  setOpen: (open: boolean) => void;
}

export const SidebarContext = createContext<SidebarContextProps | undefined>(undefined);

export const useSidebar = () => {
  const ctx = useContext(SidebarContext);
  if (!ctx) throw new Error('useSidebar must be used within SidebarProvider');
  return ctx;
};
