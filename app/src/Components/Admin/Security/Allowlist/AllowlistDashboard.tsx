import React, { useCallback, useEffect, useState } from 'react';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import { Add, Remove } from '../../../../Shared/Components/Button';
import ConfirmModal from '../../../../Shared/Components/Modal/ConfirmModal';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import AddAllowlistEntryModal from './AddAllowlistEntryModal';

interface IPAllowlistEntry {
    ip: string;
    note?: string;
    created_at: string;
    created_by: string;
}

const WRITE_SCOPES = [AppScopes.AdminSecurityIpAllowlistWrite, AppScopes.SuperAdmin];

// AllowlistDashboard mirrors BansDashboard.tsx's shape — same
// unpaginated, admin-curated list pattern.
export const AllowlistDashboard: React.FC = () => {
    const { get, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [entries, setEntries] = useState<IPAllowlistEntry[]>([]);
    const [loading, setLoading] = useState(true);
    const [showAddModal, setShowAddModal] = useState(false);
    const [removeTarget, setRemoveTarget] = useState<string | null>(null);
    const [removing, setRemoving] = useState(false);

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<{ message: IPAllowlistEntry[] }>('admin/security/ip-allowlist');
            setEntries(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load the allowlist.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void load();
    }, [load]);

    const confirmRemove = async () => {
        if (!removeTarget) return;
        setRemoving(true);
        try {
            await del(`admin/security/ip-allowlist/{ip:${removeTarget}}`);
            successMessage(`${removeTarget} removed from the allowlist.`);
            setRemoveTarget(null);
            await load();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to remove IP from the allowlist.');
        } finally {
            setRemoving(false);
        }
    };

    const columns: TableColumn<IPAllowlistEntry>[] = [
        { key: 'ip', label: 'IP address' },
        { key: 'note', label: 'Note' },
        { key: 'created_by', label: 'Added by' },
        {
            key: 'created_at',
            label: 'Added',
            render: value => new Date(String(value)).toLocaleString(),
        },
        {
            key: 'actions',
            label: '',
            render: (_value, row) => (
                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <Remove onClick={() => setRemoveTarget(row.ip)} />
                </ScopeBasedComponentAccess>
            ),
        },
    ];

    return (
        <div>
            <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100">IP Allowlist</h2>
                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <Add label="Add to allowlist" onClick={() => setShowAddModal(true)} />
                </ScopeBasedComponentAccess>
            </div>

            {loading ? (
                <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>
            ) : (
                <Table columns={columns} data={entries} rowKey={row => row.ip} emptyText="The allowlist is empty." />
            )}

            {showAddModal && (
                <AddAllowlistEntryModal
                    isOpen={showAddModal}
                    onClose={() => setShowAddModal(false)}
                    onCreated={() => {
                        setShowAddModal(false);
                        void load();
                    }}
                />
            )}

            {removeTarget && (
                <ConfirmModal
                    title="Remove from allowlist"
                    message={`Remove ${removeTarget} from the allowlist? It will become subject to rate limiting and bans again.`}
                    confirmLabel="Remove"
                    variant="danger"
                    isLoading={removing}
                    onConfirm={confirmRemove}
                    onCancel={() => setRemoveTarget(null)}
                />
            )}
        </div>
    );
};

export default AllowlistDashboard;
