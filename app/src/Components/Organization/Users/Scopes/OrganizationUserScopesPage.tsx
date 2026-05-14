import React from 'react';
import { useParams } from 'react-router-dom';
import Title from '../../../../Shared/Components/Font/Title';
import { OrganizationHeader } from '../../Shared/OrganizationHeader';
import { OrganizationUserScopes } from '../Edit/Partials/OrganizationUserScopes';
import { SideMenu, type SideMenuItem } from '../../../../Shared/Components/SideMenu';
import { AppScopes } from '../../../../config/security/scopes';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const OrganizationUserScopesPage: React.FC = () => {
  const { orgId, userId } = useParams<{ orgId: string; userId: string }>();
  useBreadcrumbItems([
    { label: 'Organizations', href: '/organizations' },
    { label: 'Users' },
    { label: 'Scopes' },
  ]);

  if (!orgId || !userId) return <div>Missing organization or user id.</div>;

  const sideMenuItems: SideMenuItem[] = [
    {
      id: 'details',
      label: 'Details',
      href: `/organizations/${orgId}/users/edit/${userId}`,
    },
    {
      id: 'scopes',
      label: 'Scopes',
      content: <OrganizationUserScopes userId={userId} />,
    },
    {
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
    },
  ];

  return (
    <div className="max-w-full">
      <OrganizationHeader organizationId={orgId} />
      <Title>Manage Organization User</Title>
      <div className="mt-6">
        <SideMenu items={sideMenuItems} initialActiveId="scopes" width="md" />
      </div>
    </div>
  );
};

export default OrganizationUserScopesPage;
