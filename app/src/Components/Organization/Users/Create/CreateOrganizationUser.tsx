import React, { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { OrganizationUserResource } from '../../model/organizationUser';
import { OrganizationUserForm, type OrganizationUserFormData } from '../Form/OrganizationUserForm';
import { OrganizationHeader } from '../../Shared/OrganizationHeader';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const CreateOrganizationUser: React.FC = () => {
    useBreadcrumbItems([{ label: 'Organizations', href: '/organizations' }, { label: 'Users' }, { label: 'Create' }]);
    const { orgId } = useParams<{ orgId: string }>();
    const [loading, setLoading] = useState(false);
    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const navigate = useNavigate();

    const backTo = `/organizations/${orgId}/users`;

    const handleSubmit = async (data: OrganizationUserFormData) => {
        if (!orgId) return;
        setLoading(true);
        try {
            const response = await post<{
                message?: {
                    user?: { id?: string };
                    activation_error?: string;
                };
            }>(
                `organizations/{${OrganizationUserResource}}`,
                {
                    username: data.username,
                    email: data.email,
                    organization_id: orgId,
                    is_active: data.is_active,
                    ...(data.redirect_application_id ? { redirect_application_id: data.redirect_application_id } : {}),
                }
            );
            const payload = response?.message;
            const newId = payload?.user?.id;

            if (payload?.activation_error) {
                successMessage(`User ${data.username} created — but the invite email could not be sent: ${payload.activation_error}`);
            } else {
                successMessage(`User ${data.username} created and invite email sent.`);
            }

            if (newId) {
                navigate(`/organizations/${orgId}/users/edit/${newId}`);
            } else {
                navigate(backTo);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to create user';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
            throw err;
        } finally {
            setLoading(false);
        }
    };

    if (!orgId) return <div>Missing organization id.</div>;

    return (
        <div className="max-w-2xl">
            <OrganizationHeader organizationId={orgId} />
            <Title>Create Organization User</Title>
            <OrganizationUserForm
                onSubmit={handleSubmit}
                loading={loading}
                submitLabel="Create User"
                cancelTo={backTo}
                organizationId={orgId}
            />
        </div>
    );
};

export default CreateOrganizationUser;
