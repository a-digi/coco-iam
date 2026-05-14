import React from 'react';
import { Link } from 'react-router-dom';
import { EditAction, DeleteAction } from '../../../../Shared/Components/Actions';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { useColorPair } from '../../../../Shared/Components/ColorPalette';
import { AppScopes } from '../../../../config/security/scopes';
import { formatDateOnly } from '../../../../config/data/date/date';
import type { Application } from '../../model/application';

interface ApplicationCardProps {
  application: Application;
  workspaceId: string;
  onDelete: (id: string, title: string) => void;
}

const initials = (title: string): string => {
  const trimmed = title.trim();
  if (!trimmed) return '?';
  const parts = trimmed.split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
};

export const ApplicationCard: React.FC<ApplicationCardProps> = ({ application, workspaceId, onDelete }) => {
  const { bg, text } = useColorPair(application.id);
  const avatar = initials(application.title);
  const editHref = `/workspaces/${workspaceId}/applications/edit/${application.id}`;

  return (
    <div className="bg-white dark:bg-surface-800 rounded-2xl border border-gray-200 dark:border-surface-700 shadow-sm hover:shadow-md transition-shadow overflow-hidden">
      {/* Logo header */}
      <div className={`${bg} ${text} h-28 flex items-center justify-center relative`}>
        <div className="w-16 h-16 rounded-2xl bg-white/30 backdrop-blur flex items-center justify-center text-xl font-bold tracking-wide shadow-lg">
          {avatar}
        </div>
        {!application.isActive && (
          <span className="absolute top-3 right-3 px-2 py-0.5 text-[0.625rem] font-semibold uppercase tracking-widest rounded-full bg-black/40 text-white">
            Inactive
          </span>
        )}
      </div>

      {/* Body */}
      <div className="p-4 space-y-2">
        <Link
          to={editHref}
          className="block text-[0.9375rem] font-semibold text-gray-900 dark:text-gray-100 hover:text-indigo-600 dark:hover:text-indigo-400 truncate"
          title={application.title}
        >
          {application.title}
        </Link>

        <div className="flex items-center gap-1 text-[0.6875rem] text-gray-500 dark:text-gray-400 font-mono truncate">
          <span className="uppercase tracking-widest font-sans font-semibold">ID</span>
          <span className="truncate" title={application.clientId}>{application.clientId}</span>
        </div>

        {application.description && (
          <p className="text-[0.8125rem] text-gray-600 dark:text-gray-400 line-clamp-3">
            {application.description}
          </p>
        )}

        <div className="flex items-center justify-between pt-2 border-t border-gray-100 dark:border-surface-700">
          <span className="text-[0.6875rem] text-gray-500 dark:text-gray-400">
            Created {formatDateOnly(application.createdAt)}
          </span>
          <div className="flex items-center gap-1">
            <EditAction to={editHref} />
            <ScopeBasedComponentAccess
              requiredScopes={[AppScopes.ApplicationsDelete, AppScopes.Applications, AppScopes.SuperAdmin]}
            >
              <DeleteAction onClick={() => onDelete(application.id, application.title)} />
            </ScopeBasedComponentAccess>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ApplicationCard;
