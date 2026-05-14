import React, { useCallback, useEffect, useState } from 'react';
import { publicMediaURL, slugMediaURL } from '../../../../../api/client';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useApplicationSlugs } from '../../../hooks/useApplicationSlugs';
import { mapObjects } from '../../../../../config/data/mapper/mapper';
import { formatDateOnly } from '../../../../../config/data/date/date';
import ScopeBasedComponentAccess from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../../config/security/scopes';
import { ApplicationResource, ApplicationSchema, type Application } from '../../../model/application';
import { UsersCount } from './Widgets/UsersCount';
import { GroupsCount } from './Widgets/GroupsCount';
import { ScopesCount } from './Widgets/ScopesCount';
import { PendingRecoveries } from './Widgets/PendingRecoveries';
import { RecentGrants } from './Widgets/RecentGrants';

interface Props {
  applicationId: string;
}

interface MediaFile {
  id: string;
  filename: string;
  folder_id: string | null;
}

interface MediaListing {
  message?: { folders: unknown[]; files: MediaFile[] };
}

/**
 * DetailPanel is the first side-menu entry on EditApplication. It
 * shows read-only application details + analytics widgets. Each
 * widget is wrapped in ScopeBasedComponentAccess and hidden when the
 * caller lacks the scope. If the caller has no analytics scopes at
 * all, the "Detail" sidebar entry itself is hidden (handled upstream
 * in EditApplication), and the panel never mounts.
 */
export const DetailPanel: React.FC<Props> = ({ applicationId }) => {
  const { get } = useHttpClient();
  const [app, setApp] = useState<Application | null>(null);
  const [logoFilename, setLogoFilename] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [appResp, mediaResp] = await Promise.all([
        get<{ message: unknown }>(
          `applications/{${ApplicationResource}}/{id:${applicationId}}`
        ),
        get<MediaListing>(
          `applications/{${ApplicationResource}}/{id:${applicationId}}/media`
        ),
      ]);
      const raw = appResp?.message ?? appResp;
      if (raw) {
        const mapped = mapObjects(ApplicationSchema, [raw]) as unknown as Application[];
        setApp(mapped[0] ?? null);
      }
      const files = mediaResp?.message?.files ?? [];
      const logo = files.find(
        f => f.folder_id === null && /^logo\.[a-zA-Z0-9]+$/.test(f.filename)
      );
      setLogoFilename(logo?.filename ?? null);
    } catch {
      // Silent; loading state resolves and the read-only section just shows blanks.
    } finally {
      setLoading(false);
    }
  }, [get, applicationId]);

  useEffect(() => {
    void load();
  }, [load]);

  const slugs = useApplicationSlugs(applicationId);

  if (loading || !app) {
    return <div className="text-sm text-gray-500 p-4">Loading detail…</div>;
  }

  const logoURL = logoFilename
    ? slugs
      ? slugMediaURL(slugs.organization_id, slugs.workspace_id, slugs.client_id, logoFilename)
      : publicMediaURL(applicationId, logoFilename)
    : null;

  return (
    <div className="space-y-6 mt-2">
      {/* Read-only summary */}
      <section className="bg-white dark:bg-surface-800 border border-gray-200 dark:border-surface-700 rounded-xl p-5 shadow-sm">
        <div className="flex items-start gap-4">
          {logoURL ? (
            <img
              src={logoURL}
              alt=""
              className="h-16 w-16 object-contain rounded-lg bg-white border border-gray-200 dark:border-surface-700 shrink-0"
            />
          ) : (
            <div className="h-16 w-16 rounded-lg bg-gray-50 dark:bg-surface-900 border border-dashed border-gray-300 dark:border-surface-600 flex items-center justify-center text-xs text-gray-400 shrink-0">
              no logo
            </div>
          )}
          <div className="flex-1 min-w-0">
            <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100 truncate">{app.title}</h2>
            <div className="mt-1 text-[0.8125rem] font-mono text-gray-500 truncate">{app.clientId}</div>
            {app.description && (
              <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">{app.description}</p>
            )}
            <div className="mt-3 flex flex-wrap gap-2">
              <Badge
                label={app.isActive ? 'Active' : 'Inactive'}
                tone={app.isActive ? 'emerald' : 'gray'}
              />
              <Badge
                label={app.allowRecovery ? 'Recovery allowed' : 'Recovery disabled'}
                tone={app.allowRecovery ? 'emerald' : 'amber'}
              />
              <Badge
                label={app.allowRegistration ? 'Registration allowed' : 'Registration disabled'}
                tone={app.allowRegistration ? 'emerald' : 'gray'}
              />
              <Badge label={`Created ${formatDateOnly(app.createdAt)}`} tone="indigo" />
            </div>
          </div>
        </div>
      </section>

      {/* Analytics widgets — each gated by its own scope + umbrella read. */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <ScopeBasedComponentAccess
          requiredScopes={[
            AppScopes.ApplicationsAnalyticsUsersRead,
            AppScopes.ApplicationsAnalyticsRead,
            AppScopes.SuperAdmin,
          ]}
        >
          <UsersCount applicationId={applicationId} />
        </ScopeBasedComponentAccess>

        <ScopeBasedComponentAccess
          requiredScopes={[
            AppScopes.ApplicationsAnalyticsGroupsRead,
            AppScopes.ApplicationsAnalyticsRead,
            AppScopes.SuperAdmin,
          ]}
        >
          <GroupsCount applicationId={applicationId} />
        </ScopeBasedComponentAccess>

        <ScopeBasedComponentAccess
          requiredScopes={[
            AppScopes.ApplicationsAnalyticsScopesRead,
            AppScopes.ApplicationsAnalyticsRead,
            AppScopes.SuperAdmin,
          ]}
        >
          <ScopesCount applicationId={applicationId} />
        </ScopeBasedComponentAccess>

        <ScopeBasedComponentAccess
          requiredScopes={[
            AppScopes.ApplicationsAnalyticsPendingRecoveriesRead,
            AppScopes.ApplicationsAnalyticsRead,
            AppScopes.SuperAdmin,
          ]}
        >
          <PendingRecoveries applicationId={applicationId} />
        </ScopeBasedComponentAccess>
      </div>

      <ScopeBasedComponentAccess
        requiredScopes={[
          AppScopes.ApplicationsAnalyticsRecentGrantsRead,
          AppScopes.ApplicationsAnalyticsRead,
          AppScopes.SuperAdmin,
        ]}
      >
        <RecentGrants applicationId={applicationId} />
      </ScopeBasedComponentAccess>
    </div>
  );
};

const Badge: React.FC<{ label: string; tone: 'emerald' | 'amber' | 'gray' | 'indigo' }> = ({
  label,
  tone,
}) => {
  const toneClasses = {
    emerald:
      'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
    amber:
      'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
    gray: 'bg-gray-100 text-gray-600 dark:bg-surface-700 dark:text-gray-400',
    indigo:
      'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300',
  }[tone];
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-[0.6875rem] font-semibold ${toneClasses}`}
    >
      {label}
    </span>
  );
};

export default DetailPanel;
