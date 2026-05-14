import React, { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import Title from '../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { ApplicationResource } from '../model/application';
import { ApplicationHeader } from '../Shared/ApplicationHeader';
import { Switch } from '../../../Shared/Components/Switch';
import { Submit, Cancel } from '../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../Shared/Components/Form';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

const slugify = (s: string) =>
    s.toLowerCase()
        .trim()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '')
        .slice(0, 60);

export const CreateApplication: React.FC = () => {
    useBreadcrumbItems([{ label: 'Workspaces', href: '/workspaces' }, { label: 'Applications' }, { label: 'Create' }]);
    const { workspaceId } = useParams<{ workspaceId: string }>();
    const [loading, setLoading] = useState(false);
    const [title, setTitle] = useState('');
    const [clientId, setClientId] = useState('');
    const [clientIdTouched, setClientIdTouched] = useState(false);
    const [description, setDescription] = useState('');
    const [isActive, setIsActive] = useState(true);
    const [allowRecovery, setAllowRecovery] = useState(true);
    const [allowRegistration, setAllowRegistration] = useState(false);

    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const navigate = useNavigate();

    const backTo = `/workspaces/${workspaceId}/applications`;

    const suggestedClientId = clientIdTouched ? clientId : slugify(title);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!workspaceId) return;
        setLoading(true);
        try {
            const response = await post<{ message?: { id: string }, id?: string }>(
                `applications/{${ApplicationResource}}`,
                {
                    workspace_id: workspaceId,
                    client_id: suggestedClientId,
                    title,
                    description,
                    is_active: isActive,
                    allow_recovery: allowRecovery,
                    allow_registration: allowRegistration,
                }
            );
            const rawData = response?.message || response;
            const newId = rawData?.id;
            successMessage(`Application ${title} created successfully!`);
            if (newId) {
                navigate(`/workspaces/${workspaceId}/applications/edit/${newId}`);
            } else {
                navigate(backTo);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to create application';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    };

    if (!workspaceId) return <div>Missing workspace id.</div>;

    return (
        <div className="max-w-2xl">
            <ApplicationHeader workspaceId={workspaceId} />
            <Title>Create Application</Title>
            <form onSubmit={handleSubmit} className="space-y-6 mt-6">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <FormInput
                        id="title"
                        label="Title"
                        value={title}
                        onChange={setTitle}
                        required
                    />
                    <FormInput
                        id="clientId"
                        label="Client ID"
                        value={suggestedClientId}
                        onChange={e => { setClientId(e); setClientIdTouched(true); }}
                        required
                        placeholder="auto-generated from title"
                        description="Unique identifier, lowercase, no spaces."
                        inputClassName="font-mono text-sm"
                    />
                    <FormTextarea
                        id="description"
                        label="Description"
                        value={description}
                        onChange={setDescription}
                        className="md:col-span-2"
                    />
                </div>

                <div className="flex flex-col gap-3 mt-4 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                    <Switch id="is_active" checked={isActive} onChange={setIsActive} label="Is Active" />
                    <Switch
                        id="allow_recovery"
                        checked={allowRecovery}
                        onChange={setAllowRecovery}
                        label="Allow password recovery"
                    />
                    <Switch
                        id="allow_registration"
                        checked={allowRegistration}
                        onChange={setAllowRegistration}
                        label="Allow registration"
                    />
                </div>

                <div className="flex justify-start gap-4 items-center pt-4">
                    <Submit loading={loading} label="Create Application" />
                    <Cancel to={backTo} />
                </div>
            </form>
        </div>
    );
};

export default CreateApplication;
