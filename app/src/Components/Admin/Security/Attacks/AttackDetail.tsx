import React, { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import Title from '../../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';
import { formatDate } from '../../../../config/data/date/date';

interface AttackTarget {
    path: string;
    method: string;
    hit_count: number;
}

interface AttackDetailResponse {
    id: string;
    ip: string;
    tier: string;
    started_at: string;
    last_seen_at: string;
    ended_at?: string;
    hit_count: number;
    ban_count: number;
    targets: AttackTarget[];
}

const Field: React.FC<{ label: string; value: React.ReactNode }> = ({ label, value }) => (
    <div>
        <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">{label}</div>
        <div className="text-sm text-gray-900 dark:text-gray-100">{value}</div>
    </div>
);

// AttackDetail is a standalone page (own route, no SecurityPage tab
// shell) — mirrors Queue/Task/TaskDetail.tsx's master-detail pattern
// rather than nesting inside the Bans/Allowlist/Attacks tab bar, which
// would misleadingly imply you're still switching tabs once you've
// drilled into one specific episode.
export const AttackDetail: React.FC = () => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Security', href: '/admin/security/attacks' }, { label: 'Attack' }]);
    const { id } = useParams<{ id: string }>();
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [attack, setAttack] = useState<AttackDetailResponse | null>(null);
    const [loading, setLoading] = useState(true);

    const load = useCallback(async () => {
        if (!id) return;
        setLoading(true);
        try {
            const resp = await get<{ message: AttackDetailResponse }>(`admin/security/attacks/{id:${id}}`);
            setAttack(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load attack episode.');
        } finally {
            setLoading(false);
        }
    }, [id, get, errorMessage]);

    useEffect(() => {
        void load();
    }, [load]);

    const columns: TableColumn<AttackTarget>[] = [
        { key: 'path', label: 'Path' },
        { key: 'method', label: 'Method' },
        { key: 'hit_count', label: 'Hits' },
    ];

    return (
        <div>
            <div className="mb-4">
                <Link
                    to="/admin/security/attacks"
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    ← Back to attacks
                </Link>
            </div>

            <Title>Attack episode</Title>

            {loading && !attack ? (
                <div className="mt-6 text-sm text-gray-500">Loading...</div>
            ) : !attack ? (
                <div className="mt-6 text-sm text-red-500">Attack episode not found.</div>
            ) : (
                <div className="mt-6 space-y-6">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <Field label="IP address" value={<span className="font-mono text-sm">{attack.ip}</span>} />
                        <Field label="Tier" value={attack.tier} />
                        <Field label="Status" value={attack.ended_at ? 'Closed' : 'Active'} />
                        <Field label="Hits" value={attack.hit_count} />
                        <Field label="Ban triggers" value={attack.ban_count} />
                        <Field label="Started" value={formatDate(attack.started_at)} />
                        <Field label="Last seen" value={formatDate(attack.last_seen_at)} />
                        {attack.ended_at && <Field label="Ended" value={formatDate(attack.ended_at)} />}
                    </div>

                    <div>
                        <div className="text-xs uppercase tracking-wide text-gray-500 mb-2">Targeted endpoints</div>
                        <Table
                            columns={columns}
                            data={attack.targets}
                            rowKey={(row, index) => `${row.method} ${row.path} ${index}`}
                            emptyText="No endpoint breakdown recorded."
                        />
                    </div>
                </div>
            )}
        </div>
    );
};

export default AttackDetail;
