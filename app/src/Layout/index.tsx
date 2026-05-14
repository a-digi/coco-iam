import type { ReactNode } from 'react';
import { LayoutProvider } from './LayoutContext';

interface LayoutProps {
  children: ReactNode;
}

const Layout = ({ children }: LayoutProps) => {
  return <LayoutProvider>{children}</LayoutProvider>;
};

export default Layout;
