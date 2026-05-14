import React, { useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { Link } from 'react-router-dom';
import { OrganizationResource, type Organization, OrganizationSchema } from '../model/organization';
import { mapObjects } from '../../../config/data/mapper/mapper';

interface OrganizationHeaderProps {
    organizationId: string;
}

export const OrganizationHeader: React.FC<OrganizationHeaderProps> = ({ organizationId }) => {
    const [organization, setOrganization] = useState<Organization | null>(null);
    const [loading, setLoading] = useState(true);

    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    useEffect(() => {
        if (!organizationId) return;
        let cancelled = false;
        (async () => {
            try {
                const response = await get<{ message: unknown }>(`organizations/{${OrganizationResource}}/{id:${organizationId}}`);
                const raw = response?.message || response;
                if (raw && !cancelled) {
                    const mapped = mapObjects(OrganizationSchema, [raw]) as unknown as Organization[];
                    setOrganization(mapped[0] ?? null);
                }
            } catch (err: unknown) {
                let errorMsg = 'Failed to load organization';
                if (err instanceof Error) errorMsg = err.message || errorMsg;
                errorMessage(errorMsg);
            } finally {
                if (!cancelled) setLoading(false);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [organizationId, get, errorMessage]);

    if (loading) {
        return (
            <div className="mb-6 pb-4 border-b border-gray-200 dark:border-gray-700">
                <div className="text-sm text-gray-500">Loading organization...</div>
            </div>
        );
    }

    if (!organization) {
        return (
            <div className="mb-6 pb-4 border-b border-gray-200 dark:border-gray-700">
                <div className="text-sm text-red-500">Organization not found.</div>
            </div>
        );
    }

    return (
        <div className="mb-6 pb-4 border-b border-gray-200 dark:border-gray-700">
            <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Organization</div>
            <div className="flex items-baseline justify-between flex-wrap gap-2">
                <div>
                    <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">{organization.title}</h2>
                    {organization.description && (
                        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{organization.description}</p>
                    )}
                </div>
                <Link
                    to={`/organizations/edit/${organization.id}`}
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    ← Back to organization
                </Link>
            </div>
        </div>
    );
};

export default OrganizationHeader;
