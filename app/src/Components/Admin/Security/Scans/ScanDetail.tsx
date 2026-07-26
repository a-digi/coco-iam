import React, { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';
import { formatDate } from '../../../../config/data/date/date';

interface ScanDetailResponse {
    id: string;
    ip: string;
    started_at: string;
    last_seen_at: string;
    ended_at?: string;
    distinct_ports: number;
    hit_count: number;
    sample_ports: string;
}

const Field: React.FC<{ label: string; value: React.ReactNode }> = ({ label, value }) => (
    <div>
        <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">{label}</div>
        <div className="text-sm text-gray-900 dark:text-gray-100">{value}</div>
    </div>
);

// ScanDetail is a standalone page (own route, no SecurityPage tab
// shell) — same reasoning as AttackDetail.tsx: once you've drilled
// into one specific episode, you've left the tab-switching UI. See
// plan/port-scan-detection/plan.md Phase B.
export const ScanDetail: React.FC = () => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Security', href: '/admin/security/scans' }, { label: 'Port scan' }]);
    const { id } = useParams<{ id: string }>();
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [scan, setScan] = useState<ScanDetailResponse | null>(null);
    const [loading, setLoading] = useState(true);

    const load = useCallback(async () => {
        if (!id) return;
        setLoading(true);
        try {
            const resp = await get<{ message: ScanDetailResponse }>(`admin/security/scans/{id:${id}}`);
            setScan(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load scan episode.');
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
                    to="/admin/security/scans"
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    ← Back to port scans
                </Link>
            </div>

            <Title>Port-scan episode</Title>

            {loading && !scan ? (
                <div className="mt-6 text-sm text-gray-500">Loading...</div>
            ) : !scan ? (
                <div className="mt-6 text-sm text-red-500">Scan episode not found.</div>
            ) : (
                <div className="mt-6 space-y-6">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <Field label="IP address" value={<span className="font-mono text-sm">{scan.ip}</span>} />
                        <Field label="Status" value={scan.ended_at ? 'Closed' : 'Active'} />
                        <Field label="Distinct ports" value={scan.distinct_ports} />
                        <Field label="Hits" value={scan.hit_count} />
                        <Field label="Started" value={formatDate(scan.started_at)} />
                        <Field label="Last seen" value={formatDate(scan.last_seen_at)} />
                        {scan.ended_at && <Field label="Ended" value={formatDate(scan.ended_at)} />}
                    </div>

                    <div>
                        <div className="text-xs uppercase tracking-wide text-gray-500 mb-2">Sample ports</div>
                        <div className="text-sm font-mono text-gray-900 dark:text-gray-100 break-all">
                            {scan.sample_ports || '—'}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default ScanDetail;
