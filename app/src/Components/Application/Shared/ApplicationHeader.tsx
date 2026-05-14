import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { WorkspaceResource, type Workspace, WorkspaceSchema } from '../../Workspace/model/workspace';
import { ApplicationResource, type Application, ApplicationSchema } from '../model/application';
import { mapObjects } from '../../../config/data/mapper/mapper';

interface ApplicationHeaderProps {
    workspaceId: string;
    applicationId?: string;
}

export const ApplicationHeader: React.FC<ApplicationHeaderProps> = ({ workspaceId, applicationId }) => {
    const [workspace, setWorkspace] = useState<Workspace | null>(null);
    const [application, setApplication] = useState<Application | null>(null);
    const [loading, setLoading] = useState(true);

    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    useEffect(() => {
        let cancelled = false;
        (async () => {
            try {
                if (workspaceId) {
                    const wsRes = await get<{ message: unknown }>(`workspaces/{${WorkspaceResource}}/{id:${workspaceId}}`);
                    const rawWs = wsRes?.message || wsRes;
                    if (rawWs && !cancelled) {
                        const mapped = mapObjects(WorkspaceSchema, [rawWs]) as unknown as Workspace[];
                        setWorkspace(mapped[0] ?? null);
                    }
                }
                if (applicationId) {
                    const appRes = await get<{ message: unknown }>(`applications/{${ApplicationResource}}/{id:${applicationId}}`);
                    const rawApp = appRes?.message || appRes;
                    if (rawApp && !cancelled) {
                        const mapped = mapObjects(ApplicationSchema, [rawApp]) as unknown as Application[];
                        setApplication(mapped[0] ?? null);
                    }
                }
            } catch (err: unknown) {
                let errorMsg = 'Failed to load header context';
                if (err instanceof Error) errorMsg = err.message || errorMsg;
                errorMessage(errorMsg);
            } finally {
                if (!cancelled) setLoading(false);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [workspaceId, applicationId, get, errorMessage]);

    if (loading) {
        return (
            <div className="mb-6 pb-4 border-b border-gray-200 dark:border-gray-700">
                <div className="text-sm text-gray-500">Loading...</div>
            </div>
        );
    }

    // When no applicationId is set we're on the listing page — the consuming
    // component already renders its own "Applications" title and doesn't need
    // a back-link, so we only show the workspace breadcrumb there.
    const isListingContext = !applicationId;

    return (
        <div className="mb-6 p-4 rounded-lg bg-gradient-to-r from-indigo-50 to-white dark:from-surface-800 dark:to-surface-900 border border-indigo-100 dark:border-surface-800">
            <div className="flex flex-col gap-1">
                {workspace && (
                    <div className="flex items-center gap-2 text-xs uppercase tracking-wide text-indigo-600 dark:text-indigo-400">
                        <Link to={`/workspaces/edit/${workspace.id}`} className="hover:underline">
                            Workspace: {workspace.title}
                        </Link>
                        {application && <span aria-hidden="true">›</span>}
                        {application && <span>Application</span>}
                    </div>
                )}
                {application && (
                    <div>
                        <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">{application.title}</h2>
                        <div className="text-xs text-gray-500 mt-1">
                            <span className="font-mono">{application.clientId}</span>
                            {application.description && <span className="ml-2">— {application.description}</span>}
                        </div>
                    </div>
                )}
                {!isListingContext && (
                    <div className="mt-2">
                        <Link to={`/workspaces/${workspaceId}/applications`} className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400">
                            ← Back to applications
                        </Link>
                    </div>
                )}
            </div>
        </div>
    );
};

export default ApplicationHeader;
