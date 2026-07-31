import React, { useCallback, useEffect, useMemo, useState, type SyntheticEvent } from 'react';
import { Link } from 'react-router-dom';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../../Shared/Components/Button';
import ScopeBasedComponentAccess from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { FormInput, FormSelect } from '../../../../../Shared/Components/Form';
import { AppScopes } from '../../../../../config/security/scopes';
import { mapObjects } from '../../../../../config/data/mapper/mapper';
import { OrganizationResource } from '../../../model/organization';
import type { EmailAccount } from '../../../../Admin/Settings/EmailAccounts/model/emailAccount';
import { type EmailTemplate, EmailTemplateSchema } from '../../../../Admin/Settings/EmailTemplates/model/emailTemplate';
import type { EventBinding, MailEvent } from '../../../../Admin/Settings/Email/model/emailSettings';
import { type OrgMailSettings, type OrgActivationSettings, EMPTY_ORG_ACTIVATION } from './model/orgEmailSettings';

interface Props {
    organizationId: string;
}

const WRITE_SCOPES = [AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin];

interface TemplatesListResponse {
    items?: unknown[];
}

const EMPTY_BINDING = { template: '', account: '' };

/**
 * OrgEmailSettings — the org-scoped equivalent of the global Admin
 * Settings → Email page. Shows exactly what THIS org has customized;
 * anything left blank falls back to the global mail engine at send
 * time — see api/src/mail/scopedsettings.ScopedResolver.
 */
