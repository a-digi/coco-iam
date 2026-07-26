import React, { useCallback, useEffect, useState } from 'react';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import { Add, Remove } from '../../../../Shared/Components/Button';
import ConfirmModal from '../../../../Shared/Components/Modal/ConfirmModal';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import BanIPModal from './BanIPModal';

interface IPBan {
    ip: string;
    tier: string;
    reason: string;
    banned_at: string;
    expires_at: string;
    hit_count: number;
    created_by?: string;
}

const WRITE_SCOPES = [AppScopes.AdminSecurityIpBansWrite, AppScopes.SuperAdmin];

// BansDashboard is unpaginated by design — IPBanListHandler returns
// every currently-tracked ban, bounded by whatever's actually active,
// not a growing history (see plan/ip-abuse-protection/plan.md
// section 3's "no request-log table" note).
export const BansDashboard: React.FC = () => {
    const { get, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [bans, setBans] = useState<IPBan[]>([]);
    const [loading, setLoading] = useState(true);
    const [showBanModal, setShowBanModal] = useState(false);
    const [unbanTarget, setUnbanTarget] = useState<string | null>(null);
    const [unbanning, setUnbanning] = useState(false);

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<{ message: IPBan[] }>('admin/security/ip-bans');
            setBans(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load bans.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void load();
    }, [load]);

    const confirmUnban = async () => {
        if (!unbanTarget) return;
        setUnbanning(true);
        try {
            await del(`admin/security/ip-bans/{ip:${unbanTarget}}`);
            successMessage(`${unbanTarget} unbanned.`);
            setUnbanTarget(null);
            await load();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to unban IP.');
        } finally {
            setUnbanning(false);
        }
    };

    const columns: TableColumn<IPBan>[] = [
        { key: 'ip', label: 'IP address' },
        { key: 'tier', label: 'Tier' },
        { key: 'reason', label: 'Reason' },
        { key: 'hit_count', label: 'Hits' },
        {
            key: 'expires_at',
            label: 'Expires',
            render: value => new Date(String(value)).toLocaleString(),
        },
        {
            key: 'actions',
            label: '',
            render: (_value, row) => (
                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <Remove label="Unban" onClick={() => setUnbanTarget(row.ip)} />
                </ScopeBasedComponentAccess>
            ),
        },
    ];

    return (
        <div>
            <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100">Banned IPs</h2>
                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <Add label="Ban IP" onClick={() => setShowBanModal(true)} />
                </ScopeBasedComponentAccess>
            </div>

            {loading ? (
                <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>
            ) : (
                <Table columns={columns} data={bans} rowKey={row => row.ip} emptyText="No IPs are currently banned." />
            )}

            {showBanModal && (
                <BanIPModal
                    isOpen={showBanModal}
                    onClose={() => setShowBanModal(false)}
                    onCreated={() => {
                        setShowBanModal(false);
                        void load();
                    }}
                />
            )}

            {unbanTarget && (
                <ConfirmModal
                    title="Unban IP"
                    message={`Unban ${unbanTarget}? It will immediately be allowed through again.`}
                    confirmLabel="Unban"
                    variant="danger"
                    isLoading={unbanning}
                    onConfirm={confirmUnban}
                    onCancel={() => setUnbanTarget(null)}
                />
            )}
        </div>
    );
};

export default BansDashboard;
