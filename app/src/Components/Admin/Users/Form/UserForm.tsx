import React, { useState } from 'react';

import { Switch } from '../../../../Shared/Components/Switch';
import { Submit, Cancel } from '../../../../Shared/Components/Button';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { FormInput } from '../../../../Shared/Components/Form';

export interface UserFormData {
    username: string;
    email: string;
    password?: string;
    is_active: boolean;
    is_super_admin: boolean;
}

interface UserFormProps {
    initialData?: Partial<UserFormData>;
    onSubmit: (data: UserFormData) => Promise<void>;
    loading: boolean;
    isEditMode?: boolean;
    submitLabel?: string;
}

export const UserForm: React.FC<UserFormProps> = ({
    initialData,
    onSubmit,
    loading,
    isEditMode = false,
    submitLabel = 'Save',
}) => {
    const [username, setUsername] = useState(initialData?.username || '');
    const [email, setEmail] = useState(initialData?.email || '');
    const [password, setPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [isActive, setIsActive] = useState(initialData?.is_active ?? true);
    const [isSuperAdmin, setIsSuperAdmin] = useState(initialData?.is_super_admin ?? false);
    const [error, setError] = useState<string | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);

        if (isEditMode && password && password !== confirmPassword) {
            setError('Passwords do not match');
            return;
        }

        await onSubmit({
            username,
            email,
            ...(isEditMode && { password: password || undefined }),
            is_active: isActive,
            is_super_admin: isSuperAdmin,
        });
        if (isEditMode) {
            setPassword('');
            setConfirmPassword('');
        }
    };

    return (
        <form onSubmit={handleSubmit} className="space-y-6 mt-6">
            {error && (
                <div className="mb-4 p-3 text-red-700 bg-red-100 rounded-lg border border-red-200">
                    {error}
                </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <FormInput
                    id="username"
                    label="Username"
                    value={username}
                    onChange={setUsername}
                    required
                    autoComplete="off"
                />
                <FormInput
                    id="email"
                    type="email"
                    label="Email"
                    value={email}
                    onChange={setEmail}
                    required
                    autoComplete="off"
                />
                {isEditMode && (
                    <>
                        <FormInput
                            id="password"
                            type="password"
                            label="New Password (optional)"
                            value={password}
                            onChange={setPassword}
                            minLength={8}
                            autoComplete="new-password"
                        />
                        <FormInput
                            id="confirmPassword"
                            type="password"
                            label="Confirm Password"
                            value={confirmPassword}
                            onChange={setConfirmPassword}
                            required={password.length > 0}
                            minLength={8}
                            autoComplete="new-password"
                        />
                    </>
                )}
            </div>

            <div className="flex items-center space-x-6 mt-4 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                <Switch
                    id="is_active"
                    checked={isActive}
                    onChange={setIsActive}
                    label="Is Active"
                />

                <ScopeBasedComponentAccess requiredScopes={[AppScopes.SuperAdmin]}>
                    <Switch
                        id="is_super_admin"
                        checked={isSuperAdmin}
                        onChange={setIsSuperAdmin}
                        label="Super Admin"
                    />
                </ScopeBasedComponentAccess>
            </div>

            <div className="flex justify-start gap-4 items-center pt-4">
                <Submit
                    loading={loading}
                    label={submitLabel}
                />
                <Cancel to="/admin/users" />
            </div>
        </form>
    );
};
