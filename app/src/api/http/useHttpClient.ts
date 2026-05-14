import { useContext } from 'react';
import { HttpClientContext, type HttpClientContextType } from './HttpClientContext';

export function useHttpClient(): HttpClientContextType {
  const context = useContext(HttpClientContext);
  if (!context) throw new Error('useHttpClient must be used within a HttpClientProvider');
  return context;
}
