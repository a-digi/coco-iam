import { createContext } from 'react';

export type HttpClientContextType = {
  get: <T = unknown>(endpoint: string, options?: RequestInit) => Promise<T>;
  post: <T = unknown>(endpoint: string, options?: unknown) => Promise<T>;
  postMultipart: <T = unknown>(endpoint: string, options: FormData) => Promise<T>;
  postPublicApi: <T = unknown>(endpoint: string, options?: unknown) => Promise<T>;
  put: <T = unknown>(endpoint: string, options?: unknown) => Promise<T>;
  patch: <T = unknown>(endpoint: string, options?: unknown) => Promise<T>;
  del: <T = unknown>(endpoint: string, options?: RequestInit) => Promise<T>;
};

export const HttpClientContext = createContext<HttpClientContextType | undefined>(undefined);
