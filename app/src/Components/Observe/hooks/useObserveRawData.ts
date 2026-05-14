import { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import type { Batch } from '../model/observe';
import type { MetricRange } from './useObserveMetrics';

export interface RawPage {
  total: number;
  page: number;
  limit: number;
  items: Batch[];
}

interface RawDataState {
  data: RawPage | null;
  loading: boolean;
  error: string | null;
}

const DEFAULT_LIMIT = 50;

export function useObserveRawData(
  agentId: string,
  range: MetricRange,
  page: number,
  enabled = false,
): RawDataState {
  const { get } = useHttpClient();
  const [data, setData] = useState<RawPage | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetch = useCallback(async () => {
    if (!agentId || !enabled) return;
    setLoading(true);
    setError(null);
    try {
      const result = await get<RawPage>(
        `admin/observe/metrics/raw?agent_id=${encodeURIComponent(agentId)}&range=${range}&page=${page}&limit=${DEFAULT_LIMIT}`
      );
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load raw data');
    } finally {
      setLoading(false);
    }
  }, [get, agentId, range, page, enabled]);

  useEffect(() => { void fetch(); }, [fetch]);

  return { data, loading, error };
}
