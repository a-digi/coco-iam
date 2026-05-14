import React, { useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import { ObserveCard } from '../Partials/ObserveCard';
import { AgentStatusBadge } from '../Partials/AgentStatusBadge';
import { useObserveAgents } from '../hooks/useObserveAgents';
import { WidgetSkeleton, WidgetError } from '../../Dashboard/Partials/WidgetState';
import type { Agent, CreateAgentRequest, CreateAgentResponse, ProcessTarget } from '../model/observe';
import { buildHeaders, API_BASE_URL } from '../../../api/client';

interface AgentRowProps {
  agent: Agent;
  onDelete: (id: string) => void;
}

const ARCHES = ['amd64', 'arm64'] as const;
type Arch = typeof ARCHES[number];

const AgentRow: React.FC<AgentRowProps> = ({ agent, onDelete }) => {
  const navigate = useNavigate();
  const [downloading, setDownloading] = useState<Arch | null>(null);
  const [menuPos, setMenuPos] = useState<{ top: number; right: number } | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

  const handleToggleMenu = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (menuPos) { setMenuPos(null); return; }
    const rect = btnRef.current?.getBoundingClientRect();
    if (!rect) return;
    setMenuPos({ top: rect.bottom + 4, right: window.innerWidth - rect.right });
  };

  const handleDownload = async (arch: Arch) => {
    setMenuPos(null);
    setDownloading(arch);
    try {
      const headers = await buildHeaders();
      const res = await fetch(
        `${API_BASE_URL}admin/observe/agents/${agent.id}/download?arch=${arch}`,
        { headers: headers as HeadersInit },
      );
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || res.statusText);
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `observe-agent-${arch}-${agent.name}`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Download failed');
    } finally {
      setDownloading(null);
    }
  };

  return (
    <tr
      className="border-b border-gray-100 dark:border-surface-700 cursor-pointer hover:bg-gray-50 dark:hover:bg-surface-700/40 transition-colors"
      onClick={() => navigate(`/admin/observe/agents/${agent.id}`)}
    >
      <td className="py-3 px-4">
        <div className="font-medium text-gray-900 dark:text-gray-100 text-sm">{agent.name}</div>
        <div className="text-xs text-gray-400 font-mono mt-0.5">{agent.api_key}</div>
      </td>
      <td className="py-3 px-4">
        <AgentStatusBadge lastSeenAt={agent.last_seen_at} enabled={agent.enabled} />
      </td>
      <td className="py-3 px-4 text-xs text-gray-500 dark:text-gray-400">
        {agent.last_seen_at
          ? new Date(agent.last_seen_at).toLocaleString()
          : <span className="italic">Never</span>}
      </td>
      <td className="py-3 px-4 text-xs text-gray-400">
        {new Date(agent.registered_at).toLocaleDateString()}
      </td>
      <td className="py-3 px-4 text-right">
        <div className="inline-flex items-center gap-3">
          <button
            ref={btnRef}
            type="button"
            onClick={handleToggleMenu}
            disabled={downloading !== null}
            className="text-xs text-indigo-500 hover:text-indigo-700 dark:text-indigo-400 dark:hover:text-indigo-300 transition-colors disabled:opacity-50"
          >
            {downloading ? `Downloading ${downloading}…` : 'Download ▾'}
          </button>
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); onDelete(agent.id); }}
            className="text-xs text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 transition-colors"
          >
            Delete
          </button>
        </div>
        {menuPos && createPortal(
          <>
            {/* backdrop: catches outside clicks without interfering with the menu buttons */}
            <div
              style={{ position: 'fixed', inset: 0, zIndex: 9998 }}
              onClick={() => setMenuPos(null)}
            />
            <div
              style={{ position: 'fixed', top: menuPos.top, right: menuPos.right, zIndex: 9999 }}
              className="bg-white dark:bg-surface-800 border border-gray-200 dark:border-surface-600 rounded-lg shadow-lg py-1 min-w-[120px]"
            >
              {ARCHES.map(arch => (
                <button
                  key={arch}
                  type="button"
                  onClick={() => void handleDownload(arch)}
                  className="block w-full text-left px-3 py-1.5 text-xs text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-surface-700 transition-colors"
                >
                  Linux {arch}
                </button>
              ))}
            </div>
          </>,
          document.body,
        )}
      </td>
    </tr>
  );
};

interface CreateModalProps {
  onClose: () => void;
  onCreate: (req: CreateAgentRequest) => Promise<CreateAgentResponse>;
}

const emptyProcess = (): ProcessTarget => ({ name: '' });

