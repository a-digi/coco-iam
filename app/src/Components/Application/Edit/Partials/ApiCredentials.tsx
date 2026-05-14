import React, { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import { ApplicationResource } from '../../model/application';
import { CreateApiCredentialModal, type CreatedCredential } from './CreateApiCredentialModal';

interface Props {
    applicationId: string;
}

export interface ApiCredential {
    id: string;
    api_id: string;
    label: string;
    purposes: string[];
    expires_at: string;
    is_active: boolean;
    last_used_at?: string | null;
    created_at: string;
    revoked_at?: string | null;
}

interface ListResponse {
    message?: { credentials: ApiCredential[] };
}

const formatDateTime = (iso?: string | null): string => {
    if (!iso) return '—';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString();
};

/**
 * isCredentialRevoked distinguishes a revoked credential from a
 * never-revoked one. Split out so tests could pin it, and so the
 * table row can decide its colour class in one place.
 */
const isCredentialRevoked = (c: ApiCredential): boolean =>
    !c.is_active || Boolean(c.revoked_at);

/**
 * isCredentialExpired returns true when a (still-active) credential
 * has passed its expires_at. A revoked credential is rendered under
 * "revoked" regardless.
 */
const isCredentialExpired = (c: ApiCredential, now = new Date()): boolean => {
    if (!c.expires_at) return false;
    const exp = new Date(c.expires_at);
    if (Number.isNaN(exp.getTime())) return false;
    return exp.getTime() <= now.getTime();
};

const statusLabel = (c: ApiCredential): { label: string; cls: string } => {
    if (isCredentialRevoked(c)) return { label: 'Revoked', cls: 'text-gray-500 bg-gray-100 dark:bg-surface-800' };
    if (isCredentialExpired(c)) return { label: 'Expired', cls: 'text-amber-700 bg-amber-50 dark:text-amber-300 dark:bg-amber-900/30' };
    return { label: 'Active', cls: 'text-emerald-700 bg-emerald-50 dark:text-emerald-300 dark:bg-emerald-900/30' };
};

export const ApiCredentials: React.FC<Props> = ({ applicationId }) => {
    const { get, post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [loading, setLoading] = useState(true);
    const [creds, setCreds] = useState<ApiCredential[]>([]);
    const [createOpen, setCreateOpen] = useState(false);
    const [createdOnce, setCreatedOnce] = useState<CreatedCredential | null>(null);
    const [revokeTarget, setRevokeTarget] = useState<ApiCredential | null>(null);
    const [revoking, setRevoking] = useState(false);

    const fetchList = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<ListResponse>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/api-credentials`,
            );
            setCreds(resp?.message?.credentials ?? []);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load credentials');
        } finally {
            setLoading(false);
        }
    }, [get, applicationId, errorMessage]);

    useEffect(() => {
        void fetchList();
    }, [fetchList]);

    const handleCreated = (created: CreatedCredential) => {
        setCreatedOnce(created);
        setCreateOpen(false);
        void fetchList();
    };

    const confirmRevoke = async () => {
        if (!revokeTarget) return;
        setRevoking(true);
        try {
            await post(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/api-credentials/${revokeTarget.id}/revoke`,
                {},
            );
            successMessage('Credential revoked.');
            setRevokeTarget(null);
            void fetchList();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to revoke credential');
        } finally {
            setRevoking(false);
        }
    };

    return (
        <div className="space-y-6 mt-2">
            <div className="rounded-lg border border-amber-200 dark:border-amber-900/40 bg-amber-50 dark:bg-amber-900/20 px-4 py-3 text-sm text-amber-800 dark:text-amber-200">
                <div className="font-semibold mb-1">Machine-auth credentials</div>
                An API credential lets an external service authenticate against the public
                <code className="mx-1 text-[0.8rem] font-mono">/a/&lt;org&gt;/&lt;ws&gt;/&lt;app&gt;/…</code> endpoints
                using HTTP Basic with the api id + api secret. The secret is only shown once — at creation time.
            </div>

            <div className="flex items-center justify-between">
                <div className="text-sm text-gray-600 dark:text-gray-300">
                    {loading ? 'Loading…' : `${creds.length} credential${creds.length === 1 ? '' : 's'}`}
                </div>
                <button
                    type="button"
                    onClick={() => setCreateOpen(true)}
                    className="inline-flex items-center gap-2 px-3 py-2 rounded-md bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-500"
                >
                    + Create credential
                </button>
            </div>

            <div className="overflow-x-auto rounded-md border border-gray-200 dark:border-surface-800">
                <table className="min-w-full text-sm">
                    <thead className="bg-gray-50 dark:bg-surface-900/50 text-gray-600 dark:text-gray-300">
                        <tr>
                            <th className="text-left px-3 py-2 font-medium">Label</th>
                            <th className="text-left px-3 py-2 font-medium">API ID</th>
                            <th className="text-left px-3 py-2 font-medium">Purposes</th>
                            <th className="text-left px-3 py-2 font-medium">Expires</th>
                            <th className="text-left px-3 py-2 font-medium">Last used</th>
                            <th className="text-left px-3 py-2 font-medium">Status</th>
                            <th className="text-right px-3 py-2 font-medium">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {creds.length === 0 && !loading && (
                            <tr>
                                <td colSpan={7} className="px-3 py-6 text-center text-gray-500">
                                    No credentials yet. Click <strong>Create credential</strong> to issue the first one.
                                </td>
                            </tr>
                        )}
                        {creds.map(c => {
                            const status = statusLabel(c);
                            return (
                                <tr key={c.id} className="border-t border-gray-200 dark:border-surface-800">
                                    <td className="px-3 py-2">{c.label || <span className="text-gray-400 italic">—</span>}</td>
                                    <td className="px-3 py-2 font-mono text-[0.8rem]">{c.api_id}</td>
                                    <td className="px-3 py-2 font-mono text-[0.75rem]">{c.purposes.join(', ') || '—'}</td>
                                    <td className="px-3 py-2">{formatDateTime(c.expires_at)}</td>
                                    <td className="px-3 py-2">{formatDateTime(c.last_used_at)}</td>
                                    <td className="px-3 py-2">
                                        <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${status.cls}`}>
                                            {status.label}
                                        </span>
                                    </td>
                                    <td className="px-3 py-2 text-right">
                                        {!isCredentialRevoked(c) && (
                                            <button
                                                type="button"
                                                onClick={() => setRevokeTarget(c)}
                                                className="text-red-600 hover:underline text-sm"
                                            >
                                                Revoke
                                            </button>
                                        )}
                                    </td>
                                </tr>
                            );
                        })}
                    </tbody>
                </table>
            </div>

            <CreateApiCredentialModal
                isOpen={createOpen}
                onClose={() => setCreateOpen(false)}
                applicationId={applicationId}
                onCreated={handleCreated}
            />

            {createdOnce && (
                <CreatedSecretModal
                    credential={createdOnce}
                    onClose={() => setCreatedOnce(null)}
                />
            )}

            <ConfirmModal
                isOpen={revokeTarget !== null}
                onClose={() => (revoking ? undefined : setRevokeTarget(null))}
                onConfirm={confirmRevoke}
                title="Revoke credential?"
                message={
                    <>
                        <p className="mb-2">
                            The credential <strong>{revokeTarget?.label || revokeTarget?.api_id}</strong> will stop working immediately.
                        </p>
                        <p className="text-xs text-gray-500">
                            The row is kept for audit. To undo, issue a new credential.
                        </p>
                    </>
                }
                confirmLabel="Revoke"
                cancelLabel="Keep active"
                variant="danger"
                isLoading={revoking}
            />
        </div>
    );
};

// CreatedSecretModal is the "copy this once" screen shown right after
// a successful create. The plaintext secret is nowhere else — the
// admin must copy it before dismissing.
const CreatedSecretModal: React.FC<{
    credential: CreatedCredential;
    onClose: () => void;
}> = ({ credential, onClose }) => {
    const { successMessage, errorMessage } = useSnackBar();
    const copy = (value: string, label: string) => {
        void navigator.clipboard
            .writeText(value)
            .then(() => successMessage(`${label} copied.`))
            .catch(() => errorMessage(`Could not copy ${label}.`));
    };

    return (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" role="dialog" aria-modal="true">
            <div className="relative w-full sm:max-w-xl bg-white dark:bg-surface-800 rounded-xl shadow-2xl border border-gray-200 dark:border-surface-700">
                <div className="px-6 py-4 border-b border-gray-200 dark:border-surface-700">
                    <h3 className="text-lg font-bold text-gray-900 dark:text-gray-100">Copy your credential now</h3>
                </div>
                <div className="px-6 py-4 space-y-4 text-sm text-gray-700 dark:text-gray-300">
                    <div className="rounded-md border border-amber-200 dark:border-amber-900/40 bg-amber-50 dark:bg-amber-900/20 px-3 py-2 text-amber-800 dark:text-amber-200">
                        This is the only time the <strong>API Secret</strong> will be shown. Save it now in a secret manager — you will not be able to retrieve it later.
                    </div>
                    <div>
                        <div className="text-xs font-semibold uppercase tracking-wide text-gray-500 mb-1">API ID</div>
                        <div className="flex items-center gap-2">
                            <code className="flex-1 px-2 py-1 rounded bg-gray-50 dark:bg-surface-900 font-mono text-[0.8rem] break-all">{credential.credential.api_id}</code>
                            <button
                                type="button"
                                onClick={() => copy(credential.credential.api_id, 'API ID')}
                                className="text-xs text-indigo-600 hover:underline"
                            >
                                Copy
                            </button>
                        </div>
                    </div>
                    <div>
                        <div className="text-xs font-semibold uppercase tracking-wide text-gray-500 mb-1">API Secret</div>
                        <div className="flex items-center gap-2">
                            <code className="flex-1 px-2 py-1 rounded bg-gray-50 dark:bg-surface-900 font-mono text-[0.8rem] break-all">{credential.api_secret}</code>
                            <button
                                type="button"
                                onClick={() => copy(credential.api_secret, 'API Secret')}
                                className="text-xs text-indigo-600 hover:underline"
                            >
                                Copy
                            </button>
                        </div>
                    </div>
                    {credential.clamped && (
                        <div className="text-xs text-amber-700 dark:text-amber-300">
                            The requested expiry exceeded the maximum of 1 year and was reduced to {formatDateTime(credential.credential.expires_at)}.
                        </div>
                    )}
                </div>
                <div className="px-6 py-4 border-t border-gray-200 dark:border-surface-700 bg-gray-50 dark:bg-surface-900 rounded-b-xl flex justify-end">
                    <button
                        type="button"
                        onClick={onClose}
                        className="px-3 py-2 rounded-md bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-500"
                    >
                        I've saved the secret
                    </button>
                </div>
            </div>
        </div>
    );
};

export default ApiCredentials;
