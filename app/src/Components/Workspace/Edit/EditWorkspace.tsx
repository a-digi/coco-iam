import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import Title from '../../../Shared/Components/Font/Title';
import { PageHead, PageHeadBack, PageHeadMeta } from '../../../Shared/Components/PageHead';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { WorkspaceResource, type Workspace, WorkspaceSchema } from '../model/workspace';
import { OrganizationResource, OrganizationSchema, type Organization } from '../model/organization';
import { mapObjects } from '../../../config/data/mapper/mapper';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { WorkspaceForm, type WorkspaceFormData } from '../Form/WorkspaceForm';
import { WorkspaceDetailsPanel } from '../Details/WorkspaceDetailsPanel';
import { AppScopes } from '../../../config/security/scopes';
import { Tabs, type TabData } from '../../../Shared/Components/Tabs';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

export const EditWorkspace: React.FC = () => {
  useBreadcrumbItems([{ label: 'Workspaces', href: '/workspaces' }, { label: 'Manage' }]);
  const { id } = useParams<{ id: string }>();
  const [fetching, setFetching] = useState(true);
  const [loading, setLoading] = useState(false);
  const [initialData, setInitialData] = useState<Partial<WorkspaceFormData> | undefined>(undefined);
  const [organization, setOrganization] = useState<Organization | null>(null);

  const { get, patch } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const navigate = useNavigate();

  const fetchWorkspace = React.useCallback(async () => {
    if (!id) return;
    setFetching(true);
    try {
      const response = await get<{ message: unknown }>(`workspaces/{${WorkspaceResource}}/{id:${id}}`);
      const raw = response?.message || response;

      if (raw) {
        const mapped = mapObjects(WorkspaceSchema, [raw]) as unknown as Workspace[];
        const ws = mapped[0];
        setInitialData({
          workspace_id: ws.workspaceId || '',
          title: ws.title || '',
          description: ws.description || '',
          organization_id: ws.organizationId || '',
          is_active: ws.isActive ?? true,
        });

        if (ws.organizationId) {
          try {
            const orgResp = await get<{ message: unknown }>(
              `organizations/{${OrganizationResource}}/{id:${ws.organizationId}}`,
            );
            const orgRaw = orgResp?.message || orgResp;
            if (orgRaw) {
              const orgMapped = mapObjects(
                OrganizationSchema,
                [orgRaw] as Record<string, unknown>[],
              ) as unknown as Organization[];
              setOrganization(orgMapped[0] || null);
            }
          } catch {
            // non-fatal: fall back to id-only display
          }
        }
      } else {
        errorMessage('Workspace not found');
      }
    } catch (err: unknown) {
      let errorMsg = 'Failed to fetch workspace data';
      if (err instanceof Error) {
        errorMsg = err.message || errorMsg;
      }
      errorMessage(errorMsg);
    } finally {
      setFetching(false);
    }
  }, [id, get, errorMessage]);

  useEffect(() => {
    void fetchWorkspace();
  }, [fetchWorkspace]);

  const handleSubmit = async (data: WorkspaceFormData) => {
    if (!id) return;
    setLoading(true);
    try {
      // organization_id is immutable on edit — workspace_id, title,
      // description, is_active are sent.
      const payload: Record<string, unknown> = {
        workspace_id: data.workspace_id,
        title: data.title,
        description: data.description,
        is_active: data.is_active,
      };

      await patch(`workspaces/{${WorkspaceResource}}/{id:${id}}`, payload);
      successMessage(`Workspace ${data.title} updated successfully!`);
      navigate('/workspaces');
    } catch (err: unknown) {
      let errorMsg = 'Failed to update workspace';
      if (err instanceof Error) {
        errorMsg = err.message || errorMsg;
      }
      errorMessage(errorMsg);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  if (fetching) {
    return (
      <div className="max-w-full">
        <Title>Manage Workspace</Title>
        <div className="mt-6 flex items-center justify-center p-6">
          <svg className="animate-spin h-6 w-6 text-indigo-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"></path>
          </svg>
          <span className="ml-3 text-gray-500">Loading workspace data...</span>
        </div>
      </div>
    );
  }

  const orgId = initialData?.organization_id;
  const orgDisplay = orgId ? { id: orgId, title: organization?.title } : undefined;

  return (
    <div className="max-w-full">
      {initialData?.title && (
        <>
          <PageHeadBack to="/workspaces" label="Back to workspaces" />
          <PageHead
            kicker="Workspace"
            title={initialData.title}
            description={initialData.description}
            meta={
              orgId ? (
                <PageHeadMeta
                  label="Organization"
                  value={organization?.title || orgId}
                  to={`/organizations/edit/${orgId}`}
                />
              ) : undefined
            }
          />
        </>
      )}

      <Tabs
        items={(() => {
          const items: TabData[] = [
            {
              id: 'details',
              title: 'Details',
              content: id ? <WorkspaceDetailsPanel workspaceId={id} /> : undefined,
            }
          ];
          if (id) {
            items.push({
              id: 'applications',
              title: 'Manage Applications',
              href: `/workspaces/${id}/applications`,
              scopes: [AppScopes.ApplicationsRead, AppScopes.Applications, AppScopes.SuperAdmin],
            });
          }

          items.push(
            {
              id: 'edit',
              title: 'Edit',
              content: (
                <WorkspaceForm
                  initialData={initialData}
                  onSubmit={handleSubmit}
                  loading={loading}
                  submitLabel="Update Workspace"
                  hideOrganizationField
                  organizationDisplay={orgDisplay}
                />
              ),
            }
          );

          return items;
        })()}
        initialActiveId="details"
      />
    </div>
  );
};

export default EditWorkspace;
