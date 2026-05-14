import React, { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../../Shared/Components/Form';
import { Switch } from '../../../../Shared/Components/Switch';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { EditAction, DeleteAction } from '../../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import { ApplicationResource } from '../../model/application';

// Admin panel for OAuth clients — third-party apps that use
// coco-iam as their OIDC provider. Inverse of the Authentication
// tab's "Continue with X" providers, which is about us being a
// CLIENT of external IdPs.
//
// Secrets are returned by the backend exactly once, at creation
// or rotation; we surface the plaintext in a modal so the admin
// can copy it before it disappears.

type ClientKind = 'public' | 'confidential';

interface ClientView {
    id: string;
    application_id: string;
    client_id: string;
    client_secret_mask: string;
    client_type: ClientKind;
    display_name: string;
    redirect_uris: string[];
    allowed_scopes: string[];
    require_consent: boolean;
    access_token_ttl: number;
    refresh_token_ttl: number;
    is_active: boolean;
    created_at: string;
    updated_at: string;
}

interface FormState {
    client_id: string;
    client_secret: string;
    client_type: ClientKind;
    display_name: string;
    redirect_uris: string;
    allowed_scopes: string;
    require_consent: boolean;
    access_token_ttl: number;
    refresh_token_ttl: number;
    is_active: boolean;
}

const EMPTY_FORM: FormState = {
    client_id: '',
    client_secret: '',
    client_type: 'confidential',
    display_name: '',
    redirect_uris: '',
    allowed_scopes: 'openid profile email',
    require_consent: true,
    access_token_ttl: 3600,
    refresh_token_ttl: 1209600,
    is_active: true,
};

const formFromView = (v: ClientView): FormState => ({
    client_id: v.client_id,
    client_secret: '', // blank → keep stored
    client_type: v.client_type,
    display_name: v.display_name,
    redirect_uris: v.redirect_uris.join('\n'),
    allowed_scopes: v.allowed_scopes.join(' '),
    require_consent: v.require_consent,
    access_token_ttl: v.access_token_ttl,
    refresh_token_ttl: v.refresh_token_ttl,
    is_active: v.is_active,
});

interface Props {
    applicationId?: string;
}

interface SlugInfo {
    orgSlug: string;
    wsSlug: string;
    appSlug: string;
}

const oauthEndpoints = (s: SlugInfo | null) => {
    const base = typeof window !== 'undefined' ? window.location.origin : '';
    const seg = s
        ? `/a/${s.orgSlug}/${s.wsSlug}/${s.appSlug}`
        : '/a/{orgSlug}/{wsSlug}/{appSlug}';
    return {
        authorize: `${base}${seg}/oauth/authorize`,
        token:     `${base}${seg}/oauth/token`,
        userinfo:  `${base}${seg}/oauth/userinfo`,
    };
};

const CopyableUrl: React.FC<{ label: string; url: string; onCopy: (v: string) => void }> = ({ label, url, onCopy }) => (
    <div className="flex items-center gap-2">
        <span className="w-24 shrink-0 text-xs text-gray-500 dark:text-gray-400">{label}</span>
        <span className="flex-1 font-mono text-xs text-gray-800 dark:text-gray-200 truncate">{url}</span>
        <button
            type="button"
            onClick={() => onCopy(url)}
            className="shrink-0 text-xs text-blue-600 dark:text-blue-400 hover:underline"
        >
            Copy
        </button>
    </div>
);

export const OAuthClients: React.FC<Props> = ({ applicationId }) => {
    const { get, post, patch, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [slugs, setSlugs] = useState<SlugInfo | null>(null);
    const [clients, setClients] = useState<ClientView[]>([]);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [formOpen, setFormOpen] = useState(false);
    const [editingId, setEditingId] = useState<string | null>(null);
    const [form, setForm] = useState<FormState>(EMPTY_FORM);
    const [confirmDelete, setConfirmDelete] = useState<ClientView | null>(null);
    // Plaintext secret returned exactly once on create / rotate.
    // Cleared when the admin dismisses the modal.
    const [revealedSecret, setRevealedSecret] = useState<{ clientID: string; secret: string } | null>(null);

    const refresh = useCallback(async () => {
        if (!applicationId) return;
        setLoading(true);
        try {
            const [clientsResp, slugsResp] = await Promise.all([
                get<{ message?: { clients?: ClientView[] } }>(
                    `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-clients`,
                ),
                get<{ message?: { organization_slug?: string; workspace_slug?: string; client_id?: string } }>(
                    `applications/{${ApplicationResource}}/{id:${applicationId}}/login-settings`,
                ),
            ]);
            setClients(clientsResp?.message?.clients ?? []);
            const m = slugsResp?.message;
            if (m?.organization_slug && m?.workspace_slug && m?.client_id) {
                setSlugs({ orgSlug: m.organization_slug, wsSlug: m.workspace_slug, appSlug: m.client_id });
            }
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load OAuth clients');
        } finally {
            setLoading(false);
        }
    }, [applicationId, get, errorMessage]);

    useEffect(() => { void refresh(); }, [refresh]);

    const startCreate = () => {
        setEditingId(null);
        setForm(EMPTY_FORM);
        setFormOpen(true);
    };

    const startEdit = (v: ClientView) => {
        setEditingId(v.id);
        setForm(formFromView(v));
        setFormOpen(true);
    };

    const cancelForm = () => {
        setFormOpen(false);
        setEditingId(null);
        setForm(EMPTY_FORM);
    };

    const save = async () => {
        if (!applicationId) return;
        if (!form.client_id.trim()) {
            errorMessage('Client ID is required');
            return;
        }
        const uris = form.redirect_uris.split('\n').map(s => s.trim()).filter(Boolean);
        if (uris.length === 0) {
            errorMessage('At least one redirect URI is required');
            return;
        }
        const scopes = form.allowed_scopes.split(/\s+/).map(s => s.trim()).filter(Boolean);
        setSaving(true);
        try {
            const body: Record<string, unknown> = {
                display_name: form.display_name.trim(),
                redirect_uris: uris,
                allowed_scopes: scopes,
                require_consent: form.require_consent,
                access_token_ttl: form.access_token_ttl,
                refresh_token_ttl: form.refresh_token_ttl,
                is_active: form.is_active,
            };
            if (editingId) {
                if (form.client_secret.trim() !== '') {
                    body.client_secret = form.client_secret;
                }
                const resp = await patch<{ message?: { client?: ClientView; client_secret?: string } }>(
                    `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-clients/${editingId}`,
                    body,
                );
                if (resp?.message?.client_secret) {
                    setRevealedSecret({ clientID: resp.message.client?.client_id ?? '', secret: resp.message.client_secret });
                }
                successMessage('Client updated.');
            } else {
                body.client_id = form.client_id.trim();
                body.client_type = form.client_type;
                const resp = await post<{ message?: { client?: ClientView; client_secret?: string } }>(
                    `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-clients`,
                    body,
                );
                if (resp?.message?.client_secret) {
                    setRevealedSecret({ clientID: resp.message.client?.client_id ?? '', secret: resp.message.client_secret });
                }
                successMessage('Client created.');
            }
            cancelForm();
            await refresh();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to save OAuth client');
        } finally {
            setSaving(false);
        }
    };

    const rotateSecret = async (v: ClientView) => {
        if (!applicationId) return;
        try {
            const resp = await post<{ message?: { client?: ClientView; client_secret?: string } }>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-clients/${v.id}/rotate-secret`,
                {},
            );
            if (resp?.message?.client_secret) {
                setRevealedSecret({ clientID: v.client_id, secret: resp.message.client_secret });
            }
            successMessage('Client secret rotated.');
            await refresh();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to rotate secret');
        }
    };

    const remove = async (v: ClientView) => {
        if (!applicationId) return;
        try {
            await del(`applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-clients/${v.id}`);
            successMessage('Client removed.');
            setConfirmDelete(null);
            await refresh();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to remove client');
        }
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text).then(
            () => successMessage('Copied to clipboard.'),
            () => errorMessage('Clipboard copy failed'),
        );
    };

    const endpoints = oauthEndpoints(slugs);

    return (
        <div>
            <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">
                Register third-party applications that authenticate their users via this
                application's OAuth 2.0 / OIDC endpoints. Each client gets its own
                client_id; confidential clients additionally hold a one-time-revealed
                client_secret.
            </p>

            <div className="mb-5 rounded-lg border border-gray-200 dark:border-surface-700 bg-gray-50 dark:bg-surface-900 p-3 space-y-2">
                <div className="text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">OAuth server endpoints</div>
                <CopyableUrl label="Auth URL"      url={endpoints.authorize} onCopy={copyToClipboard} />
                <CopyableUrl label="Token URL"     url={endpoints.token}     onCopy={copyToClipboard} />
                <CopyableUrl label="User Info URL" url={endpoints.userinfo}  onCopy={copyToClipboard} />
            </div>

            {loading ? (
                <div className="text-sm text-gray-500">Loading…</div>
            ) : (
                <>
                    {clients.length === 0 ? (
                        <div className="text-sm text-gray-500 italic mb-4">No OAuth clients registered.</div>
                    ) : (
                        <div className="space-y-3 mb-4">
                            {clients.map(v => (
                                <div key={v.id} className="rounded-lg border border-gray-200 dark:border-surface-700 p-3">
                                    <div className="flex items-start gap-3">
                                        <div className="flex-1">
                                            <div className="flex items-center gap-2">
                                                <span className="font-medium text-gray-900 dark:text-gray-100">
                                                    {v.display_name || v.client_id}
                                                </span>
                                                <span className={`text-xs px-2 py-0.5 rounded ${v.client_type === 'public'
                                                    ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300'
                                                    : 'bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300'}`}>
                                                    {v.client_type}
                                                </span>
                                                {!v.is_active && (
                                                    <span className="text-xs px-2 py-0.5 rounded bg-gray-200 dark:bg-surface-700 text-gray-700 dark:text-gray-300">
                                                        inactive
                                                    </span>
                                                )}
                                                {!v.require_consent && (
                                                    <span className="text-xs px-2 py-0.5 rounded bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300">
                                                        consent skipped
                                                    </span>
                                                )}
                                            </div>
                                            <div className="text-xs text-gray-500 dark:text-gray-400 mt-1 font-mono">
                                                client_id: {v.client_id}
                                            </div>
                                            <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                                redirect: <span className="font-mono">{v.redirect_uris[0] ?? '(none)'}</span>
                                                {v.redirect_uris.length > 1 ? ` +${v.redirect_uris.length - 1} more` : ''}
                                            </div>
                                            <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                                scopes: <span className="font-mono">{v.allowed_scopes.join(' ') || '(none)'}</span>
                                            </div>
                                        </div>
                                        <div className="flex flex-col gap-2 items-end">
                                            <div className="flex gap-2">
                                                <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsOauth, AppScopes.Applications, AppScopes.SuperAdmin]}>
                                                    <EditAction onClick={() => startEdit(v)} disabled={saving} />
                                                </ScopeBasedComponentAccess>
                                                <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsOauth, AppScopes.Applications, AppScopes.SuperAdmin]}>
                                                    <DeleteAction onClick={() => setConfirmDelete(v)} disabled={saving} />
                                                </ScopeBasedComponentAccess>
                                            </div>
                                            {v.client_type === 'confidential' && (
                                                <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsOauth, AppScopes.Applications, AppScopes.SuperAdmin]}>
                                                    <button
                                                        type="button"
                                                        onClick={() => void rotateSecret(v)}
                                                        className="text-xs text-blue-600 hover:underline"
                                                    >
                                                        Rotate secret
                                                    </button>
                                                </ScopeBasedComponentAccess>
                                            )}
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}

                    {!formOpen && (
                        <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsOauth, AppScopes.Applications, AppScopes.SuperAdmin]}>
                            <Submit type="button" onClick={startCreate} label="Register OAuth client" />
                        </ScopeBasedComponentAccess>
                    )}
                </>
            )}

            {formOpen && (
                <div className="mt-4 p-4 rounded-lg border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900">
                    <div className="text-sm font-medium mb-3">
                        {editingId ? 'Edit OAuth client' : 'Register OAuth client'}
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <FormInput
                            id="client_id"
                            label="Client ID"
                            value={form.client_id}
                            onChange={v => setForm({ ...form, client_id: v })}
                            disabled={!!editingId}
                            placeholder="reporter"
                        />
                        {!editingId && (
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Client type</label>
                                <select
                                    value={form.client_type}
                                    onChange={e => setForm({ ...form, client_type: e.target.value as ClientKind })}
                                    className="w-full px-3 py-2 rounded-lg border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100"
                                >
                                    <option value="confidential">Confidential (server-side, has client_secret)</option>
                                    <option value="public">Public (SPA / native, PKCE only)</option>
                                </select>
                            </div>
                        )}
                        <FormInput
                            id="display_name"
                            label="Display name"
                            value={form.display_name}
                            onChange={v => setForm({ ...form, display_name: v })}
                            placeholder="Reporter"
                        />
                        {editingId && form.client_type === 'confidential' && (
                            <FormInput
                                id="client_secret"
                                label="Client Secret (leave blank to keep)"
                                value={form.client_secret}
                                onChange={v => setForm({ ...form, client_secret: v })}
                                type="password"
                            />
                        )}
                    </div>

                    <div className="mt-3">
                        <FormTextarea
                            label="Redirect URIs (one per line, exact match required)"
                            value={form.redirect_uris}
                            onChange={v => setForm({ ...form, redirect_uris: v })}
                            placeholder={'https://reporter.example/cb\nhttp://localhost:3000/cb'}
                        />
                    </div>

                    <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-4">
                        <FormInput
                            id="allowed_scopes"
                            label="Allowed scopes (space-separated)"
                            value={form.allowed_scopes}
                            onChange={v => setForm({ ...form, allowed_scopes: v })}
                        />
                        <div className="flex items-end gap-6 pb-1">
                            <Switch
                                checked={form.require_consent}
                                onChange={v => setForm({ ...form, require_consent: v })}
                                label="Require consent screen"
                            />
                            {editingId && (
                                <Switch
                                    checked={form.is_active}
                                    onChange={v => setForm({ ...form, is_active: v })}
                                    label="Active"
                                />
                            )}
                        </div>
                    </div>

                    <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-4">
                        <FormInput
                            id="access_token_ttl"
                            label="Access token TTL (seconds)"
                            type="number"
                            value={form.access_token_ttl}
                            onChange={v => setForm({ ...form, access_token_ttl: Number(v) || 3600 })}
                        />
                        <FormInput
                            id="refresh_token_ttl"
                            label="Refresh token TTL (seconds)"
                            type="number"
                            value={form.refresh_token_ttl}
                            onChange={v => setForm({ ...form, refresh_token_ttl: Number(v) || 1209600 })}
                        />
                    </div>

                    <div className="flex gap-2 mt-4">
                        <Submit type="button" onClick={() => void save()} disabled={saving}
                            label={editingId ? 'Save changes' : 'Create client'} />
                        <button
                            type="button"
                            onClick={cancelForm}
                            className="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-surface-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-surface-700"
                        >
                            Cancel
                        </button>
                    </div>
                </div>
            )}

            {revealedSecret && (
                <ConfirmModal
                    title="Save the client secret"
                    message={`This is the only time the client_secret for "${revealedSecret.clientID}" is shown. Copy it now — it cannot be retrieved later.\n\n${revealedSecret.secret}`}
                    confirmLabel="I've saved it"
                    onConfirm={() => {
                        copyToClipboard(revealedSecret.secret);
                        setRevealedSecret(null);
                    }}
                    onCancel={() => setRevealedSecret(null)}
                />
            )}

            {confirmDelete && (
                <ConfirmModal
                    title="Remove OAuth client"
                    message={`Remove the "${confirmDelete.display_name || confirmDelete.client_id}" client? Existing tokens issued to this client will be rejected on next use; the client will not be able to start new authorization flows.`}
                    confirmLabel="Remove"
                    onConfirm={() => void remove(confirmDelete)}
                    onCancel={() => setConfirmDelete(null)}
                />
            )}
        </div>
    );
};

export default OAuthClients;
