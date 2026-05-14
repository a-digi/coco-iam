import { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import type { Agent, CreateAgentRequest, CreateAgentResponse, ProcessTarget } from '../model/observe';

interface UpdateAgentRequest {
  processes: ProcessTarget[];
  track_os?: boolean;
}

interface AgentsState {
  agents: Agent[];
  loading: boolean;
  error: string | null;
  reload: () => void;
  createAgent: (req: CreateAgentRequest) => Promise<CreateAgentResponse>;
  updateAgent: (id: string, req: UpdateAgentRequest) => Promise<void>;
  deleteAgent: (id: string) => Promise<void>;
}

export function useObserveAgents(): AgentsState {
  const { get, post, patch, del } = useHttpClient();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await get<Agent[]>('admin/observe/agents');
      setAgents(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agents');
    } finally {
      setLoading(false);
    }
  }, [get]);

  useEffect(() => { void reload(); }, [reload]);

  const createAgent = useCallback(async (req: CreateAgentRequest): Promise<CreateAgentResponse> => {
    const result = await post<CreateAgentRequest>('admin/observe/agents', req);
    await reload();
    return result as unknown as CreateAgentResponse;
  }, [post, reload]);

  const updateAgent = useCallback(async (id: string, req: UpdateAgentRequest): Promise<void> => {
    await patch(`admin/observe/agents/${id}`, req);
    await reload();
  }, [patch, reload]);

  const deleteAgent = useCallback(async (id: string): Promise<void> => {
    await del(`admin/observe/agents/${id}`);
    await reload();
  }, [del, reload]);

  return { agents, loading, error, reload, createAgent, updateAgent, deleteAgent };
}
