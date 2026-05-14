import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput } from '../../../../Shared/Components/Form';
import { Switch } from '../../../../Shared/Components/Switch';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import { ApplicationResource } from '../../model/application';

// The admin-facing Authentication panel. Renders every supported
// auth type as an accordion row: Legacy (password/email) first,
// then each external IdP. Every row carries its enable toggle in
// the header so the admin can flip it without expanding; the
// expanded body holds the provider-specific configuration.
//
// Invariant: an auth type is only usable when its enable toggle
// is on AND — for OAuth — the row is configured (client id +
// secret present). OAuth rows default to disabled on creation
// so a partially-filled configuration can't accidentally go live.

export type ProviderKind = 'google' | 'github' | 'microsoft';

interface ProviderView {
    id: string;
    application_id: string;
    provider: ProviderKind;
    client_id: string;
    client_secret_mask: string;
    discovery_url?: string;
    authorize_url?: string;
    token_url?: string;
    userinfo_url?: string;
    scopes: string[];
    allow_login: boolean;
    allow_registration: boolean;
    is_active: boolean;
    created_at: string;
    updated_at: string;
}

interface Props {
    applicationId?: string;
    appSlug?: string;
    orgSlug?: string;
    wsSlug?: string;
}

const SUPPORTED_PROVIDERS: ProviderKind[] = ['google', 'github', 'microsoft'];

const PROVIDER_LABELS: Record<ProviderKind, string> = {
    google: 'Google',
    github: 'GitHub',
    microsoft: 'Microsoft',
};

const DEFAULT_SCOPES: Record<ProviderKind, string> = {
    google: 'openid email profile',
    github: 'read:user user:email',
    microsoft: 'openid email profile',
};

const publicBase = () => {
    if (typeof window === 'undefined') return '';
    return window.location.origin.replace(/\/$/, '');
};

const callbackURL = (orgSlug: string | undefined, wsSlug: string | undefined, appSlug: string | undefined, provider: ProviderKind) =>
    `${publicBase()}/a/${orgSlug || '{orgSlug}'}/${wsSlug || '{wsSlug}'}/${appSlug || '{appSlug}'}/auth/oauth/${provider}/callback`;

// ProviderFormState tracks the per-row config inputs while the
// admin is editing. Each accordion row keeps its own state
// because the admin might have multiple rows expanded at once.
interface ProviderFormState {
    client_id: string;
    client_secret: string;
    discovery_url: string;
    authorize_url: string;
    token_url: string;
    userinfo_url: string;
    scopes: string;
    allow_registration: boolean;
}

const emptyForm = (provider: ProviderKind): ProviderFormState => ({
    client_id: '',
    client_secret: '',
    discovery_url: '',
    authorize_url: '',
    token_url: '',
    userinfo_url: '',
    scopes: DEFAULT_SCOPES[provider],
    allow_registration: false,
});

const formFromView = (p: ProviderView): ProviderFormState => ({
    client_id: p.client_id,
    client_secret: '', // blank means "don't rotate"
    discovery_url: p.discovery_url ?? '',
    authorize_url: p.authorize_url ?? '',
    token_url: p.token_url ?? '',
    userinfo_url: p.userinfo_url ?? '',
    scopes: p.scopes.join(' '),
    allow_registration: p.allow_registration,
});

// AppFlags is the slice of the Application row this panel reads
// and writes (allow_password_login lives on the app, not on any
// OAuth row). We keep it narrow so the Authentication tab
// doesn't have to care about the rest of the Edit form's state.
interface AppFlags {
    allow_password_login: boolean;
    // These two are edited on the "Edit" tab; we only READ them
    // here to forward in the PATCH body unchanged (the admin API
    // requires full objects on update).
    title: string;
    client_id: string;
    description: string;
    is_active: boolean;
    allow_recovery: boolean;
    allow_registration: boolean;
}

// Top-level accordion sections identify by id. The OAuth group
// is treated as its own section; its id is used to drive the
// single-expand invariant separately from the inner provider
// rows (so expanding Google doesn't collapse OAuth itself).
const SECTION_PASSWORD = 'password';
const SECTION_OAUTH = 'oauth';