const CreateModal: React.FC<CreateModalProps> = ({ onClose, onCreate }) => {
  const [name, setName] = useState('');
  const [trackOS, setTrackOS] = useState(true);
  const [processes, setProcesses] = useState<ProcessTarget[]>([]);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<CreateAgentResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const addProcess = () => setProcesses(ps => [...ps, emptyProcess()]);
  const removeProcess = (i: number) => setProcesses(ps => ps.filter((_, idx) => idx !== i));
  const updateProcess = (i: number, value: string) =>
    setProcesses(ps => ps.map((p, idx) => idx === i ? { ...p, name: value } : p));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const validProcesses = processes.filter(p => p.name.trim());
      const res = await onCreate({
        name: name.trim(),
        processes: validProcesses,
        track_os: trackOS,
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create agent');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white dark:bg-surface-800 rounded-2xl shadow-xl w-full max-w-lg p-6 max-h-[90vh] overflow-y-auto">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">Register Agent</h2>

        {result ? (
          <div className="space-y-4">
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Agent created. Copy these credentials now — the secret will not be shown again.
            </p>
            <div className="space-y-2 bg-gray-50 dark:bg-surface-700 rounded-xl p-4 text-sm font-mono">
              <div><span className="text-gray-500">API Key: </span><span className="text-gray-900 dark:text-gray-100">{result.api_key}</span></div>
              <div><span className="text-gray-500">Secret: </span><span className="text-emerald-600 dark:text-emerald-400">{result.api_secret}</span></div>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="w-full py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium transition-colors"
            >
              Done
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
            {/* Name */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Agent name
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. production-server-01"
                className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 text-sm text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                autoFocus
              />
            </div>

            {/* Track OS toggle */}
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-700 dark:text-gray-300">Track host OS metrics</p>
                <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">CPU, RAM, disk, network</p>
              </div>
              <button
                type="button"
                onClick={() => setTrackOS(v => !v)}
                className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${trackOS ? 'bg-indigo-600' : 'bg-gray-300 dark:bg-surface-600'
                  }`}
              >
                <span
                  className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${trackOS ? 'translate-x-6' : 'translate-x-1'
                    }`}
                />
              </button>
            </div>

            {/* Go process targets */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <div>
                  <p className="text-sm font-medium text-gray-700 dark:text-gray-300">Go processes to monitor</p>
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">Scrapes /debug/vars for heap, GC, goroutines</p>
                </div>
                <button
                  type="button"
                  onClick={addProcess}
                  className="text-xs px-2 py-1 rounded-md bg-indigo-50 dark:bg-indigo-900/30 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-100 dark:hover:bg-indigo-900/50 transition-colors"
                >
                  + Add
                </button>
              </div>

              {processes.length === 0 ? (
                <p className="text-xs text-gray-400 dark:text-gray-500 italic">
                  No processes — only OS metrics will be collected.
                </p>
              ) : (
                <div className="space-y-2">
                  {processes.map((p, i) => (
                    <div key={i} className="flex gap-2 items-center">
                      <input
                        type="text"
                        value={p.name}
                        onChange={e => updateProcess(i, e.target.value)}
                        placeholder="Process name (e.g. coco-iam-api)"
                        className="flex-1 px-2.5 py-1.5 rounded-md border border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 text-xs text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                      />
                      <button
                        type="button"
                        onClick={() => removeProcess(i)}
                        className="text-gray-400 hover:text-red-500 dark:hover:text-red-400 transition-colors text-lg leading-none"
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {error && <p className="text-sm text-red-500">{error}</p>}
            <div className="flex gap-3">
              <button
                type="button"
                onClick={onClose}
                className="flex-1 py-2 rounded-lg border border-gray-300 dark:border-surface-600 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-surface-700 transition-colors"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={loading || !name.trim()}
                className="flex-1 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-medium transition-colors"
              >
                {loading ? 'Creating…' : 'Create'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};

export const AgentsSection: React.FC = () => {
  const { agents, loading, error, reload, createAgent, deleteAgent } = useObserveAgents();
  const [showCreate, setShowCreate] = useState(false);

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this agent and all its data?')) return;
    try {
      await deleteAgent(id);
    } catch {
      // error shown inline on next load
    }
  };

  return (
    <>
      <ObserveCard
        title="Agents"
        actions={
          <button
            type="button"
            onClick={() => setShowCreate(true)}
            className="px-3 py-1.5 text-xs font-medium rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white transition-colors"
          >
            + Register
          </button>
        }
      >
        {loading && <WidgetSkeleton className="h-[120px]" />}
        {error && <WidgetError message={error} onRetry={reload} />}
        {!loading && !error && (
          agents.length === 0 ? (
            <p className="text-sm text-gray-400 dark:text-gray-500 py-6 text-center italic">
              No agents registered yet.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs uppercase tracking-wide text-gray-400 dark:text-gray-500 border-b border-gray-100 dark:border-surface-700">
                    <th className="pb-2 px-4">Agent</th>
                    <th className="pb-2 px-4">Status</th>
                    <th className="pb-2 px-4">Last seen</th>
                    <th className="pb-2 px-4">Registered</th>
                    <th className="pb-2 px-4" />
                  </tr>
                </thead>
                <tbody>
                  {agents.map((agent: Agent) => (
                    <AgentRow
                      key={agent.id}
                      agent={agent}
                      onDelete={handleDelete}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )
        )}
      </ObserveCard>

      {showCreate && (
        <CreateModal
          onClose={() => setShowCreate(false)}
          onCreate={createAgent}
        />
      )}
    </>
  );
};
