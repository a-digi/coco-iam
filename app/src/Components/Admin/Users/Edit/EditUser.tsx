import React, { useEffect, useState } from 'react';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useParams } from 'react-router-dom';
import { type User, StandardSchema } from '../model/user';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Accordions, type AccordionData } from '../../../../Shared/Components/Accordion';
import { Switch } from '../../../../Shared/Components/Switch';
import { Scopes, type InheritedScopes } from '../../../Auth/Scopes/Partials/Scopes';
import { UserGroups } from './Partial/UserGroups';
import { SendActivationButton } from './Partial/SendActivationButton';
import { AppScopes } from '../../../../config/security/scopes';
import { useAuth } from '../../../../Components/Auth/Guard/useAuth';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput } from '../../../../Shared/Components/Form';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const EditUser: React.FC = () => {
  useBreadcrumbItems([{ label: 'Admin' }, { label: 'Users', href: '/admin/users' }, { label: 'Edit' }]);
  const { id } = useParams<{ id: string }>();
  const { authToken } = useAuth();
  const [fetching, setFetching] = useState(true);
  const [loading, setLoading] = useState(false);
  const [originalUser, setOriginalUser] = useState<User | null>(null);

  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [oldPassword, setOldPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isActive, setIsActive] = useState(true);
  const [isSuperAdmin, setIsSuperAdmin] = useState(false);
  const [inheritedScopes, setInheritedScopes] = useState<InheritedScopes[]>([]);

  const { get, patch } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();

  const fetchUser = React.useCallback(async () => {
    if (!id) return;
    setFetching(true);
    try {
      const response = await get<{ message: unknown }>(`admin/{res:users}/{id:${id}}`);
      const rawUser = response?.message || response;

      if (rawUser) {
        const mappedUsers = mapObjects(StandardSchema, [rawUser]) as unknown as User[];
        const user = mappedUsers[0];
        setOriginalUser(user);
        setUsername(user.username || '');
        setEmail(user.email || '');
        setIsActive(user.isActive ?? true);
        setIsSuperAdmin(user.isSuperAdmin ?? false);
      } else {
        errorMessage('User not found');
      }

    } catch (err: unknown) {
      let errorMsg = 'Failed to fetch user data';
      if (err instanceof Error) {
        errorMsg = err.message || errorMsg;
      }
      errorMessage(errorMsg);
    } finally {
      setFetching(false);
    }
  }, [id, get, errorMessage]);

  useEffect(() => {
    void fetchUser();
  }, [fetchUser]);

  const handlePatch = async (payload: Record<string, unknown>, sectionName: string) => {
    setLoading(true);
    try {
      await patch(`admin/{res:users}/{id:${id}}`, payload);
      successMessage(`${sectionName} updated successfully!`);
      await fetchUser();
    } catch (err: unknown) {
      let errorMsg = `Failed to update ${sectionName}`;
      if (err instanceof Error) {
        errorMsg = err.message || errorMsg;
      }
      errorMessage(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  const handleEmailSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (originalUser && email !== originalUser.email) {
      errorMessage('Email is read-only and cannot be changed.');
      return;
    }
    void handlePatch({ email }, 'Email');
  };

  const handleAuthSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (password !== confirmPassword) {
      errorMessage('Passwords do not match');
      return;
    }

    if (password.length < 8) {
      errorMessage('Password must be at least 8 characters long');
      return;
    }

    void handlePatch({ password, old_password: oldPassword }, 'Authentication').then(() => {
      setPassword('');
      setConfirmPassword('');
      setOldPassword('');
    });

  };


  const accordionItems: AccordionData[] = [
    {
      id: 'email',
      title: 'Email',
      content: (
        <form onSubmit={handleEmailSubmit} className="space-y-4">
          <FormInput
            id="email"
            type="email"
            label="Email Address"
            value={email}
            onChange={() => { }}
            readOnly
            inputClassName="bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400 cursor-not-allowed"
          />
        </form>
      )
    },
    {
      id: 'auth',
      title: 'Authentication',
      content: (
        <form onSubmit={handleAuthSubmit} className="space-y-4">
          <FormInput
            id="oldPassword"
            type="password"
            label="Old Password"
            value={oldPassword}
            onChange={setOldPassword}
            minLength={8}
            required
          />
          <FormInput
            id="newPassword"
            type="password"
            label="New Password"
            value={password}
            onChange={setPassword}
            minLength={8}
            required
          />
          <FormInput
            id="confirmPassword"
            type="password"
            label="Confirm New Password"
            value={confirmPassword}
            onChange={setConfirmPassword}
            minLength={8}
            required
          />
          <div className="flex justify-end">
            <Submit
              loading={loading}
              loadingText="Saving..."
              disabled={!password || !confirmPassword}
              label="Update Password"
            />
          </div>
        </form>
      )
    },
    {
      id: 'active',
      title: 'Active & Hierarchy',
      scopes: [AppScopes.SuperAdmin],
      content: (
        <div className="space-y-6 py-2">
          <Switch
            id="is_active"
            checked={isActive}
            onChange={checked => {
              setIsActive(checked);
              void handlePatch({ is_active: checked, is_super_admin: isSuperAdmin }, 'Active Status');
            }}
            label="Active User"
            disabled={loading || id === authToken?.user?.id}
          />
          <Switch
            id="is_super_admin"
            checked={isSuperAdmin}
            onChange={checked => {
              setIsSuperAdmin(checked);
              void handlePatch({ is_active: isActive, is_super_admin: checked }, 'Super Administrator Status');
            }}
            label="Super Administrator"
            disabled={loading}
          />
          {id && (
            <SendActivationButton userId={id} isActive={isActive} onSent={fetchUser} />
          )}
        </div>
      )
    },
    {
      id: 'scopes',
      scopes: [
        ...(id === authToken?.user?.id ? [AppScopes.UserMe] : []),
        AppScopes.AdminUsersAclRead,
        AppScopes.AdminUsersAclWrite,
        AppScopes.SuperAdmin
      ],
      title: 'User Scopes',
      content: id ? <Scopes resourceKey={'user_id'} resourceName={'admin_acl'} entityId={id} inheritedScopes={inheritedScopes} /> : null,
      forceMount: true
    },
    {
      id: 'user_groups',
      title: 'User Groups',
      scopes: [
        ...(id === authToken?.user?.id ? [AppScopes.UserMe] : []),
        AppScopes.AdminGroupsRead,
        AppScopes.AdminGroupsWrite,
        AppScopes.SuperAdmin
      ],
      content: id ? <UserGroups entityId={id} onInheritedScopesChange={setInheritedScopes} /> : null,
      forceMount: true
    }
  ];

  if (fetching) {
    return (
      <div className="max-w-full">
        <Title>Edit Admin User</Title>
        <div className="mt-6 flex items-center justify-center p-6">
          <svg className="animate-spin h-6 w-6 text-indigo-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"></path>
          </svg>
          <span className="ml-3 text-gray-500">Loading user data...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-full">
      <Title>Edit Admin User</Title>

      {username && (
        <div className="mt-6">
          <p className="text-sm font-medium text-gray-500 mb-1">Username</p>
          <p className="text-xl font-bold text-gray-700 dark:text-gray-300 tracking-tight">{username}</p>
        </div>
      )}

      <div className="mt-6 space-y-4">
        <Accordions items={accordionItems} initialExpandedId="email" />
      </div>
    </div>
  );
};

export default EditUser;
