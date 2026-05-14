import React, { useEffect, useState } from 'react';
import Title from '../../../Shared/Components/Font/Title';
import { Submit } from '../../../Shared/Components/Button';
import { FormInput } from '../../../Shared/Components/Form';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { useAdminProfile } from '../../Auth/Profile/useAdminProfile';
import { profileDisplayName } from '../../Auth/Profile/profileDisplayName';
import { absolutePublicURL } from '../../../api/client';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

// ProfilePage edits the authenticated admin's own profile row.
// Reads /api/v1/admin/users/me on mount (via AdminProfileProvider),
// PATCHes back on save. Avatar upload + delete talk to dedicated
// endpoints. Every mutation calls `refresh()` so the top-bar
// UserMenu (same provider) updates immediately.
export const ProfilePage: React.FC = () => {
    useBreadcrumbItems([{ label: 'Profile' }]);
    const { patch, postMultipart, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const { profile, loading, refresh } = useAdminProfile();

    const [firstName, setFirstName] = useState('');
    const [lastName, setLastName] = useState('');
    const [phone, setPhone] = useState('');
    const [locale, setLocale] = useState('');
    const [timezone, setTimezone] = useState('');
    const [saving, setSaving] = useState(false);
    const [uploading, setUploading] = useState(false);

    // Sync local form state from the context-loaded profile
    // whenever it arrives or changes. Avoids a dual source of
    // truth while keeping the form editable.
    useEffect(() => {
        if (!profile) return;
        setFirstName(profile.first_name ?? '');
        setLastName(profile.last_name ?? '');
        setPhone(profile.phone ?? '');
        setLocale(profile.locale ?? '');
        setTimezone(profile.timezone ?? '');
    }, [profile]);

    const save = async () => {
        setSaving(true);
        try {
            await patch('admin/users/me', {
                first_name: firstName,
                last_name: lastName,
                phone,
                locale,
                timezone,
            });
            await refresh();
            successMessage('Profile saved.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to save profile.');
        } finally {
            setSaving(false);
        }
    };

    const onAvatarPicked = async (file: File) => {
        setUploading(true);
        try {
            const fd = new FormData();
            fd.append('file', file);
            await postMultipart('admin/users/me/avatar', fd);
            await refresh();
            successMessage('Avatar updated.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to upload avatar.');
        } finally {
            setUploading(false);
        }
    };

    const clearAvatar = async () => {
        setUploading(true);
        try {
            await del('admin/users/me/avatar');
            await refresh();
            successMessage('Avatar removed.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to remove avatar.');
        } finally {
            setUploading(false);
        }
    };

    if (loading && !profile) {
        return <div className="text-sm text-gray-500 p-4">Loading profile…</div>;
    }

    const avatarURL = profile?.avatar_url ? absolutePublicURL(profile.avatar_url) : '';
    const displayName = profileDisplayName(profile);

    return (
        <div className="max-w-2xl">
            <Title>Profile</Title>

            {profile && (
                <div className="flex items-center gap-4 mt-4 mb-6">
                    {avatarURL ? (
                        <img
                            src={avatarURL}
                            alt=""
                            className="w-20 h-20 rounded-full object-cover bg-gray-100 dark:bg-surface-700"
                        />
                    ) : (
                        <div className="flex items-center justify-center w-20 h-20 rounded-full bg-blue-100 dark:bg-blue-900/50">
                            <svg className="w-10 h-10 text-blue-600 dark:text-blue-300" fill="currentColor" viewBox="0 0 20 20">
                                <path fillRule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clipRule="evenodd" />
                            </svg>
                        </div>
                    )}
                    <div className="flex flex-col gap-2">
                        <div>
                            <div className="text-sm font-semibold text-gray-900 dark:text-gray-100">
                                {displayName}
                            </div>
                            <div className="text-xs text-gray-500">{profile.email}</div>
                        </div>
                        <div className="flex items-center gap-2">
                            <label
                                className="inline-flex items-center px-3 py-1.5 text-xs font-medium rounded-md border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-800 hover:bg-gray-50 dark:hover:bg-surface-700 cursor-pointer"
                            >
                                {uploading ? 'Working…' : 'Upload avatar'}
                                <input
                                    type="file"
                                    accept="image/png,image/jpeg,image/webp,image/gif"
                                    className="hidden"
                                    disabled={uploading}
                                    onChange={async e => {
                                        const file = e.target.files?.[0];
                                        e.target.value = ''; // allow re-upload of same file
                                        if (file) await onAvatarPicked(file);
                                    }}
                                />
                            </label>
                            {avatarURL && (
                                <button
                                    type="button"
                                    onClick={() => void clearAvatar()}
                                    disabled={uploading}
                                    className="px-3 py-1.5 text-xs font-medium rounded-md border border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/50 dark:text-red-300 dark:hover:bg-red-900/30 disabled:opacity-40"
                                >
                                    Remove
                                </button>
                            )}
                        </div>
                    </div>
                </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <FormInput
                    id="first_name"
                    label="First name"
                    value={firstName}
                    onChange={setFirstName}
                />
                <FormInput
                    id="last_name"
                    label="Last name"
                    value={lastName}
                    onChange={setLastName}
                />
                <FormInput
                    id="phone"
                    label="Phone"
                    value={phone}
                    onChange={setPhone}
                    placeholder="+49 30 …"
                />
                <FormInput
                    id="locale"
                    label="Locale"
                    value={locale}
                    onChange={setLocale}
                    placeholder="en-US"
                />
                <FormInput
                    id="timezone"
                    label="Timezone"
                    value={timezone}
                    onChange={setTimezone}
                    placeholder="Europe/Berlin"
                />
            </div>

            <div className="mt-6 flex items-center gap-3">
                <Submit type="button" onClick={() => void save()} loading={saving} label="Save profile" />
            </div>
        </div>
    );
};

export default ProfilePage;
