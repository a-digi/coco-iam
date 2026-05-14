import React, { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { OrganizationGroupResource } from '../../model/organizationGroup';
import { OrganizationGroupForm, type OrganizationGroupFormData } from '../Form/OrganizationGroupForm';
import { OrganizationHeader } from '../../Shared/OrganizationHeader';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const CreateOrganizationGroup: React.FC = () => {
  useBreadcrumbItems([{ label: 'Organizations', href: '/organizations' }, { label: 'Groups' }, { label: 'Create' }]);
  const { orgId } = useParams<{ orgId: string }>();
  const [loading, setLoading] = useState(false);
  const { post } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const navigate = useNavigate();

  const backTo = `/organizations/${orgId}/groups`;

  const handleSubmit = async (data: OrganizationGroupFormData) => {
    if (!orgId) return;
    setLoading(true);
    try {
      const response = await post<{ message?: { id: string }, id?: string }>(
        `organizations/{${OrganizationGroupResource}}`,
        {
          title: data.title,
          group_description: data.group_description,
          organization_id: orgId,
          is_active: data.is_active,
        }
      );
      const rawData = response?.message || response;
      const newId = rawData?.id;
      successMessage(`Group ${data.title} created successfully!`);

      if (newId) {
        navigate(`/organizations/${orgId}/groups/edit/${newId}`);
      } else {
        navigate(backTo);
      }
    } catch (err: unknown) {
      let errorMsg = 'Failed to create group';
      if (err instanceof Error) errorMsg = err.message || errorMsg;
      errorMessage(errorMsg);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  if (!orgId) return <div>Missing organization id.</div>;

  return (
    <div className="max-w-full">
      <OrganizationHeader organizationId={orgId} />
      <Title>Create Organization Group</Title>
      <OrganizationGroupForm
        onSubmit={handleSubmit}
        loading={loading}
        submitLabel="Create Group"
        cancelTo={backTo}
      />
    </div>
  );
};

export default CreateOrganizationGroup;
