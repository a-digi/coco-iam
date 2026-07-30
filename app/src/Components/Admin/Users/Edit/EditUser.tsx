import React, { useEffect, useState } from 'react';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { Link, useParams } from 'react-router-dom';
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
  const [newPassword, setNewPassword] = useState('');
  const [confirmNewPassword, setConfirmNewPassword] = useState('');
  const [showResetPassword, setShowResetPassword] = useState(false);
  const [resettingPassword, setResettingPassword] = useState(false);
  const [isActive, setIsActive] = useState(true);
  const [isSuperAdmin, setIsSuperAdmin] = useState(false);
  const [inheritedScopes, setInheritedScopes] = useState<InheritedScopes[]>([]);

  const { get, patch, post } = useHttpClient();
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

  // Admin-privileged reset for ANOTHER user's password — deliberately a
  // separate endpoint from the generic PATCH used elsewhere in this
  // file, and deliberately never shown for your own account (see the
  // "auth" accordion item below): resetting someone else's password
  // needs no "old password" at all, since the privilege boundary is
  // the admin:users:write/super:admin scope on this route, not a
  // secret the admin has no way of knowing. Self password changes stay
  // on the dedicated Change Password page, which correctly still
  // requires your current password.
  const handleResetPassword = (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword !== confirmNewPassword) {
      errorMessage('Passwords do not match');
      return;
    }
    if (newPassword.length < 8) {
      errorMessage('Password must be at least 8 characters long');
      return;
    }

    setResettingPassword(true);
    post(`admin/users/{id:${id}}/reset-password`, { new_password: newPassword })
      .then(() => {
        successMessage('Password reset successfully!');
        setNewPassword('');
        setConfirmNewPassword('');
        setShowResetPassword(false);
      })
      .catch((err: unknown) => {
        errorMessage(err instanceof Error ? err.message : 'Failed to reset password');
      })
      .finally(() => setResettingPassword(false));
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
      scopes: [
        ...(id === authToken?.user?.id ? [AppScopes.UserMe] : []),
        AppScopes.AdminUsersWrite,
        AppScopes.SuperAdmin
      ],
      content: id === authToken?.user?.id ? (
        <div className="space-y-3">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Manage your own password from the dedicated Change Password page — it correctly
            verifies your current password first.
          </p>
          <Link
            to="/account/change-password"
            className="inline-block text-sm font-medium text-indigo-600 dark:text-indigo-400 hover:underline"
          >
            Change my password →
          </Link>
        </div>
      ) : (
        <form onSubmit={handleResetPassword} className="space-y-4">
          <p className="text-xs text-amber-700 dark:text-amber-400">
            You are resetting {username || 'this user'}&apos;s password as an administrator —
            they&apos;ll need to use this new password to log in next time.
          </p>
          <FormInput
            id="newPassword"
            type={showResetPassword ? 'text' : 'password'}
            label="New Password"
            value={newPassword}
            onChange={setNewPassword}
            minLength={8}
            required
            trailing={
              <button
                type="button"
                onClick={() => setShowResetPassword(v => !v)}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                aria-label={showResetPassword ? 'Hide password' : 'Show password'}
                tabIndex={-1}
              >
                {showResetPassword ? (
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88" />
                  </svg>
                ) : (
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
                  </svg>
                )}
              </button>
            }
          />
          <FormInput
            id="confirmNewPassword"
            type={showResetPassword ? 'text' : 'password'}
            label="Confirm New Password"
            value={confirmNewPassword}
            onChange={setConfirmNewPassword}
            minLength={8}
            required
          />
          <div className="flex justify-end">
            <Submit
              loading={resettingPassword}
              loadingText="Resetting..."
              disabled={!newPassword || !confirmNewPassword}
              label="Reset Password"
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
