import { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { ApplicationResource } from '../../../model/application';

interface WidgetState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

/**
 * Tiny fetch-once hook used by each Detail widget. Handles loading +
 * error state so widget components stay focused on rendering.
 * `suffix` is the part of the URL after `/{id}/analytics/` — e.g.
 * 'users-count'.
 */
export function useWidget<T>(applicationId: string, suffix: string): WidgetState<T> {
  const { get } = useHttpClient();
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await get<{ message: T }>(
        `applications/{${ApplicationResource}}/{id:${applicationId}}/analytics/${suffix}`
      );
      const payload = response?.message ?? (response as unknown as T);
      setData(payload);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [get, applicationId, suffix]);

  useEffect(() => {
    void reload();
  }, [reload]);

  return { data, loading, error, reload };
}