export const Authentication: React.FC<Props> = ({
    applicationId, orgSlug, wsSlug, appSlug,
}) => {
    const { get, post, patch, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [providers, setProviders] = useState<ProviderView[]>([]);
    const [appFlags, setAppFlags] = useState<AppFlags | null>(null);
    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState(false);
    // topSection is the top-level accordion row that's open
    // (SECTION_PASSWORD or SECTION_OAUTH). Only one at a time.
    const [topSection, setTopSection] = useState<string | null>(SECTION_OAUTH);
    // providerOpen is the OAuth provider currently expanded
    // inside the OAuth group. Independent of topSection so
    // picking Google doesn't collapse the outer OAuth row.
    const [providerOpen, setProviderOpen] = useState<ProviderKind | null>(null);
    // Per-provider form state, keyed by provider kind.
    const [forms, setForms] = useState<Partial<Record<ProviderKind, ProviderFormState>>>({});
    const [confirmDelete, setConfirmDelete] = useState<ProviderView | null>(null);

    const refresh = useCallback(async () => {
        if (!applicationId) return;
        setLoading(true);
        try {
            const [appResp, provResp] = await Promise.all([
                get<{ message?: { allow_password_login?: boolean; title?: string; client_id?: string; description?: string; is_active?: boolean; allow_recovery?: boolean; allow_registration?: boolean } }>(`applications/{${ApplicationResource}}/{id:${applicationId}}`),
                get<{ message?: { providers?: ProviderView[] } }>(
                    `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-providers`,
                ),
            ]);
            const raw = appResp?.message;
            if (raw) {
                setAppFlags({
                    allow_password_login: raw.allow_password_login ?? true,
                    title: raw.title ?? '',
                    client_id: raw.client_id ?? '',
                    description: raw.description ?? '',
                    is_active: raw.is_active ?? true,
                    allow_recovery: raw.allow_recovery ?? true,
                    allow_registration: raw.allow_registration ?? false,
                });
            }
            setProviders(provResp?.message?.providers ?? []);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load authentication settings');
        } finally {
            setLoading(false);
        }
    }, [applicationId, get, errorMessage]);

    useEffect(() => {
        void refresh();
    }, [refresh]);

    // Seed / reset a provider's form state. Called when the
    // accordion row is expanded for the first time.
    const ensureForm = useCallback((provider: ProviderKind, current?: ProviderView) => {
        setForms(prev => {
            if (prev[provider]) return prev;
            return { ...prev, [provider]: current ? formFromView(current) : emptyForm(provider) };
        });
    }, []);

    const updateForm = (provider: ProviderKind, patch: Partial<ProviderFormState>) => {
        setForms(prev => ({
            ...prev,
            [provider]: { ...(prev[provider] ?? emptyForm(provider)), ...patch },
        }));
    };

    // Top-level section toggle (Password row or OAuth group).
    const toggleTopSection = (id: string) => {
        setTopSection(prev => (prev === id ? null : id));
    };

    // Nested provider toggle inside the OAuth group.
    const toggleProvider = (kind: ProviderKind, current?: ProviderView) => {
        const next = providerOpen === kind ? null : kind;
        setProviderOpen(next);
        if (next) {
            ensureForm(next, current);
        }
    };

    const setPasswordAllowed = async (next: boolean) => {
        if (!applicationId || !appFlags) return;
        setBusy(true);
        try {
            await patch(`applications/{${ApplicationResource}}/{id:${applicationId}}`, {
                title: appFlags.title,
                client_id: appFlags.client_id,
                description: appFlags.description,
                is_active: appFlags.is_active,
                allow_recovery: appFlags.allow_recovery,
                allow_registration: appFlags.allow_registration,
                allow_password_login: next,
            });
            setAppFlags({ ...appFlags, allow_password_login: next });
            successMessage(next ? 'Password login enabled.' : 'Password login disabled.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to update password login');
        } finally {
            setBusy(false);
        }
    };

    const setProviderAllowed = async (p: ProviderView, next: boolean) => {
        if (!applicationId) return;
        setBusy(true);
        try {
            // PATCH is full-object; resend the existing values and
            // flip just allow_login. Secret stays untouched (omitted
            // client_secret → backend preserves the stored one).
            await patch(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-providers/${p.id}`,
                {
                    client_id: p.client_id,
                    discovery_url: p.discovery_url ?? '',
                    authorize_url: p.authorize_url ?? '',
                    token_url: p.token_url ?? '',
                    userinfo_url: p.userinfo_url ?? '',
                    scopes: p.scopes,
                    allow_login: next,
                    allow_registration: p.allow_registration,
                    is_active: p.is_active,
                },
            );
            setProviders(prev => prev.map(x => x.id === p.id ? { ...x, allow_login: next } : x));
            successMessage(next ? `${PROVIDER_LABELS[p.provider]} enabled.` : `${PROVIDER_LABELS[p.provider]} disabled.`);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to update provider');
        } finally {
            setBusy(false);
        }
    };

    const saveProvider = async (provider: ProviderKind, existing?: ProviderView) => {
        if (!applicationId) return;
        const form = forms[provider];
        if (!form) return;
        if (!form.client_id.trim()) {
            errorMessage('Client ID is required');
            return;
        }
        if (!existing && !form.client_secret.trim()) {
            errorMessage('Client Secret is required for new providers');
            return;
        }
        setBusy(true);
        try {
            const base: Record<string, unknown> = {
                client_id: form.client_id.trim(),
                discovery_url: form.discovery_url.trim(),
                authorize_url: form.authorize_url.trim(),
                token_url: form.token_url.trim(),
                userinfo_url: form.userinfo_url.trim(),
                scopes: form.scopes.split(/\s+/).map(s => s.trim()).filter(Boolean),
                allow_registration: form.allow_registration,
            };
            if (existing) {
                const body: Record<string, unknown> = {
                    ...base,
                    allow_login: existing.allow_login,
                    is_active: existing.is_active,
                };
                if (form.client_secret.trim() !== '') {
                    body.client_secret = form.client_secret;
                }
                await patch(
                    `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-providers/${existing.id}`,
                    body,
                );
                successMessage(`${PROVIDER_LABELS[provider]} updated.`);
            } else {
                // New providers are ALWAYS created with allow_login=false
                // — the admin flips the header toggle once they've
                // verified the callback URL on the IdP side.
                await post(
                    `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-providers`,
                    {
                        ...base,
                        provider,
                        client_secret: form.client_secret,
                        allow_login: false,
                    },
                );
                successMessage(`${PROVIDER_LABELS[provider]} added. Flip the toggle to enable it.`);
            }
            await refresh();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to save provider');
        } finally {
            setBusy(false);
        }
    };

    const deleteProvider = async (p: ProviderView) => {
        if (!applicationId) return;
        try {
            await del(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-providers/${p.id}`,
            );
            successMessage(`${PROVIDER_LABELS[p.provider]} removed.`);
            setConfirmDelete(null);
            await refresh();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to remove provider');
        }
    };

    const copyCallback = (provider: ProviderKind) => {
        const url = callbackURL(orgSlug, wsSlug, appSlug, provider);
        if (navigator.clipboard) {
            navigator.clipboard.writeText(url).then(
                () => successMessage(url.includes('{') ? `Callback template copied — fill slugs: ${url}` : `Callback URL copied: ${url}`),
                () => errorMessage('Clipboard copy failed'),
            );
        }
    };

    const providerByKind = useMemo(() => {
        const m: Partial<Record<ProviderKind, ProviderView>> = {};
        providers.forEach(p => { m[p.provider] = p; });
        return m;
    }, [providers]);

    if (loading) {
        return <div className="text-sm text-gray-500">Loading authentication settings…</div>;
    }

    return (
        <div>
            <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">
                Configure which authentication types can be used for this application.
                Password login is enabled by default; external providers are disabled by
                default and stay that way until you flip their toggle.
            </p>

            <div className="space-y-3">
                {/* Legacy / password row */}
                <AccordionRow
                    id={SECTION_PASSWORD}
                    title="Password / Email"
                    subtitle="Legacy username + password login against the user's stored credentials."
                    expanded={topSection === SECTION_PASSWORD}
                    onToggle={() => toggleTopSection(SECTION_PASSWORD)}
                    enabled={appFlags?.allow_password_login ?? true}
                    onEnabledChange={v => void setPasswordAllowed(v)}
                    enabledDisabled={busy}
                    statusChip={appFlags?.allow_password_login ? 'enabled' : 'disabled'}
                >
                    <div className="text-sm text-gray-600 dark:text-gray-400">
                        Password login is controlled at the application level. Recovery and self-registration
                        toggles live on the Edit tab — this row only governs whether the endpoint accepts
                        username + password submissions at all.
                    </div>
                </AccordionRow>

                {/* OAuth group — wraps the individual IdP providers. No
                    master toggle; each inner provider has its own. */}
                <AccordionRow
                    id={SECTION_OAUTH}
                    title="OAuth"
                    subtitle="External identity providers — Google, GitHub, Microsoft. Configure each provider and flip its toggle to enable."
                    expanded={topSection === SECTION_OAUTH}
                    onToggle={() => toggleTopSection(SECTION_OAUTH)}
                    summaryBadge={oauthSummaryBadge(providers)}
                >
                    <div className="space-y-3">
                        {SUPPORTED_PROVIDERS.map(kind => {
                            const view = providerByKind[kind];
                            const configured = !!view;
                            const form = forms[kind] ?? (view ? formFromView(view) : emptyForm(kind));
                            const rowEnabled = view?.allow_login ?? false;
                            const toggleDisabled = busy || !configured;

                            return (
                                <AccordionRow
                                    key={kind}
                                    id={`oauth-${kind}`}
                                    title={PROVIDER_LABELS[kind]}
                                    subtitle={configured
                                        ? `Configured • client_id: ${view!.client_id.slice(0, 16)}…`
                                        : 'Not configured — expand to add credentials.'}
                                    expanded={providerOpen === kind}
                                    onToggle={() => toggleProvider(kind, view)}
                                    enabled={rowEnabled}
                                    onEnabledChange={v => view && void setProviderAllowed(view, v)}
                                    enabledDisabled={toggleDisabled}
                                    statusChip={rowEnabled ? 'enabled' : (configured ? 'disabled' : 'not configured')}
                                    nested
                                >
                                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsOauth, AppScopes.Applications, AppScopes.SuperAdmin]}>
                                        <div>
                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                            <FormInput
                                                id={`${kind}_client_id`}
                                                label="Client ID"
                                                value={form.client_id}
                                                onChange={v => updateForm(kind, { client_id: v })}
                                            />
                                            <FormInput
                                                id={`${kind}_client_secret`}
                                                label={configured ? 'Client Secret (leave blank to keep)' : 'Client Secret'}
                                                value={form.client_secret}
                                                onChange={v => updateForm(kind, { client_secret: v })}
                                                type="password"
                                            />
                                            <FormInput
                                                id={`${kind}_scopes`}
                                                label="Scopes (space-separated)"
                                                value={form.scopes}
                                                onChange={v => updateForm(kind, { scopes: v })}
                                            />
                                            <FormInput
                                                id={`${kind}_discovery_url`}
                                                label="Discovery URL (optional, OIDC only)"
                                                value={form.discovery_url}
                                                onChange={v => updateForm(kind, { discovery_url: v })}
                                                placeholder="https://accounts.google.com/.well-known/openid-configuration"
                                            />
                                            <FormInput
                                                id={`${kind}_authorize_url`}
                                                label="Authorize URL (override)"
                                                value={form.authorize_url}
                                                onChange={v => updateForm(kind, { authorize_url: v })}
                                            />
                                            <FormInput
                                                id={`${kind}_token_url`}
                                                label="Token URL (override)"
                                                value={form.token_url}
                                                onChange={v => updateForm(kind, { token_url: v })}
                                            />
                                            <FormInput
                                                id={`${kind}_userinfo_url`}
                                                label="Userinfo URL (override)"
                                                value={form.userinfo_url}
                                                onChange={v => updateForm(kind, { userinfo_url: v })}
                                            />
                                        </div>
                                        <div className="flex items-center gap-6 mt-3">
                                            <Switch
                                                checked={form.allow_registration}
                                                onChange={v => updateForm(kind, { allow_registration: v })}
                                                label="Allow new sign-ups via this provider"
                                            />
                                        </div>
                                        <div className="flex items-center gap-3 mt-4">
                                            <Submit type="button" onClick={() => void saveProvider(kind, view)} disabled={busy} label={configured ? 'Save changes' : 'Create provider'} />
                                            <button
                                                type="button"
                                                onClick={() => copyCallback(kind)}
                                                className="px-3 py-1.5 text-xs font-medium rounded-md border border-gray-300 dark:border-surface-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-surface-700"
                                            >
                                                Copy callback URL
                                            </button>
                                            {view && (
                                                <button
                                                    type="button"
                                                    onClick={() => setConfirmDelete(view)}
                                                    disabled={busy}
                                                    className="ml-auto px-3 py-1.5 text-xs font-medium rounded-md border border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/50 dark:text-red-300 dark:hover:bg-red-900/30"
                                                >
                                                    Remove provider
                                                </button>
                                            )}
                                        </div>
                                        </div>
                                    </ScopeBasedComponentAccess>
                                </AccordionRow>
                            );
                        })}
                    </div>
                </AccordionRow>
            </div>

            {confirmDelete && (
                <ConfirmModal
                    title="Remove provider"
                    message={`Remove ${PROVIDER_LABELS[confirmDelete.provider]} from this application? Existing linked users are unaffected but won't be able to log in via this provider anymore.`}
                    confirmLabel="Remove"
                    onConfirm={() => void deleteProvider(confirmDelete)}
                    onCancel={() => setConfirmDelete(null)}
                />
            )}
        </div>
    );
};

