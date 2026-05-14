import { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import type { Batch } from '../model/observe';

export type MetricRange = '1h' | '6h' | '24h' | '7d';

interface MetricsState {
  batches: Batch[];
  loading: boolean;
  error: string | null;
  reload: () => void;
}

export function useObserveMetrics(
  agentId: string,
  range: MetricRange,
  enabled = true,
): MetricsState {
  const { get } = useHttpClient();
  const [batches, setBatches] = useState<Batch[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    if (!agentId || !enabled) return;
    setLoading(true);
    setError(null);
    try {
      const data = await get<Batch[]>(
        `admin/observe/metrics?agent_id=${encodeURIComponent(agentId)}&range=${range}`
      );
      setBatches(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load metrics');
    } finally {
      setLoading(false);
    }
  }, [get, agentId, range, enabled]);

  useEffect(() => { void reload(); }, [reload]);

  return { batches, loading, error, reload };
}
