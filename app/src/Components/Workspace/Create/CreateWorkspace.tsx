import React, { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import Title from '../../../Shared/Components/Font/Title';
import { Back } from '../../../Shared/Components/Button';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { WorkspaceForm, type WorkspaceFormData } from '../Form/WorkspaceForm';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { WorkspaceResource } from '../model/workspace';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

export const CreateWorkspace: React.FC = () => {
    const { orgId } = useParams<{ orgId?: string }>();
    useBreadcrumbItems(
        orgId
            ? [{ label: 'Organizations', href: '/organizations' }, { label: 'Create Workspace' }]
            : [{ label: 'Workspaces', href: '/workspaces' }, { label: 'Create' }]
    );
    const [loading, setLoading] = useState(false);
    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const navigate = useNavigate();

    const backTo = orgId ? `/organizations/edit/${orgId}` : '/workspaces';
    const backLabel = orgId ? 'Back to organization' : 'Back to workspaces';

    const handleSubmit = async (data: WorkspaceFormData) => {
        setLoading(true);

        try {
            const payload: Record<string, unknown> = {
                workspace_id: data.workspace_id,
                title: data.title,
                description: data.description,
                organization_id: data.organization_id,
                is_active: data.is_active,
            };

            const response = await post<{ message?: { id: string }, id?: string }>(`workspaces/{${WorkspaceResource}}`, payload);

            const rawData = response?.message || response;
            const newId = rawData?.id;
            successMessage(`Workspace ${data.title} created successfully!`);

            if (orgId) {
                navigate(`/organizations/edit/${orgId}`);
            } else if (newId) {
                navigate(`/workspaces/edit/${newId}`);
            } else {
                navigate(`/workspaces`);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to create workspace';
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
                <Back to={backTo} label={backLabel} />
                <Title className="mb-0">Create Workspace</Title>
            </div>
            <WorkspaceForm
                onSubmit={handleSubmit}
                loading={loading}
                submitLabel="Create Workspace"
                lockedOrganizationId={orgId}
                cancelTo={backTo}
            />
        </div>
    );
};

export default CreateWorkspace;
