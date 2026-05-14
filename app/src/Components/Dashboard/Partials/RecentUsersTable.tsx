import React from 'react';
import { Link } from 'react-router-dom';
import { formatDateOnly } from '../../../config/data/date/date';
import type { RecentUser } from '../model/dashboard';

interface RecentUsersTableProps {
  users: RecentUser[];
}

export const RecentUsersTable: React.FC<RecentUsersTableProps> = ({ users }) => {
  if (users.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-gray-400 dark:text-gray-500 text-[0.875rem]">
        <span className="text-2xl mb-2">—</span>
        No users yet
      </div>
    );
  }

  return (
    <table className="w-full text-[0.8125rem]">
      <thead>
        <tr className="border-b border-gray-100 dark:border-surface-700">
          <th className="text-left py-2 pr-4 font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide text-[0.6875rem]">
            Username
          </th>
          <th className="text-left py-2 font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide text-[0.6875rem]">
            Created
          </th>
        </tr>
      </thead>
      <tbody>
        {users.map(user => (
          <tr
            key={user.id}
            className="border-b border-gray-50 dark:border-surface-700/50 hover:bg-gray-50 dark:hover:bg-surface-700/30 transition-colors"
          >
            <td className="py-2.5 pr-4">
              <Link
                to={`/admin/users/edit/${user.id}`}
                className="text-indigo-600 dark:text-indigo-400 hover:underline font-medium"
              >
                {user.username}
              </Link>
            </td>
            <td className="py-2.5 text-gray-500 dark:text-gray-400">
              {formatDateOnly(user.created_at)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default RecentUsersTable;