// -- AccordionRow --------------------------------------------------

// AccordionRow renders one collapsible section. Three modes:
//   1. Default — title + subtitle + status chip + enable Switch
//      on the right. Used for Password and each OAuth provider.
//   2. Group — no toggle, an optional summaryBadge on the right.
//      Used for the OAuth parent that wraps three nested rows.
//   3. Nested — visual indent so nested rows sit clearly under
//      their parent.
interface AccordionRowProps {
    id: string;
    title: string;
    subtitle: string;
    expanded: boolean;
    onToggle: () => void;
    // enabled + onEnabledChange together enable the per-row
    // Switch. Both absent → the row has no toggle and instead
    // shows summaryBadge on the right-hand side.
    enabled?: boolean;
    onEnabledChange?: (next: boolean) => void;
    enabledDisabled?: boolean;
    statusChip?: 'enabled' | 'disabled' | 'not configured';
    // summaryBadge is rendered instead of the Switch when the
    // row is a group header (no enable semantics of its own).
    summaryBadge?: React.ReactNode;
    // nested renders the row with a subtle indent + lighter
    // border so it reads as "inside" the parent section.
    nested?: boolean;
    children: React.ReactNode;
}

const AccordionRow: React.FC<AccordionRowProps> = ({
    id, title, subtitle, expanded, onToggle, enabled, onEnabledChange,
    enabledDisabled, statusChip, summaryBadge, nested, children,
}) => {
    const chipClass = statusChip === 'enabled'
        ? 'bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300'
        : statusChip === 'disabled'
            ? 'bg-gray-200 dark:bg-surface-700 text-gray-700 dark:text-gray-300'
            : 'bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300';
    const hasToggle = typeof enabled === 'boolean' && onEnabledChange !== undefined;

    return (
        <div className={`rounded-lg border overflow-hidden ${nested
            ? 'border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900'
            : 'border-gray-200 dark:border-surface-700'
        }`}>
            <div className="flex items-stretch">
                <button
                    type="button"
                    onClick={onToggle}
                    className="flex items-center gap-3 flex-1 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-surface-800/50"
                    aria-expanded={expanded}
                    aria-controls={`accordion-body-${id}`}
                >
                    <span className={`inline-block transform transition-transform text-gray-500 ${expanded ? 'rotate-90' : ''}`} aria-hidden="true">
                        ▶
                    </span>
                    <div className="flex-1">
                        <div className="flex items-center gap-2">
                            <span className="font-medium text-gray-900 dark:text-gray-100">{title}</span>
                            {statusChip && (
                                <span className={`text-xs px-2 py-0.5 rounded ${chipClass}`}>{statusChip}</span>
                            )}
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{subtitle}</div>
                    </div>
                </button>
                <div className="flex items-center px-4 border-l border-gray-200 dark:border-surface-700 bg-gray-50 dark:bg-surface-800/40">
                    {hasToggle ? (
                        <Switch
                            checked={enabled!}
                            onChange={onEnabledChange!}
                            label="Enabled"
                            disabled={enabledDisabled}
                        />
                    ) : summaryBadge}
                </div>
            </div>
            {expanded && (
                <div
                    id={`accordion-body-${id}`}
                    className="px-4 pt-3 pb-4 border-t border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900"
                >
                    {children}
                </div>
            )}
        </div>
    );
};

// oauthSummaryBadge renders the "X of N enabled" / "N not yet
// configured" indicator shown on the OAuth group header. Kept
// outside the component so AccordionRow stays layout-only.
function oauthSummaryBadge(providers: ProviderView[]): React.ReactNode {
    const total = SUPPORTED_PROVIDERS.length;
    const enabled = providers.filter(p => p.allow_login).length;
    const configured = providers.length;
    if (configured === 0) {
        return <span className="text-xs text-gray-500 dark:text-gray-400">none configured</span>;
    }
    const toneClass = enabled > 0
        ? 'bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300'
        : 'bg-gray-200 dark:bg-surface-700 text-gray-700 dark:text-gray-300';
    return (
        <span className={`text-xs px-2 py-0.5 rounded ${toneClass}`}>
            {enabled} of {total} enabled
        </span>
    );
}

export default Authentication;
