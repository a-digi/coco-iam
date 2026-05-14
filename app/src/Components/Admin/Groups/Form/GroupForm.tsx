import React, { useState } from 'react';

import { Switch } from '../../../../Shared/Components/Switch';
import { Submit, Cancel } from '../../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../../Shared/Components/Form';

export interface GroupFormData {
    title: string;
    group_description: string;
    is_active: boolean;
}

interface GroupFormProps {
    initialData?: Partial<GroupFormData>;
    onSubmit: (data: GroupFormData) => Promise<void>;
    loading: boolean;
    submitLabel?: string;
}

export const GroupForm: React.FC<GroupFormProps> = ({
    initialData,
    onSubmit,
    loading,
    submitLabel = 'Save',
}) => {
    const [title, setTitle] = useState(initialData?.title || '');
    const [groupDescription, setGroupDescription] = useState(initialData?.group_description || '');
    const [isActive, setIsActive] = useState(initialData?.is_active ?? true);
    const [error, setError] = useState<string | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);

        await onSubmit({
            title,
            group_description: groupDescription,
            is_active: isActive,
        });
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
                    id="title"
                    label="Title"
                    value={title}
                    onChange={setTitle}
                    required
                    autoComplete="off"
                />
                <FormTextarea
                    id="groupDescription"
                    label="Group Description"
                    value={groupDescription}
                    onChange={setGroupDescription}
                    className="md:col-span-2"
                />
            </div>

            <div className="flex items-center space-x-6 mt-4 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                <Switch
                    id="is_active"
                    checked={isActive}
                    onChange={setIsActive}
                    label="Is Active"
                />
            </div>

            <div className="flex justify-start gap-4 items-center pt-4">
                <Submit
                    loading={loading}
                    label={submitLabel}
                />
                <Cancel to="/admin/groups" />
            </div>
        </form>
    );
};
