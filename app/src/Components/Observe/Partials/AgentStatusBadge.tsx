import React from 'react';

interface AgentStatusBadgeProps {
  lastSeenAt?: string;
  enabled: boolean;
}

function isOnline(lastSeenAt?: string): boolean {
  if (!lastSeenAt) return false;
  const diff = Date.now() - new Date(lastSeenAt).getTime();
  return diff < 5 * 60 * 1000; // within 5 minutes
}

export const AgentStatusBadge: React.FC<AgentStatusBadgeProps> = ({ lastSeenAt, enabled }) => {
  if (!enabled) {
    return (
      <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-500 dark:bg-surface-700 dark:text-gray-400">
        <span className="w-1.5 h-1.5 rounded-full bg-gray-400" />
        Disabled
      </span>
    );
  }
  const online = isOnline(lastSeenAt);
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${
        online
          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
          : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
      }`}
    >
      <span
        className={`w-1.5 h-1.5 rounded-full ${online ? 'bg-emerald-500 animate-pulse' : 'bg-amber-500'}`}
      />
      {online ? 'Online' : 'Offline'}
    </span>
  );
};
