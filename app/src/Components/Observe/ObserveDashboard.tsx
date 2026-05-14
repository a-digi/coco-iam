import React from 'react';
import { useBreadcrumbItems } from '../../Layout/Breadcrumb/useBreadcrumb';
import ScopeBasedComponentAccess from '../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../config/security/scopes';
import { AgentsSection } from './Sections/AgentsSection';

const ObserveDashboard: React.FC = () => {
  useBreadcrumbItems([{ label: 'Admin' }, { label: 'Observe' }]);

  return (
    <div className="space-y-6 p-6">
      <div className="mb-6 p-4 rounded-lg bg-gradient-to-r from-indigo-50 to-white dark:from-surface-800 dark:to-surface-900 border border-indigo-100 dark:border-surface-800">
        <div className="text-xs uppercase tracking-wide text-indigo-600 dark:text-indigo-400 mb-1">System</div>
        <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Observability</h2>
        <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">
          Monitor registered agents and their runtime metrics. Click an agent to view its dashboard.
        </p>
      </div>

      <ScopeBasedComponentAccess
        requiredScopes={[AppScopes.ObserveManage, AppScopes.SuperAdmin]}
      >
        <AgentsSection />
      </ScopeBasedComponentAccess>
    </div>
  );
};

export default ObserveDashboard;
