import { useState, type ReactNode } from 'react';
import { SidebarContext } from './SidebarContextContext';

export const SidebarProvider = ({ children }: { children: ReactNode }) => {
  const [open, setOpen] = useState(() => window.innerWidth >= 768);
  const toggle = () => setOpen((o) => !o);
  return (
    <SidebarContext.Provider value={{ open, toggle, setOpen }}>
      {children}
    </SidebarContext.Provider>
  );
};
