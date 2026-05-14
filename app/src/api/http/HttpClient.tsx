import type { ReactNode } from 'react';
import { useRef } from 'react';
import * as client from '../client';
import { get as getApi } from '../get';
import { HttpClientContext } from './HttpClientContext';

export function HttpClientProvider({ children }: { children: ReactNode }) {
  const recentRequests = useRef<Record<string, { timestamp: number; promise: Promise<unknown> }>>({});
  const DEDUPLICATION_WINDOW_MS = 100;

  function deduplicated<T>(key: string, fn: () => Promise<T>): Promise<T> {
    const now = Date.now();
    const recent = recentRequests.current[key];

    if (recent && now - recent.timestamp < DEDUPLICATION_WINDOW_MS) {
      return recent.promise as Promise<T>;
    }

    const promise = fn();
    recentRequests.current[key] = { timestamp: now, promise };

    promise.finally(() => {
      if (recentRequests.current[key]?.promise === promise) {
        delete recentRequests.current[key];
      }
    });

    return promise;
  }

  function get<T = unknown>(endpoint: string, options?: RequestInit): Promise<T> {
    const key = `get:${endpoint}:${JSON.stringify(options)}`;
    return deduplicated(key, () => getApi(endpoint, options).then(res => res as T));
  }

  function post<T = unknown>(endpoint: string, options?: unknown): Promise<T> {
    const key = `post:${endpoint}:${JSON.stringify(options)}`;
    return deduplicated(key, () => client.post(endpoint, options) as Promise<T>);
  }

  function postMultipart<T = unknown>(endpoint: string, options: FormData): Promise<T> {
    const key = `postMultipart:${endpoint}:${JSON.stringify(options)}`;
    return deduplicated(key, () => client.postMultipart(endpoint, options) as Promise<T>);
  }

  function postPublicApi<T = unknown>(endpoint: string, options?: unknown): Promise<T> {
    const key = `postPublicApi:${endpoint}:${JSON.stringify(options)}`;
    return deduplicated(key, () => client.postPublicApi(endpoint, options) as Promise<T>);
  }

  function put<T = unknown>(endpoint: string, options?: unknown): Promise<T> {
    const key = `put:${endpoint}:${JSON.stringify(options)}`;
    return deduplicated(key, () => client.put(endpoint, options) as Promise<T>);
  }

  function patch<T = unknown>(endpoint: string, options?: unknown): Promise<T> {
    const key = `patch:${endpoint}:${JSON.stringify(options)}`;
    return deduplicated(key, () => client.patch(endpoint, options) as Promise<T>);
  }

  function del<T = unknown>(endpoint: string, options?: RequestInit): Promise<T> {
    const key = `del:${endpoint}:${JSON.stringify(options)}`;
    return deduplicated(key, () => client.del(endpoint, options) as Promise<T>);
  }

  return (
    <HttpClientContext.Provider value={{ get, post, postMultipart, postPublicApi, put, patch, del }}>
      {children}
    </HttpClientContext.Provider>
  );
}
