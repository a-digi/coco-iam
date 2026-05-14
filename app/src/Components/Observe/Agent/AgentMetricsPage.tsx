import React, { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';
import { PageHeadBack } from '../../../Shared/Components/PageHead/PageHeadBack';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../config/security/scopes';
import { MetricsSection } from '../Sections/MetricsSection';
import { ObserveCard } from '../Partials/ObserveCard';
import { useObserveAgents } from '../hooks/useObserveAgents';
import type { ProcessTarget } from '../model/observe';

// ─── edit modal ──────────────────────────────────────────────────────────────

interface EditModalProps {
  agentId: string;
  initialProcesses: ProcessTarget[];
  initialTrackOS: boolean;
  onClose: () => void;
  onSave: (processes: ProcessTarget[], trackOS: boolean) => Promise<void>;
}

const emptyProcess = (): ProcessTarget => ({ name: '' });

const EditModal: React.FC<EditModalProps> = ({
  agentId: _agentId,
  initialProcesses,
  initialTrackOS,
  onClose,
  onSave,
}) => {
  const [processes, setProcesses] = useState<ProcessTarget[]>(
    (initialProcesses ?? []).length ? initialProcesses : [],
  );
  const [trackOS, setTrackOS] = useState(initialTrackOS);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const addProcess = () => setProcesses(ps => [...ps, emptyProcess()]);
  const removeProcess = (i: number) => setProcesses(ps => ps.filter((_, idx) => idx !== i));
  const updateName = (i: number, value: string) =>
    setProcesses(ps => ps.map((p, idx) => idx === i ? { ...p, name: value } : p));

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await onSave(processes.filter(p => p.name.trim()), trackOS);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update agent');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white dark:bg-surface-800 rounded-2xl shadow-xl w-full max-w-lg p-6 max-h-[90vh] overflow-y-auto">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">Edit Monitoring Config</h2>
        <form onSubmit={handleSave} className="space-y-5">
          {/* Track OS toggle */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-700 dark:text-gray-300">Track host OS metrics</p>
              <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">CPU, RAM, disk, network</p>
            </div>
            <button
              type="button"
              onClick={() => setTrackOS(v => !v)}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                trackOS ? 'bg-indigo-600' : 'bg-gray-300 dark:bg-surface-600'
              }`}
            >
              <span
                className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                  trackOS ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
          </div>

          {/* Processes */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <div>
                <p className="text-sm font-medium text-gray-700 dark:text-gray-300">Processes to monitor</p>
                <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">Matched by process name on the host</p>
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
                      onChange={e => updateName(i, e.target.value)}
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

          <p className="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 rounded-lg px-3 py-2">
            After saving, download and restart the agent binary so it picks up the new config.
          </p>

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
              disabled={loading}
              className="flex-1 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-medium transition-colors"
            >
              {loading ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

// ─── page ─────────────────────────────────────────────────────────────────────

const AgentMetricsPage: React.FC = () => {
  const { agentId = '' } = useParams<{ agentId: string }>();
  const { agents, updateAgent } = useObserveAgents();
  const [showEdit, setShowEdit] = useState(false);

  const agent = agents.find(a => a.id === agentId);
  const agentName = agent?.name ?? agentId;

  useBreadcrumbItems([
    { label: 'Admin' },
    { label: 'Observe', href: '/admin/observe' },
    { label: agentName },
  ]);

  const handleSave = async (processes: ProcessTarget[], trackOS: boolean) => {
    await updateAgent(agentId, { processes, track_os: trackOS });
  };

  return (
    <div className="space-y-6 p-6">
      <PageHeadBack to="/admin/observe" label="Back to Agents" />

      <div className="mb-2 p-4 rounded-lg bg-gradient-to-r from-indigo-50 to-white dark:from-surface-800 dark:to-surface-900 border border-indigo-100 dark:border-surface-800">
        <div className="text-xs uppercase tracking-wide text-indigo-600 dark:text-indigo-400 mb-1">Agent</div>
        <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">{agentName}</h2>
        {agent && (
          <p className="text-xs text-gray-400 font-mono mt-1">{agent.api_key}</p>
        )}
      </div>

      {/* Monitoring config card */}
      {agent && (
        <ObserveCard
          title="Monitoring Config"
          actions={
            <button
              type="button"
              onClick={() => setShowEdit(true)}
              className="text-xs px-3 py-1 rounded-md bg-gray-100 dark:bg-surface-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-surface-600 transition-colors"
            >
              Edit
            </button>
          }
        >
          <div className="flex flex-wrap gap-6 text-sm">
            {/* Track OS */}
            <div>
              <p className="text-xs font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-wide mb-1.5">Host OS</p>
              <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${
                agent.track_os
                  ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400'
                  : 'bg-gray-100 dark:bg-surface-700 text-gray-500 dark:text-gray-400'
              }`}>
                <span className={`w-1.5 h-1.5 rounded-full ${agent.track_os ? 'bg-emerald-500' : 'bg-gray-400'}`} />
                {agent.track_os ? 'Tracked' : 'Disabled'}
              </span>
            </div>

            {/* Processes */}
            <div className="flex-1">
              <p className="text-xs font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-wide mb-1.5">Processes</p>
              {(agent.processes ?? []).length === 0 ? (
                <p className="text-xs text-amber-600 dark:text-amber-400 italic">
                  None configured — click Edit to add processes to monitor.
                </p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {(agent.processes ?? []).map(p => (
                    <span
                      key={p.name}
                      className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-mono font-medium bg-indigo-50 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-400 border border-indigo-100 dark:border-indigo-800"
                    >
                      {p.name}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        </ObserveCard>
      )}

      <ScopeBasedComponentAccess
        requiredScopes={[AppScopes.ObserveView, AppScopes.SuperAdmin]}
      >
        <MetricsSection agentId={agentId} />
      </ScopeBasedComponentAccess>

      {showEdit && agent && (
        <EditModal
          agentId={agentId}
          initialProcesses={agent.processes ?? []}
          initialTrackOS={agent.track_os}
          onClose={() => setShowEdit(false)}
          onSave={handleSave}
        />
      )}
    </div>
  );
};

export default AgentMetricsPage;
