import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { OrganizationGroupResource, type OrganizationGroup, OrganizationGroupSchema } from '../../model/organizationGroup';
import { OrganizationGroupForm, type OrganizationGroupFormData } from '../Form/OrganizationGroupForm';
import { OrganizationHeader } from '../../Shared/OrganizationHeader';
import { OrganizationScopes } from '../../Shared/OrganizationScopes';
import { GroupMembers } from './Partials/GroupMembers';
import { Tabs, type TabData } from '../../../../Shared/Components/Tabs';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const EditOrganizationGroup: React.FC = () => {
  useBreadcrumbItems([{ label: 'Organizations', href: '/organizations' }, { label: 'Groups' }, { label: 'Manage' }]);
  const { orgId, groupId } = useParams<{ orgId: string; groupId: string }>();
  const [fetching, setFetching] = useState(true);
  const [loading, setLoading] = useState(false);
  const [initialData, setInitialData] = useState<Partial<OrganizationGroupFormData> | undefined>(undefined);

  const { get, patch } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const navigate = useNavigate();

  const backTo = `/organizations/${orgId}/groups`;

  const fetchGroup = React.useCallback(async () => {
    if (!groupId) return;
    setFetching(true);
    try {
      const response = await get<{ message: unknown }>(`organizations/{${OrganizationGroupResource}}/{id:${groupId}}`);
      const raw = response?.message || response;
      if (raw) {
        const mapped = mapObjects(OrganizationGroupSchema, [raw]) as unknown as OrganizationGroup[];
        const g = mapped[0];
        setInitialData({
          title: g.title || '',
          group_description: g.groupDescription || '',
          is_active: g.isActive ?? true,
        });
      } else {
        errorMessage('Group not found');
      }
    } catch (err: unknown) {
      let errorMsg = 'Failed to fetch group data';
      if (err instanceof Error) errorMsg = err.message || errorMsg;
      errorMessage(errorMsg);
    } finally {
      setFetching(false);
    }
  }, [groupId, get, errorMessage]);

  useEffect(() => {
    void fetchGroup();
  }, [fetchGroup]);

  const handleSubmit = async (data: OrganizationGroupFormData) => {
    if (!groupId) return;
    setLoading(true);
    try {
      await patch(`organizations/{${OrganizationGroupResource}}/{id:${groupId}}`, {
        title: data.title,
        group_description: data.group_description,
        is_active: data.is_active,
      });
      successMessage(`Group ${data.title} updated successfully!`);
      navigate(backTo);
    } catch (err: unknown) {
      let errorMsg = 'Failed to update group';
      if (err instanceof Error) errorMsg = err.message || errorMsg;
      errorMessage(errorMsg);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  if (!orgId) return <div>Missing organization id.</div>;

  if (fetching) {
    return (
      <div className="max-w-full">
        <OrganizationHeader organizationId={orgId} />
        <Title>Manage Organization Group</Title>
        <div className="mt-6 text-sm text-gray-500">Loading...</div>
      </div>
    );
  }

  const tabItems: TabData[] = [
    {
      id: 'details',
      title: 'Details',
      content: (
        <OrganizationGroupForm
          initialData={initialData}
          onSubmit={handleSubmit}
          loading={loading}
          submitLabel="Update Group"
          cancelTo={backTo}
        />
      ),
    },
    {
      id: 'members',
      title: 'Members',
      content: groupId ? <GroupMembers organizationId={orgId} groupId={groupId} /> : null,
    },
    {
      id: 'scopes',
      title: 'Scopes',
      content: groupId ? (
        <OrganizationScopes
          entityId={groupId}
          resourceName="organization_group_acl"
          resourceKey="group_id"
        />
      ) : null,
    },
  ];

  return (
    <div className="max-w-full">
      <OrganizationHeader organizationId={orgId} />
      <Title>Manage Organization Group</Title>
      <div className="mt-6">
        <Tabs items={tabItems} initialActiveId="details" />
      </div>
    </div>
  );
};

export default EditOrganizationGroup;
