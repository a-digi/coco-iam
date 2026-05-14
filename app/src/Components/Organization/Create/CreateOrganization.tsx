import React, { useState } from 'react';
import Title from '../../../Shared/Components/Font/Title';
import { Back } from '../../../Shared/Components/Button';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { OrganizationForm, type OrganizationFormData } from '../Form/OrganizationForm';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { useNavigate } from 'react-router-dom';
import { OrganizationResource } from '../model/organization';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

export const CreateOrganization: React.FC = () => {
    useBreadcrumbItems([{ label: 'Organizations', href: '/organizations' }, { label: 'Create' }]);
    const [loading, setLoading] = useState(false);
    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const navigate = useNavigate();

    const handleSubmit = async (data: OrganizationFormData) => {
        setLoading(true);

        try {
            const payload: Record<string, unknown> = {
                organization_id: data.organization_id,
                title: data.title,
                description: data.description,
                is_active: data.is_active,
            };

            const response = await post<{ message?: { id: string }, id?: string }>(`organizations/{${OrganizationResource}}`, payload);

            const rawData = response?.message || response;
            const newId = rawData?.id;
            successMessage(`Organization ${data.title} created successfully!`);

            if (newId) {
                navigate(`/organizations/edit/${newId}`);
            } else {
                navigate(`/organizations`);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to create organization';
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
            <div className="flex items-center gap-3 mb-4">
                <Back to="/organizations" label="Back to organizations" />
                <Title className="mb-0">Create Organization</Title>
            </div>
            <OrganizationForm
                onSubmit={handleSubmit}
                loading={loading}
                submitLabel="Create Organization"
            />
        </div>
    );
};

export default CreateOrganization;
