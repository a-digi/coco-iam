import React from 'react';
import { useAuth } from '../../Components/Auth/Guard/useAuth';

interface ContentProps {
  children?: React.ReactNode;
}

const Content: React.FC<ContentProps> = ({ children }) => {
  const { authenticated } = useAuth();
  const className = authenticated ? 'flex-1 mt-2 p-4 overflow-auto m-0' : 'flex-1 overflow-auto m-0';
  return <main className={className}>{children}</main>;
};

export default Content;
