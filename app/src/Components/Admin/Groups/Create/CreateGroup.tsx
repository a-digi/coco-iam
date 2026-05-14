import React, { useState } from 'react';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { GroupForm, type GroupFormData } from '../Form/GroupForm';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { useNavigate } from 'react-router-dom';
import { AdminGroupResource } from '../model/group';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const CreateGroup: React.FC = () => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Groups', href: '/admin/groups' }, { label: 'Create' }]);
    const [loading, setLoading] = useState(false);
    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const navigate = useNavigate();

    const handleSubmit = async (data: GroupFormData) => {
        setLoading(true);

        try {
            const response = await post<{ message?: { id: string }, id?: string }>(`admin/{${AdminGroupResource}}`, data);

            const rawData = response?.message || response;
            const newGroupId = rawData?.id;
            successMessage(`Group ${data.title} created successfully!`);

            if (newGroupId) {
                navigate(`/admin/groups/edit/${newGroupId}`);
            } else {
                navigate(`/admin/groups`);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to create group';
            if (err instanceof Error) {
                errorMsg = err.message || errorMsg;
            }
            errorMessage(errorMsg);
            throw err;
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="max-w-2xl">
            <Title>Create Admin Group</Title>
            <GroupForm
                onSubmit={handleSubmit}
                loading={loading}
                submitLabel="Create Group"
            />
        </div>
    );
};

export default CreateGroup;
