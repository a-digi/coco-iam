import React from 'react';
import { Link } from 'react-router-dom';
import { EditAction, DeleteAction } from '../../../../../Shared/Components/Actions';
import ScopeBasedComponentAccess from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { useColorPair } from '../../../../../Shared/Components/ColorPalette';
import { AppScopes } from '../../../../../config/security/scopes';
import { formatDateOnly } from '../../../../../config/data/date/date';
import type { OrganizationUser } from '../../../model/organizationUser';

interface OrganizationUserCardProps {
  user: OrganizationUser;
  organizationId: string;
  onDelete: (id: string, username: string) => void;
}

const initials = (value: string): string => {
  const trimmed = value.trim();
  if (!trimmed) return '?';
  const parts = trimmed.split(/[\s._-]+/).filter(Boolean);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
};

export const OrganizationUserCard: React.FC<OrganizationUserCardProps> = ({
  user,
  organizationId,
  onDelete,
}) => {
  const { bg, text } = useColorPair(user.id);
  const avatar = initials(user.username || user.email);
  const editHref = `/organizations/${organizationId}/users/edit/${user.id}`;

  return (
    <div className="bg-white dark:bg-surface-800 rounded-2xl border border-gray-200 dark:border-surface-700 shadow-sm hover:shadow-md transition-shadow overflow-hidden">
      {/* Avatar header */}
      <div className={`${bg} ${text} h-28 flex items-center justify-center relative`}>
        <div className="w-16 h-16 rounded-full bg-white/30 backdrop-blur flex items-center justify-center text-xl font-bold tracking-wide shadow-lg">
          {avatar}
        </div>
        {!user.isActive && (
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
          title={user.username}
        >
          {user.username}
        </Link>

        {user.email && (
          <div className="flex items-center gap-1 text-[0.75rem] text-gray-600 dark:text-gray-400 truncate">
            <svg className="h-3.5 w-3.5 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8} aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75" />
            </svg>
            <span className="truncate" title={user.email}>{user.email}</span>
          </div>
        )}

        <div className="flex items-center justify-between pt-2 border-t border-gray-100 dark:border-surface-700">
          <span className="text-[0.6875rem] text-gray-500 dark:text-gray-400">
            Created {formatDateOnly(user.createdAt)}
          </span>
          <div className="flex items-center gap-1">
            <EditAction to={editHref} />
            <ScopeBasedComponentAccess
              requiredScopes={[
                AppScopes.OrganizationsUsersDelete,
                AppScopes.OrganizationsUsers,
                AppScopes.Organizations,
                AppScopes.SuperAdmin,
              ]}
            >
              <DeleteAction onClick={() => onDelete(user.id, user.username)} />
            </ScopeBasedComponentAccess>
          </div>
        </div>
      </div>
    </div>
  );
};

export default OrganizationUserCard;
