import React, { useEffect, useState, useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../Shared/Components/Font/Title';
import { Submit } from '../../../Shared/Components/Button';
import { ConfirmModal } from '../../../Shared/Components/Modal';
import { Masonry } from '../../../Shared/Components/Masonry';
import NoEntriesFound from '../../../Shared/Components/NoEntries/NoEntriesFound';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../config/security/scopes';
import { type Application, ApplicationSchema, ApplicationResource } from '../model/application';
import { mapObjects } from '../../../config/data/mapper/mapper';
import { buildFilterQueryString } from '../../../config/data/resource/filters';
import { ApplicationHeader } from '../Shared/ApplicationHeader';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';
import { ApplicationCard } from './Partials/ApplicationCard';

const ApplicationsDashboard: React.FC = () => {
    useBreadcrumbItems([{ label: 'Workspaces', href: '/workspaces' }, { label: 'Applications' }]);
    const { workspaceId } = useParams<{ workspaceId: string }>();
    const navigate = useNavigate();
    const { get, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [applications, setApplications] = useState<Application[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [searchTerm, setSearchTerm] = useState('');

    const [isConfirmOpen, setIsConfirmOpen] = useState(false);
    const [appToDelete, setAppToDelete] = useState<{ id: string, title: string } | null>(null);
    const [isDeleting, setIsDeleting] = useState(false);

    const fetchApplications = React.useCallback(async () => {
        if (!workspaceId) return;
        setLoading(true);
        try {
            const qs = buildFilterQueryString([{ field: 'workspace_id', operator: 'exact', value: workspaceId }]);
            const response = await get<{ message?: unknown }>(`applications/{${ApplicationResource}}?${qs}`);
            const data = response?.message || response || [];
            if (Array.isArray(data)) {
                const mapped = mapObjects(ApplicationSchema, data) as unknown as Application[];
                setApplications(mapped);
            } else {
                setApplications([]);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to fetch applications';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [workspaceId, get, errorMessage]);

    useEffect(() => {
        void fetchApplications();
    }, [fetchApplications]);

    const promptDelete = React.useCallback((id: string, title: string) => {
        setAppToDelete({ id, title });
        setIsConfirmOpen(true);
    }, []);

    const confirmDelete = React.useCallback(async () => {
        if (!appToDelete) return;
        setIsDeleting(true);
        try {
            await del(`applications/{${ApplicationResource}}/{id:${appToDelete.id}}`);
            successMessage(`Application ${appToDelete.title} deleted successfully!`);
            void fetchApplications();
            setIsConfirmOpen(false);
        } catch {
            errorMessage('Failed to delete application');
        } finally {
            setIsDeleting(false);
        }
    }, [del, fetchApplications, successMessage, errorMessage, appToDelete]);

    const filteredApps = useMemo(() => {
        const term = searchTerm.trim().toLowerCase();
        if (!term) return applications;
        return applications.filter(a =>
            a.title.toLowerCase().includes(term) ||
            a.clientId.toLowerCase().includes(term) ||
            (a.description || '').toLowerCase().includes(term)
        );
    }, [applications, searchTerm]);

    if (!workspaceId) return <div>Missing workspace id.</div>;

    return (
        <div>
            <ApplicationHeader workspaceId={workspaceId} />

            <div className="flex justify-between items-center mb-6">
                <Title>Applications</Title>
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsWrite, AppScopes.Applications, AppScopes.SuperAdmin]}>
                    <Submit
                        type="button"
                        onClick={() => navigate(`/workspaces/${workspaceId}/applications/create`)}
                        label="Create Application"
                    />
                </ScopeBasedComponentAccess>
            </div>

            {loading && applications.length === 0 ? (
                <div>Loading applications...</div>
            ) : applications.length === 0 ? (
                <div className="mt-8">
                    <NoEntriesFound
                        title="No Applications Found"
                        message="There are no applications in this workspace yet. Create the first one."
                    />
                </div>
            ) : (
                <>
                    <div className="mb-4">
                        <input
                            type="text"
                            value={searchTerm}
                            onChange={e => setSearchTerm(e.target.value)}
                            placeholder="Search by title, client ID or description"
                            className="w-full md:max-w-md px-3 py-2 text-[0.875rem] rounded-lg border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                        />
                    </div>

                    {filteredApps.length === 0 ? (
                        <div className="mt-8">
                            <NoEntriesFound
                                title="No matches"
                                message="No applications match your search."
                            />
                        </div>
                    ) : (
                        <Masonry
                            columns={1}
                            gap={16}
                            breakpointCols={{ 640: 2, 1024: 3, 1440: 4 }}
                        >
                            {filteredApps.map(app => (
                                <ApplicationCard
                                    key={app.id}
                                    application={app}
                                    workspaceId={workspaceId}
                                    onDelete={promptDelete}
                                />
                            ))}
                        </Masonry>
                    )}
                </>
            )}

            <ConfirmModal
                isOpen={isConfirmOpen}
                onClose={() => setIsConfirmOpen(false)}
                onConfirm={confirmDelete}
                title="Delete Application"
                message={appToDelete ? `Are you sure you want to delete "${appToDelete.title}"?` : ''}
                confirmLabel="Delete"
                isLoading={isDeleting}
                variant="danger"
            />
        </div>
    );
};

export default ApplicationsDashboard;
