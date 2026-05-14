import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Modal, ConfirmModal } from '../../../../Shared/Components/Modal';
import { AppScopes } from '../../../../config/security/scopes';
import { API_BASE_URL } from '../../../../api/client';
import { ApplicationResource } from '../../model/application';

interface Props {
    applicationId: string;
}

type KeyStatus = 'active' | 'pending' | 'deactivated';

interface Keypair {
    id: string;
    application_id: string;
    status: KeyStatus;
    public_pem: string;
    private_pem?: string;
    has_private: boolean;
    created_at?: string;
    activated_at?: string | null;
    deactivated_at?: string | null;
    expires_at?: string | null;
}

interface ListResponse {
    message?: { keys: Keypair[] };
}

interface PairResponse {
    message?: Keypair;
}

const formatDateTime = (iso?: string | null): string => {
    if (!iso) return '—';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString();
};

const copy = (value: string, success: (s: string) => void, fail: (s: string) => void, label: string) => {
    void navigator.clipboard
        .writeText(value)
        .then(() => success(`${label} copied.`))
        .catch(() => fail(`Could not copy ${label}.`));
};

export const Keys: React.FC<Props> = ({ applicationId }) => {
    const { get, post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState(false);
    const [keys, setKeys] = useState<Keypair[]>([]);
    const [showPrivateFor, setShowPrivateFor] = useState<string | null>(null);
    const [pendingPreview, setPendingPreview] = useState<Keypair | null>(null);
    const [acceptOpen, setAcceptOpen] = useState(false);
    const [discardConfirmOpen, setDiscardConfirmOpen] = useState(false);
    const [forceExpireKey, setForceExpireKey] = useState<Keypair | null>(null);

    const jwksUrl = `${API_BASE_URL}public/applications/${applicationId}/.well-known/jwks.json`;

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<ListResponse['message']>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/keys`,
            ) as ListResponse;
            setKeys(resp?.message?.keys ?? []);
        } catch (err: unknown) {
            let msg = 'Failed to load keys';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [applicationId, get, errorMessage]);

    useEffect(() => { void load(); }, [load]);

    const { active, pending, deactivated } = useMemo(() => {
        const a = keys.find(k => k.status === 'active') ?? null;
        const p = keys.find(k => k.status === 'pending') ?? null;
        const d = keys.filter(k => k.status === 'deactivated')
            .sort((x, y) => (y.deactivated_at ?? '').localeCompare(x.deactivated_at ?? ''));
        return { active: a, pending: p, deactivated: d };
    }, [keys]);

    const handleRegenerate = async () => {
        setBusy(true);
        try {
            const resp = await post<PairResponse['message']>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/keys/regenerate`,
                {},
            ) as PairResponse;
            if (resp?.message) setPendingPreview(resp.message);
            successMessage('New keypair generated. Review it, then accept or discard.');
            await load();
        } catch (err: unknown) {
            let msg = 'Failed to generate keypair';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setBusy(false);
        }
    };

    const handleAccept = async () => {
        setAcceptOpen(false);
        setBusy(true);
        try {
            await post<null>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/keys/activate-pending`,
                {},
            );
            successMessage('Keypair rotated. The previous key remains valid for 24 hours.');
            setPendingPreview(null);
            await load();
        } catch (err: unknown) {
            let msg = 'Failed to rotate keys';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setBusy(false);
        }
    };

    const handleDiscard = async () => {
        setDiscardConfirmOpen(false);
        setBusy(true);
        try {
            await post<null>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/keys/discard-pending`,
                {},
            );
            successMessage('Pending key discarded.');
            setPendingPreview(null);
            await load();
        } catch (err: unknown) {
            let msg = 'Failed to discard pending key';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setBusy(false);
        }
    };

    const handleForceExpire = async () => {
        if (!forceExpireKey) return;
        const target = forceExpireKey;
        setForceExpireKey(null);
        setBusy(true);
        try {
            await post<null>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/keys/${target.id}/deactivate`,
                {},
            );
            successMessage('Key fully deactivated. Any tokens it signed will now be rejected.');
            await load();
        } catch (err: unknown) {
            let msg = 'Failed to deactivate key';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setBusy(false);
        }
    };

    if (loading) return <div className="text-sm text-gray-500 py-2">Loading keys…</div>;

    return (
        <div className="space-y-8 mt-4">
            <section>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-1">JWKS endpoint</h3>
                <p className="text-sm text-gray-500 mb-2">
                    Public URL your services fetch to verify tokens. Returns every key that still validates — the
                    active key plus any deactivated keys still in their 24-hour grace window.
                </p>
                <div className="flex items-center gap-2">
                    <code className="flex-1 text-xs font-mono p-2 rounded-md border border-gray-300 dark:border-surface-700 bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 break-all">
                        {jwksUrl}
                    </code>
                    <button
                        type="button"
                        onClick={() => copy(jwksUrl, successMessage, errorMessage, 'JWKS URL')}
                        className="px-3 py-1.5 text-xs rounded-md bg-indigo-600 hover:bg-indigo-500 text-white flex-shrink-0"
                    >
                        Copy URL
                    </button>
                </div>
            </section>

            {active && (
                <KeyCard
                    label="Active key"
                    description="Signs every newly-issued token. There is always exactly one active key per application."
                    accent="indigo"
                    keypair={active}
                    showPrivate={showPrivateFor === active.id}
                    onToggleShowPrivate={() => setShowPrivateFor(showPrivateFor === active.id ? null : active.id)}
                    onCopyPublic={() => copy(active.public_pem, successMessage, errorMessage, 'Public key')}
                    onCopyPrivate={active.private_pem ? () => copy(active.private_pem ?? '', successMessage, errorMessage, 'Private key') : undefined}
                />
            )}

            {pending && (
                <section className="rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 p-4 space-y-4">
                    <div>
                        <h3 className="text-lg font-semibold text-amber-900 dark:text-amber-100">Pending key</h3>
                        <p className="text-sm text-amber-800 dark:text-amber-200 mt-1">
                            A new keypair has been generated but is <strong>not</strong> signing tokens yet. Accepting
                            replaces the active key; the current active key then becomes valid for verification only,
                            for 24 hours.
                        </p>
                    </div>
                    <div className="rounded-md border border-amber-300 dark:border-amber-700 bg-white/60 dark:bg-surface-900/60 p-3 text-sm text-amber-900 dark:text-amber-100">
                        <strong>Before you accept:</strong> every downstream service must refresh its JWKS cache.
                        Services still caching only the old key will reject users signed in with the new one, and
                        once the 24-hour grace expires on the old key, logins for those users will fail entirely.
                    </div>
                    <KeyCard
                        label=""
                        description=""
                        accent="amber"
                        inline
                        keypair={pending}
                        showPrivate={showPrivateFor === pending.id}
                        onToggleShowPrivate={() => setShowPrivateFor(showPrivateFor === pending.id ? null : pending.id)}
                        onCopyPublic={() => copy(pending.public_pem, successMessage, errorMessage, 'Public key')}
                        onCopyPrivate={pending.private_pem ? () => copy(pending.private_pem ?? '', successMessage, errorMessage, 'Private key') : undefined}
                    />
                    <div className="flex items-center gap-3 pt-1">
                        <button
                            type="button"
                            disabled={busy}
                            onClick={() => setAcceptOpen(true)}
                            className="px-3 py-1.5 text-sm rounded-md bg-amber-600 hover:bg-amber-500 text-white disabled:opacity-40"
                        >
                            Accept and rotate
                        </button>
                        <button
                            type="button"
                            disabled={busy}
                            onClick={() => setDiscardConfirmOpen(true)}
                            className="px-3 py-1.5 text-sm rounded-md border border-amber-300 dark:border-amber-700 text-amber-900 dark:text-amber-100 hover:bg-amber-100 dark:hover:bg-amber-900/40 disabled:opacity-40"
                        >
                            Discard
                        </button>
                    </div>
                </section>
            )}

            <section>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-1">Generate a new key</h3>
                <p className="text-sm text-gray-500 mb-3">
                    Creates a pending key alongside the active one. You can review its PEM, then accept the rotation
                    or discard. Only one pending key can exist at a time.
                </p>
                <button
                    type="button"
                    disabled={busy || !!pending}
                    onClick={() => void handleRegenerate()}
                    title={pending ? 'Discard the existing pending key first.' : undefined}
                    className="px-3 py-1.5 text-sm rounded-md bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-40 disabled:cursor-not-allowed"
                >
                    Generate new keypair
                </button>
            </section>

            {deactivated.length > 0 && (
                <section>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-1">Deactivated keys</h3>
                    <p className="text-sm text-gray-500 mb-3">
                        No longer signing, but still verify tokens until their grace window closes. "Deactivate now"
                        forces the expiry to immediately — tokens signed by that key will then be rejected.
                    </p>
                    <div className="space-y-3">
                        {deactivated.map(k => (
                            <div key={k.id} className="rounded-md border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900 p-3">
                                <div className="flex items-start justify-between gap-3 flex-wrap">
                                    <div className="min-w-0">
                                        <div className="text-xs text-gray-500">kid</div>
                                        <code className="block text-xs font-mono break-all text-gray-900 dark:text-gray-100">{k.id}</code>
                                    </div>
                                    <div className="text-xs text-gray-500 text-right">
                                        <div>Deactivated: {formatDateTime(k.deactivated_at)}</div>
                                        <div>Expires: {formatDateTime(k.expires_at)}</div>
                                    </div>
                                    <button
                                        type="button"
                                        disabled={busy}
                                        onClick={() => setForceExpireKey(k)}
                                        className="px-2 py-1 text-xs rounded-md border border-red-300 dark:border-red-800 text-red-700 dark:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20"
                                    >
                                        Deactivate now
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>
                </section>
            )}

            {/* Accept confirmation — also visible as a modal so the
                warning lands before the irreversible click. */}
            <Modal
                isOpen={acceptOpen}
                onClose={() => setAcceptOpen(false)}
                title="Rotate signing key?"
            >
                <div className="space-y-4 text-sm text-gray-700 dark:text-gray-300">
                    <p>
                        The pending key will start signing tokens immediately. The current active key will
                        become <strong>deactivated</strong>, verifying-only for the next 24 hours.
                    </p>
                    <div className="rounded-md border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-3 text-red-800 dark:text-red-200">
                        Update every downstream service to refresh its JWKS cache before this window closes.
                        Services that don't will reject users once the grace expires.
                    </div>
                    <div className="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            onClick={() => setAcceptOpen(false)}
                            className="px-3 py-1.5 text-sm rounded-md border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800"
                        >
                            Cancel
                        </button>
                        <button
                            type="button"
                            onClick={() => void handleAccept()}
                            className="px-3 py-1.5 text-sm rounded-md bg-amber-600 hover:bg-amber-500 text-white"
                        >
                            Yes, rotate now
                        </button>
                    </div>
                </div>
            </Modal>

            <ConfirmModal
                isOpen={discardConfirmOpen}
                onClose={() => setDiscardConfirmOpen(false)}
                onConfirm={() => void handleDiscard()}
                title="Discard pending key?"
                message="The generated keypair will be deleted. You can always generate a new one."
                confirmLabel="Discard"
                variant="danger"
            />

            <ConfirmModal
                isOpen={!!forceExpireKey}
                onClose={() => setForceExpireKey(null)}
                onConfirm={() => void handleForceExpire()}
                title="Deactivate this key immediately?"
                message="Any tokens still signed by this key will be rejected. This cannot be undone."
                confirmLabel="Deactivate now"
                variant="danger"
            />

            {pendingPreview && (
                <div className="hidden">{/* reserved: future audit trail */}</div>
            )}
        </div>
    );
};

// -- KeyCard -----------------------------------------------------------

interface KeyCardProps {
    label: string;
    description: string;
    accent: 'indigo' | 'amber';
    inline?: boolean;
    keypair: Keypair;
    showPrivate: boolean;
    onToggleShowPrivate: () => void;
    onCopyPublic: () => void;
    onCopyPrivate?: () => void;
}

const KeyCard: React.FC<KeyCardProps> = ({
    label,
    description,
    keypair,
    showPrivate,
    onToggleShowPrivate,
    onCopyPublic,
    onCopyPrivate,
    inline,
}) => {
    return (
        <section className={inline ? '' : undefined}>
            {label && <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-1">{label}</h3>}
            {description && <p className="text-sm text-gray-500 mb-3">{description}</p>}

            <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs text-gray-600 dark:text-gray-400 mb-2">
                <div>
                    <div className="uppercase tracking-wide text-[10px]">kid</div>
                    <code className="block font-mono break-all text-gray-900 dark:text-gray-100">{keypair.id}</code>
                </div>
                <div>
                    <div className="uppercase tracking-wide text-[10px]">Created</div>
                    <div>{formatDateTime(keypair.created_at)}</div>
                </div>
                <div>
                    <div className="uppercase tracking-wide text-[10px]">
                        {keypair.status === 'active' ? 'Activated' : keypair.status === 'deactivated' ? 'Deactivated' : 'Generated'}
                    </div>
                    <div>
                        {keypair.status === 'active'
                            ? formatDateTime(keypair.activated_at)
                            : keypair.status === 'deactivated'
                                ? formatDateTime(keypair.deactivated_at)
                                : formatDateTime(keypair.created_at)}
                    </div>
                </div>
            </div>

            <div className="space-y-3">
                <div>
                    <div className="text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Public PEM</div>
                    <textarea
                        readOnly
                        value={keypair.public_pem}
                        spellCheck={false}
                        className="w-full h-36 font-mono text-[11px] p-3 rounded-lg border border-gray-300 dark:border-surface-700 bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100"
                    />
                    <div className="mt-1">
                        <button
                            type="button"
                            onClick={onCopyPublic}
                            className="px-3 py-1.5 text-xs rounded-md bg-indigo-600 hover:bg-indigo-500 text-white"
                        >
                            Copy PEM
                        </button>
                    </div>
                </div>

                <div>
                    <div className="text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Private PEM</div>
                    {keypair.has_private && !keypair.private_pem && (
                        <div className="rounded-md border border-gray-200 dark:border-surface-700 bg-gray-50 dark:bg-surface-900 p-3 text-xs text-gray-600 dark:text-gray-300">
                            You don't have permission to view the private key. Ask a super-admin for <code>{AppScopes.ApplicationsKeysReadPrivate}</code>.
                        </div>
                    )}
                    {keypair.private_pem && !showPrivate && (
                        <button
                            type="button"
                            onClick={onToggleShowPrivate}
                            className="px-3 py-1.5 text-xs rounded-md border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800"
                        >
                            Show private key
                        </button>
                    )}
                    {keypair.private_pem && showPrivate && (
                        <>
                            <div className="rounded-md border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-2 text-xs text-red-800 dark:text-red-200 mb-2">
                                <strong>Secret.</strong> Don't paste it into chat, tickets, or source control.
                            </div>
                            <textarea
                                readOnly
                                value={keypair.private_pem}
                                spellCheck={false}
                                className="w-full h-48 font-mono text-[11px] p-3 rounded-lg border border-red-300 dark:border-red-800 bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100"
                            />
                            <div className="mt-1 flex gap-2">
                                {onCopyPrivate && (
                                    <button
                                        type="button"
                                        onClick={onCopyPrivate}
                                        className="px-3 py-1.5 text-xs rounded-md bg-red-600 hover:bg-red-500 text-white"
                                    >
                                        Copy PEM
                                    </button>
                                )}
                                <button
                                    type="button"
                                    onClick={onToggleShowPrivate}
                                    className="px-3 py-1.5 text-xs rounded-md border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800"
                                >
                                    Hide
                                </button>
                            </div>
                        </>
                    )}
                </div>
            </div>
        </section>
    );
};

export default Keys;
