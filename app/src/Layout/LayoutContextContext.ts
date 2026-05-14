import { createContext, useContext, type ReactNode } from 'react';

export interface LayoutContextType {
  setSidebarContent: (content: ReactNode) => void;
  setTopbarContent: (content: ReactNode) => void;
  setMainContent: (content: ReactNode) => void;
}

export const LayoutContext = createContext<LayoutContextType | undefined>(undefined);

export const useLayout = () => {
  const context = useContext(LayoutContext);
  if (!context) {
    throw new Error('useLayout must be used within a LayoutProvider');
  }
  return context;
};