export const OrgEmailSettings: React.FC<Props> = ({ organizationId }) => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const settingsBase = `organizations/{${OrganizationResource}}/{id:${organizationId}}/mail/settings`;

    const [activeAccount, setActiveAccount] = useState<EmailAccount | null>(null);
    const [activation, setActivation] = useState<OrgActivationSettings>(EMPTY_ORG_ACTIVATION);
    const [events, setEvents] = useState<EventBinding[]>([]);
    const [loading, setLoading] = useState(true);

    const fetchSettings = useCallback(async () => {
        setLoading(true);
        try {
            const response = await get<{ message?: OrgMailSettings }>(settingsBase);
            const data = response?.message;
            if (data) {
                setActiveAccount(data.active_account ?? null);
                setActivation(data.activation ?? EMPTY_ORG_ACTIVATION);
                setEvents(Array.isArray(data.events) ? data.events : []);
            }
        } catch (err: unknown) {
            let msg = 'Failed to load organization mail settings';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [get, settingsBase, errorMessage]);

    useEffect(() => {
        void fetchSettings();
    }, [fetchSettings]);

    if (loading) {
        return <div className="text-sm text-gray-500 py-2 mt-4">Loading email settings…</div>;
    }

    return (
        <div className="space-y-10 mt-4">
            <ActiveAccountBanner account={activeAccount} organizationId={organizationId} />
            <OrgActivationSection organizationId={organizationId} settingsBase={settingsBase} initial={activation} onSaved={setActivation} />
            <div className="border-t border-gray-200 dark:border-surface-800" />
            <OrgEventBindingsSection organizationId={organizationId} settingsBase={settingsBase} initial={events} onSaved={setEvents} />
        </div>
    );
};

const ActiveAccountBanner: React.FC<{ account: EmailAccount | null; organizationId: string }> = ({ account }) => {
    if (!account) {
        return (
            <div className="p-4 border border-gray-200 dark:border-surface-800 bg-gray-50 dark:bg-surface-900/40 rounded-lg">
                <div className="text-sm font-medium text-gray-800 dark:text-gray-200">No active account for this organization</div>
                <p className="text-xs text-gray-500 mt-1">
                    Outbound mail for this organization falls back to the global active SMTP account. Activate an
                    account under the Accounts tab to override it.
                </p>
            </div>
        );
    }
    return (
        <div className="p-4 border border-indigo-200 dark:border-indigo-800 bg-indigo-50 dark:bg-indigo-900/20 rounded-lg">
            <div className="text-sm font-medium text-indigo-900 dark:text-indigo-200">
                Active account for this organization: <span className="font-mono">{account.name}</span>
            </div>
            <div className="text-xs text-indigo-800 dark:text-indigo-300 mt-1">
                {account.host}:{account.port} · From: {account.from_email || '(unset)'} {account.use_tls ? '· TLS' : ''}
            </div>
        </div>
    );
};

const OrgActivationSection: React.FC<{
    organizationId: string;
    settingsBase: string;
    initial: OrgActivationSettings;
    onSaved: (next: OrgActivationSettings) => void;
}> = ({ settingsBase, initial, onSaved }) => {
    const { patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [ttlHours, setTTLHours] = useState(initial.ttl_hours ?? 24);
    const [cooldown, setCooldown] = useState(initial.resend_cooldown_seconds ?? 300);
    const [submitting, setSubmitting] = useState(false);

    const handleSubmit = useCallback(async (e: SyntheticEvent) => {
        e.preventDefault();
        if (ttlHours < 1) { errorMessage('TTL must be at least 1 hour.'); return; }
        if (cooldown < 0) { errorMessage('Resend cooldown cannot be negative.'); return; }
        setSubmitting(true);
        try {
            const response = await patch<{ message?: { activation?: OrgActivationSettings } }>(settingsBase, {
                activation: { ttl_hours: ttlHours, resend_cooldown_seconds: cooldown },
            });
            const next = response?.message?.activation;
            if (next) onSaved(next);
            successMessage('Activation override saved for this organization.');
        } catch (err: unknown) {
            let msg = 'Failed to save activation override';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [ttlHours, cooldown, patch, settingsBase, successMessage, errorMessage, onSaved]);

    return (
        <form onSubmit={handleSubmit} className="space-y-5">
            <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Activation cadence override</h3>
                <p className="text-sm text-gray-500">
                    {/* Go's `omitempty` on a nil *int omits the JSON key entirely rather than
                        sending an explicit null, so this must treat "missing" the same as
                        "null" — a loose `== null` check, not `=== null`. */}
                    {initial.ttl_hours == null && initial.resend_cooldown_seconds == null
                        ? 'Not customized — currently using the global defaults shown below.'
                        : 'This organization has its own activation cadence.'}{' '}
                    Saving here sets (or updates) an override; there is no way to clear it back to the global
                    default once set — see{' '}
                    <Link to="/admin/settings/email" className="underline">Admin Settings → Email</Link>{' '}
                    for the global values.
                </p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <FormInput
                    id="ttlHours"
                    type="number"
                    label="Link lifetime (hours)"
                    value={ttlHours}
                    onChange={v => setTTLHours(parseInt(v, 10) || 0)}
                    min={1}
                    description={initial.ttl_hours == null ? 'Not customized — showing the global default.' : undefined}
                />
                <FormInput
                    id="cooldown"
                    type="number"
                    label="Resend cooldown (seconds)"
                    value={cooldown}
                    onChange={v => setCooldown(parseInt(v, 10) || 0)}
                    min={0}
                    description={initial.resend_cooldown_seconds == null ? 'Not customized — showing the global default.' : undefined}
                />
            </div>
            <div className="flex justify-end pt-2">
                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <Submit loading={submitting} loadingText="Saving…" label="Save activation override" />
                </ScopeBasedComponentAccess>
            </div>
        </form>
    );
};

const OrgEventBindingsSection: React.FC<{
    organizationId: string;
    settingsBase: string;
    initial: EventBinding[];
    onSaved: (next: EventBinding[]) => void;
}> = ({ organizationId, settingsBase, initial, onSaved }) => {
    const { get, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const templatesBase = `organizations/{${OrganizationResource}}/{id:${organizationId}}/mail/templates`;
    const accountsBase = `organizations/{${OrganizationResource}}/{id:${organizationId}}/mail/accounts`;

    const [events, setEvents] = useState<MailEvent[]>([]);
    const [templates, setTemplates] = useState<EmailTemplate[]>([]);
    const [accounts, setAccounts] = useState<EmailAccount[]>([]);
    const [bindings, setBindings] = useState<Record<string, { template: string; account: string }>>(() =>
        Object.fromEntries(initial.map(b => [b.event, { template: b.template, account: b.account }])),
    );
    const [loading, setLoading] = useState(true);
    const [submitting, setSubmitting] = useState(false);

    useEffect(() => {
        let cancelled = false;
        (async () => {
            try {
                // The event catalog itself is global and shared across every
                // tier — reuse the existing admin endpoint rather than
                // duplicating the same static list per organization.
                const [evResp, tplResp, accResp] = await Promise.all([
                    get<{ message?: MailEvent[] }>('admin/mail/settings/events'),
                    get<{ message?: TemplatesListResponse }>(`${templatesBase}?limit=500`),
                    get<{ message?: EmailAccount[] }>(accountsBase),
                ]);
                if (cancelled) return;
                setEvents(Array.isArray(evResp?.message) ? evResp.message : []);
                const rawItems = Array.isArray(tplResp?.message?.items) ? tplResp.message!.items : [];
                const mapped = mapObjects(EmailTemplateSchema, rawItems as object[]) as unknown as EmailTemplate[];
                setTemplates(mapped.filter(t => t.isActive));
                setAccounts(Array.isArray(accResp?.message) ? accResp.message : []);
            } catch (err: unknown) {
                let msg = 'Failed to load event bindings';
                if (err instanceof Error) msg = err.message || msg;
                errorMessage(msg);
            } finally {
                if (!cancelled) setLoading(false);
            }
        })();
        return () => { cancelled = true; };
    }, [get, templatesBase, accountsBase, errorMessage]);

    const templateOptions = useMemo(() => templates.map(t => ({ value: t.name, label: t.name })), [templates]);
    const accountOptions = useMemo(
        () => accounts.map(a => ({ value: a.name, label: a.is_active ? `${a.name} (active)` : a.name })),
        [accounts],
    );

    const rowErrors = useMemo(() => {
        const errs: Record<string, string> = {};
        for (const evt of events) {
            const b = bindings[evt.key] ?? EMPTY_BINDING;
            if ((b.template !== '') !== (b.account !== '')) {
                errs[evt.key] = 'Template and account must both be selected, or both left empty.';
            }
        }
        return errs;
    }, [events, bindings]);
    const hasErrors = Object.keys(rowErrors).length > 0;

    const handleSubmit = useCallback(async (e: SyntheticEvent) => {
        e.preventDefault();
        if (hasErrors) { errorMessage('Fix the highlighted rows before saving.'); return; }
        const payload: EventBinding[] = events.map(evt => {
            const b = bindings[evt.key] ?? EMPTY_BINDING;
            return { event: evt.key, template: b.template, account: b.account };
        });
        setSubmitting(true);
        try {
            const response = await patch<{ message?: { events?: EventBinding[] } }>(settingsBase, { events: payload });
            onSaved(response?.message?.events ?? payload);
            successMessage('Event bindings saved for this organization.');
        } catch (err: unknown) {
            let msg = 'Failed to save bindings';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [events, bindings, hasErrors, patch, settingsBase, successMessage, errorMessage, onSaved]);

    if (loading) {
        return <div className="text-sm text-gray-500 py-2">Loading events…</div>;
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-5">
            <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Event → template &amp; account bindings</h3>
                <p className="text-sm text-gray-500">
                    Bind this organization's own template/account to an event to override the global binding. Leave
                    both blank to keep using the global binding for that event.
                </p>
            </div>

            {events.length === 0 ? (
                <div className="text-sm text-gray-500 italic">No events defined.</div>
            ) : (
                <div className="space-y-4">
                    {events.map(evt => {
                        const b = bindings[evt.key] ?? EMPTY_BINDING;
                        const rowErr = rowErrors[evt.key];
                        return (
                            <div key={evt.key} className="grid grid-cols-1 md:grid-cols-[240px_1fr_1fr] gap-3 items-start">
                                <div>
                                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{evt.label}</div>
                                    <div className="text-xs text-gray-500 mt-1">{evt.description}</div>
                                </div>
                                <FormSelect
                                    id={`org-template-${evt.key}`}
                                    label="Template"
                                    value={b.template}
                                    onChange={v => setBindings(prev => ({ ...prev, [evt.key]: { ...(prev[evt.key] ?? EMPTY_BINDING), template: v } }))}
                                    options={templateOptions}
                                    placeholder="— Use global —"
                                    selectClassName={rowErr ? 'border-red-400 dark:border-red-500' : ''}
                                />
                                <FormSelect
                                    id={`org-account-${evt.key}`}
                                    label="Account"
                                    value={b.account}
                                    onChange={v => setBindings(prev => ({ ...prev, [evt.key]: { ...(prev[evt.key] ?? EMPTY_BINDING), account: v } }))}
                                    options={accountOptions}
                                    placeholder="— Use global —"
                                    selectClassName={rowErr ? 'border-red-400 dark:border-red-500' : ''}
                                />
                                {rowErr && (
                                    <div className="md:col-span-3 text-xs text-red-600 dark:text-red-400">{rowErr}</div>
                                )}
                            </div>
                        );
                    })}
                    {(templates.length === 0 || accounts.length === 0) && (
                        <p className="text-xs text-amber-600 dark:text-amber-400">
                            {templates.length === 0 && 'This organization has no active templates yet. '}
                            {accounts.length === 0 && 'This organization has no SMTP accounts yet. '}
                            Create them under the Templates / Accounts tabs before binding an event to them.
                        </p>
                    )}
                </div>
            )}

            <div className="flex justify-end pt-2">
                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <Submit loading={submitting} loadingText="Saving…" label="Save bindings" disabled={hasErrors} />
                </ScopeBasedComponentAccess>
            </div>
        </form>
    );
};

export default OrgEmailSettings;
