import React, { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { LinkAction } from '../../../../Shared/Components/Actions/Link';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';
import { formatDate } from '../../../../config/data/date/date';

interface ArchiveSummary {
    id: string;
    started_at: string;
    archived_at: string;
    row_count: number;
    size_bytes: number;
}

const formatBytes = (n: number): string => {
    if (n <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
    const value = n / Math.pow(1024, i);
    return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`;
};

const Field: React.FC<{ label: string; value: React.ReactNode }> = ({ label, value }) => (
    <div>
        <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">{label}</div>
        <div className="text-sm text-gray-900 dark:text-gray-100">{value}</div>
    </div>
);

// ArchiveDetail is a standalone page (own route, no SecurityPage tab
// shell) — same reasoning as AttackDetail.tsx: once you've drilled
// into one specific archive, you've left the tab-switching UI, so
// nesting inside the Bans/Allowlist/Attacks/Archives tab bar would be
// misleading. See plan/ip-attacks-db-archiving/plan.md.
export const ArchiveDetail: React.FC = () => {
    const { id } = useParams<{ id: string }>();
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Security', href: '/admin/security/archives' }, { label: 'Archive' }]);

    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [archive, setArchive] = useState<ArchiveSummary | null>(null);
    const [loading, setLoading] = useState(true);

    const load = useCallback(async () => {
        if (!id) return;
        setLoading(true);
        try {
            const resp = await get<{ message: ArchiveSummary }>(`admin/security/archives/{id:${id}}`);
            setArchive(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load archive.');
        } finally {
            setLoading(false);
        }
    }, [id, get, errorMessage]);

    useEffect(() => {
        void load();
    }, [load]);

    return (
        <div>
            <div className="mb-4">
                <Link
                    to="/admin/security/archives"
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    ← Back to archives
                </Link>
            </div>

            <Title>Archive</Title>

            {loading && !archive ? (
                <div className="mt-6 text-sm text-gray-500">Loading...</div>
            ) : !archive ? (
                <div className="mt-6 text-sm text-red-500">Archive not found.</div>
            ) : (
                <div className="mt-6 space-y-6">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <Field label="Started" value={formatDate(archive.started_at)} />
                        <Field label="Archived" value={formatDate(archive.archived_at)} />
                        <Field label="Rows" value={archive.row_count.toLocaleString()} />
                        <Field label="Size" value={formatBytes(archive.size_bytes)} />
                    </div>

                    <LinkAction to={`/admin/security/archives/${archive.id}/attacks`} label="Browse attack episodes" />
                </div>
            )}
        </div>
    );
};

export default ArchiveDetail;
