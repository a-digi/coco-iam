import { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';

interface WidgetState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

export function useWidgetData<T>(path: string): WidgetState<T> {
  const { get } = useHttpClient();
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await get<{ message: T }>(path);
      const payload = response?.message ?? (response as unknown as T);
      setData(payload);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [get, path]);

  useEffect(() => {
    void reload();
  }, [reload]);

  return { data, loading, error, reload };
}
