import React, { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import Title from '../../../../Shared/Components/Font/Title';
import { AccordionItem } from '../../../../Shared/Components/Accordion';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';
import { formatDate } from '../../../../config/data/date/date';
import { parseGeoIPInfo, formatGeoIPCountry, formatGeoIPOrg } from '../geoipInfo';

interface AttackTarget {
    path: string;
    method: string;
    hit_count: number;
    body_sample?: string;
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
    origin_hint?: string;
    geoip_info?: string;
}

const Field: React.FC<{ label: string; value: React.ReactNode }> = ({ label, value }) => (
    <div>
        <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">{label}</div>
        <div className="text-sm text-gray-900 dark:text-gray-100">{value}</div>
    </div>
);

// origin_hint is stored as a compact JSON string (see
// plan/attack-ip-attribution/plan.md Fix 3) — pretty-print it for
// readability, falling back to the raw string if it's ever not valid
// JSON (defensive only; the backend always writes valid JSON here).
const formatOriginHint = (raw: string): string => {
    try {
        return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
        return raw;
    }
};

// AttackDetail is a standalone page (own route, no SecurityPage tab
// shell) — mirrors Queue/Task/TaskDetail.tsx's master-detail pattern
// rather than nesting inside the Bans/Allowlist/Attacks tab bar, which
// would misleadingly imply you're still switching tabs once you've
// drilled into one specific episode.
//
// Reused for browsing a rotated-out archive too (see
// plan/ip-attacks-db-archiving/plan.md) — when this component is
// mounted on the /admin/security/archives/:archiveId/attacks/:id route
// instead of /admin/security/attacks/:id, archiveId is present and
// every fetch/link below reads from that archive instead of the live
// data. Purely additive: archiveId is always undefined on the live
// route, so the live page's behavior is unchanged.
export const AttackDetail: React.FC = () => {
    const { id, archiveId } = useParams<{ id: string; archiveId?: string }>();

    useBreadcrumbItems(
        archiveId
            ? [
                  { label: 'Admin' },
                  { label: 'Security', href: '/admin/security/archives' },
                  { label: 'Archive', href: `/admin/security/archives/${archiveId}` },
                  { label: 'Attack' },
              ]
            : [{ label: 'Admin' }, { label: 'Security', href: '/admin/security/attacks' }, { label: 'Attack' }]
    );
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [attack, setAttack] = useState<AttackDetailResponse | null>(null);
    const [loading, setLoading] = useState(true);

    const load = useCallback(async () => {
        if (!id) return;
        setLoading(true);
        try {
            const path = archiveId
                ? `admin/security/archives/{id:${archiveId}}/attacks/{attackId:${id}}`
                : `admin/security/attacks/{id:${id}}`;
            const resp = await get<{ message: AttackDetailResponse }>(path);
            setAttack(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load attack episode.');
        } finally {
            setLoading(false);
        }
    }, [id, archiveId, get, errorMessage]);

    useEffect(() => {
        void load();
    }, [load]);

    const columns: TableColumn<AttackTarget>[] = [
        { key: 'path', label: 'Path' },
        { key: 'method', label: 'Method' },
        { key: 'hit_count', label: 'Hits' },
        {
            key: 'body_sample',
            label: 'Body sample',
            render: value =>
                value ? (
                    <div className="max-w-xs whitespace-pre-wrap break-all font-mono text-xs text-gray-600 dark:text-gray-400">
                        {value as string}
                    </div>
                ) : (
                    <span className="text-gray-400 dark:text-gray-600">—</span>
                ),
        },
    ];

    const backTo = archiveId ? `/admin/security/archives/${archiveId}/attacks` : '/admin/security/attacks';

    const geo = attack ? parseGeoIPInfo(attack.geoip_info) : null;
    const geoCountry = geo ? formatGeoIPCountry(geo) : null;
    const geoOrg = geo ? formatGeoIPOrg(geo) : null;
    const hasGeoData = !!(geoCountry || geoOrg);

    // Collapsed by default when there's data (it's supplementary to the
    // fields above); left open when there's none, so the "no data"
    // case is visible without an extra click. `geoOpenOverride` only
    // kicks in once the admin actually toggles it themselves.
    const [geoOpenOverride, setGeoOpenOverride] = useState<boolean | null>(null);
    const geoOpen = geoOpenOverride ?? !hasGeoData;

    return (
        <div>
            <div className="mb-4">
                <Link
                    to={backTo}
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    ← Back to {archiveId ? 'archived attacks' : 'attacks'}
                </Link>
            </div>

            <Title>Attack episode{archiveId ? ' (archived)' : ''}</Title>

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

                    <AccordionItem
                        title="GeoIP info"
                        isOpen={geoOpen}
                        onToggle={() => setGeoOpenOverride(!geoOpen)}
                        variant="standalone"
                    >
                        {hasGeoData ? (
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {geoCountry && <Field label="Country" value={geoCountry} />}
                                {geoOrg && <Field label="ISP / ASN" value={geoOrg} />}
                            </div>
                        ) : (
                            <p className="text-sm text-gray-500 dark:text-gray-400">
                                No GeoIP data available for this IP.
                            </p>
                        )}
                    </AccordionItem>

                    {attack.origin_hint && (
                        <div>
                            <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Origin hint</div>
                            <div className="text-xs text-gray-500 dark:text-gray-400 mb-2">
                                The IP above is loopback/private — no configured header resolved to a real
                                address. These are the request headers that were present, captured for manual
                                correlation against proxy logs.
                            </div>
                            <pre className="rounded-xl bg-gray-50 dark:bg-surface-900 ring-1 ring-gray-200/70 dark:ring-surface-700/80 p-3 text-xs font-mono text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-all">
                                {formatOriginHint(attack.origin_hint)}
                            </pre>
                        </div>
                    )}

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
