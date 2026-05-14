import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import Title from '../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit, Cancel } from '../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../Shared/Components/Form';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

const CreateQueue: React.FC = () => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Queue', href: '/admin/queue' }, { label: 'Create' }]);
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [loading, setLoading] = useState(false);

    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const navigate = useNavigate();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!name.trim()) return;
        setLoading(true);
        try {
            await post('admin/queue/queues', { name: name.trim(), description: description.trim() });
            successMessage(`Queue ${name} created.`);
            navigate('/admin/queue');
        } catch (err: unknown) {
            let errorMsg = 'Failed to create queue';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="max-w-2xl">
            <Title>Create Queue</Title>
            <form onSubmit={handleSubmit} className="space-y-6 mt-6">
                <FormInput
                    id="name"
                    label="Name"
                    value={name}
                    onChange={setName}
                    required
                    placeholder="e.g. email-notifications"
                    description="Unique identifier used by producers to publish. Lowercase, kebab-case recommended."
                    inputClassName="font-mono text-sm"
                />
                <FormTextarea
                    id="description"
                    label="Description"
                    value={description}
                    onChange={setDescription}
                    placeholder="What is this queue for?"
                />
                <div className="flex justify-start gap-4 items-center pt-4">
                    <Submit loading={loading} label="Create Queue" />
                    <Cancel to="/admin/queue" />
                </div>
            </form>
        </div>
    );
};

export default CreateQueue;
