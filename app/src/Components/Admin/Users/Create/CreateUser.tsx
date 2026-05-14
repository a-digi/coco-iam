import React, { useState } from 'react';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { UserForm, type UserFormData } from '../Form/UserForm';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { useNavigate } from 'react-router-dom';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const CreateUser: React.FC = () => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Users', href: '/admin/users' }, { label: 'Create' }]);
    const [loading, setLoading] = useState(false);
    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const navigate = useNavigate();

    const handleSubmit = async (data: UserFormData) => {
        setLoading(true);
        try {
            const response = await post<{ message?: { user?: { id: string } } }>('admin/{res:users}', data);

            const newUserId = response?.message?.user?.id;
            successMessage(`User ${data.username} created successfully!`);

            if (newUserId) {
                navigate(`/admin/users/edit/${newUserId}`);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to create user';
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
            <Title>Create Admin User</Title>
            <UserForm
                onSubmit={handleSubmit}
                loading={loading}
                isEditMode={false}
                submitLabel="Create User"
            />
        </div>
    );
};

export default CreateUser;
