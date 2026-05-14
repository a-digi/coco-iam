import React from 'react';
import { Link } from 'react-router-dom';
import type { OrgUserCount } from '../model/dashboard';

interface TopOrgsTableProps {
  orgs: OrgUserCount[];
}

export const TopOrgsTable: React.FC<TopOrgsTableProps> = ({ orgs }) => {
  if (orgs.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-gray-400 dark:text-gray-500 text-[0.875rem]">
        <span className="text-2xl mb-2">—</span>
        No organizations yet
      </div>
    );
  }

  const max = Math.max(...orgs.map(o => o.count), 1);

  return (
    <table className="w-full text-[0.8125rem]">
      <thead>
        <tr className="border-b border-gray-100 dark:border-surface-700">
          <th className="text-left py-2 pr-3 font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide text-[0.6875rem]">
            Organization
          </th>
          <th className="text-right py-2 pr-3 font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide text-[0.6875rem] whitespace-nowrap">
            Users
          </th>
          <th className="text-left py-2 font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide text-[0.6875rem] w-[40%]">
            Share
          </th>
        </tr>
      </thead>
      <tbody>
        {orgs.map(org => {
          const pct = (org.count / max) * 100;
          return (
            <tr
              key={org.id}
              className="border-b border-gray-50 dark:border-surface-700/50 hover:bg-gray-50 dark:hover:bg-surface-700/30 transition-colors"
            >
              <td className="py-2.5 pr-3">
                <Link
                  to={`/organizations/${org.id}/users`}
                  className="text-indigo-600 dark:text-indigo-400 hover:underline font-medium truncate block"
                >
                  {org.name}
                </Link>
              </td>
              <td className="py-2.5 pr-3 text-right font-semibold text-gray-800 dark:text-gray-100 whitespace-nowrap">
                {org.count}
              </td>
              <td className="py-2.5">
                <div className="h-2 w-full rounded-full bg-gray-200 dark:bg-surface-700 overflow-hidden">
                  <div
                    className="h-full bg-teal-500 rounded-full"
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
};

export default TopOrgsTable;
