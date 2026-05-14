import React from 'react';
import { Link } from 'react-router-dom';
import { formatDateOnly } from '../../../../../../config/data/date/date';
import { useWidget } from '../useWidget';
import { WidgetFrame } from '../WidgetFrame';

interface Grant {
  user_id: string;
  username: string;
  email: string;
  created_at: string;
}

export const RecentGrants: React.FC<{ applicationId: string }> = ({ applicationId }) => {
  const { data, loading, error, reload } = useWidget<Grant[]>(applicationId, 'recent-grants');
  const grants = Array.isArray(data) ? data : [];
  return (
    <WidgetFrame title="Recent Grants" loading={loading} error={error} onRetry={reload}>
      {grants.length === 0 ? (
        <div className="text-sm text-gray-500">No users granted access yet.</div>
      ) : (
        <table className="w-full text-[0.8125rem]">
          <tbody>
            {grants.map(g => (
              <tr
                key={g.user_id}
                className="border-b border-gray-50 dark:border-surface-700/50 last:border-0"
              >
                <td className="py-1.5 pr-3">
                  <Link
                    to={`/admin/users/edit/${g.user_id}`}
                    className="text-indigo-600 dark:text-indigo-400 hover:underline font-medium"
                  >
                    {g.username}
                  </Link>
                  <span className="block text-[0.6875rem] text-gray-500">{g.email}</span>
                </td>
                <td className="py-1.5 text-[0.6875rem] text-gray-500 whitespace-nowrap text-right">
                  {formatDateOnly(g.created_at)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </WidgetFrame>
  );
};

export default RecentGrants;
