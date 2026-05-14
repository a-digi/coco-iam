import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { OrganizationUserResource, type OrganizationUser, OrganizationUserSchema } from '../../model/organizationUser';
import { OrganizationUserForm, type OrganizationUserFormData } from '../Form/OrganizationUserForm';
import { ResendActivationButton } from './Partials/ResendActivationButton';
import { OrganizationHeader } from '../../Shared/OrganizationHeader';
import { SideMenu, type SideMenuItem } from '../../../../Shared/Components/SideMenu';
import { AppScopes } from '../../../../config/security/scopes';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const EditOrganizationUser: React.FC = () => {
  useBreadcrumbItems([{ label: 'Organizations', href: '/organizations' }, { label: 'Users' }, { label: 'Manage' }]);
  const { orgId, userId } = useParams<{ orgId: string; userId: string }>();
  const [fetching, setFetching] = useState(true);
  const [loading, setLoading] = useState(false);
  const [initialData, setInitialData] = useState<Partial<OrganizationUserFormData> | undefined>(undefined);
  const [activationPending, setActivationPending] = useState(false);

  const { get, patch } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const navigate = useNavigate();

  const backTo = `/organizations/${orgId}/users`;

  const fetchUser = React.useCallback(async () => {
    if (!userId) return;
    setFetching(true);
    try {
      const response = await get<{ message: unknown }>(`organizations/{${OrganizationUserResource}}/{id:${userId}}`);
      const raw = response?.message || response;
      if (raw) {
        const mapped = mapObjects(OrganizationUserSchema, [raw]) as unknown as OrganizationUser[];
        const u = mapped[0];
        setInitialData({
          username: u.username || '',
          email: u.email || '',
          is_active: u.isActive ?? true,
        });
        setActivationPending(u.activationPending ?? false);
      } else {
        errorMessage('User not found');
      }
    } catch (err: unknown) {
      let errorMsg = 'Failed to fetch user data';
      if (err instanceof Error) errorMsg = err.message || errorMsg;
      errorMessage(errorMsg);
    } finally {
      setFetching(false);
    }
  }, [userId, get, errorMessage]);

  useEffect(() => {
    void fetchUser();
  }, [fetchUser]);

  const handleSubmit = async (data: OrganizationUserFormData) => {
    if (!userId) return;
    setLoading(true);
    try {
      await patch(`organizations/{${OrganizationUserResource}}/{id:${userId}}`, {
        email: data.email,
        is_active: data.is_active,
      });
      successMessage(`User ${data.username} updated successfully!`);
      navigate(backTo);
    } catch (err: unknown) {
      let errorMsg = 'Failed to update user';
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
        <Title>Manage Organization User</Title>
        <div className="mt-6 text-sm text-gray-500">Loading...</div>
      </div>
    );
  }

  const sideMenuItems: SideMenuItem[] = [
    {
      id: 'details',
      label: 'Details',
      content: (
        <>
          {userId && (
            <div className="mb-4">
              <ResendActivationButton
                userId={userId}
                activationPending={activationPending}
                isActive={initialData?.is_active ?? true}
                onSent={fetchUser}
              />
            </div>
          )}
          <OrganizationUserForm
            initialData={initialData}
            onSubmit={handleSubmit}
            loading={loading}
            submitLabel="Update User"
            cancelTo={backTo}
          />
        </>
      ),
    },
    {
      id: 'scopes',
      label: 'Scopes',
      href: userId ? `/organizations/${orgId}/users/edit/${userId}/scopes` : undefined,
    },
  ];

  if (orgId && userId) {
    sideMenuItems.push({
      id: 'profile',
      label: 'Profile',
      href: `/organizations/${orgId}/users/${userId}/profile`,
      scopes: [
        AppScopes.OrganizationsUsersProfileRead,
        AppScopes.OrganizationsUsersProfile,
        AppScopes.UserMe,
        AppScopes.Organizations,
        AppScopes.SuperAdmin,
      ],
    });
  }

  return (
    <div className="max-w-full">
      <OrganizationHeader organizationId={orgId} />
      <Title>Manage Organization User</Title>
      <div className="mt-6">
        <SideMenu items={sideMenuItems} initialActiveId="details" width="md" />
      </div>
    </div>
  );
};

export default EditOrganizationUser;
