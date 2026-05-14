import React, { useCallback, useEffect, useMemo, useState, type SyntheticEvent } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../../Shared/Components/Button';
import ScopeBasedComponentAccess from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../../config/security/scopes';
import { FormSelect } from '../../../../../Shared/Components/Form';
import { mapObjects } from '../../../../../config/data/mapper/mapper';
import {
    type EmailTemplate,
    EmailTemplateSchema,
} from '../../EmailTemplates/model/emailTemplate';
import type { EmailAccount } from '../../EmailAccounts/model/emailAccount';
import type { EventBinding, MailEvent } from '../model/emailSettings';

interface Props {
    initial: EventBinding[];
    onSaved: (next: EventBinding[]) => void;
}

interface TemplatesListResponse {
    items?: unknown[];
}

interface BindingState {
    template: string;
    account: string;
}

const EMPTY_BINDING: BindingState = { template: '', account: '' };

export const EventBindingsSection: React.FC<Props> = ({ initial, onSaved }) => {
    const { get, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [events, setEvents] = useState<MailEvent[]>([]);
    const [templates, setTemplates] = useState<EmailTemplate[]>([]);
    const [accounts, setAccounts] = useState<EmailAccount[]>([]);
    const [bindings, setBindings] = useState<Record<string, BindingState>>(() =>
        Object.fromEntries(initial.map(b => [b.event, { template: b.template, account: b.account }])),
    );
    const [loading, setLoading] = useState(true);
    const [submitting, setSubmitting] = useState(false);

    useEffect(() => {
        let cancelled = false;
        (async () => {
            try {
                const [evResp, tplResp, accResp] = await Promise.all([
                    get<{ message?: MailEvent[] }>('admin/mail/settings/events'),
                    get<{ message?: TemplatesListResponse }>('admin/mail/templates?limit=500'),
                    get<{ message?: EmailAccount[] }>('admin/mail/accounts'),
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
        return () => {
            cancelled = true;
        };
    }, [get, errorMessage]);

    const templateOptions = useMemo(
        () => templates.map(t => ({ value: t.name, label: t.name })),
        [templates],
    );
    const accountOptions = useMemo(
        () => accounts.map(a => ({ value: a.name, label: a.is_active ? `${a.name} (active)` : a.name })),
        [accounts],
    );

    const handleTemplateChange = useCallback((eventKey: string, templateName: string) => {
        setBindings(prev => ({
            ...prev,
            [eventKey]: { ...(prev[eventKey] ?? EMPTY_BINDING), template: templateName },
        }));
    }, []);

    const handleAccountChange = useCallback((eventKey: string, accountName: string) => {
        setBindings(prev => ({
            ...prev,
            [eventKey]: { ...(prev[eventKey] ?? EMPTY_BINDING), account: accountName },
        }));
    }, []);

    const rowErrors = useMemo(() => {
        const errs: Record<string, string> = {};
        for (const evt of events) {
            const b = bindings[evt.key] ?? EMPTY_BINDING;
            const hasTpl = b.template !== '';
            const hasAcc = b.account !== '';
            if (hasTpl !== hasAcc) {
                errs[evt.key] = 'Template and account must both be selected, or both left empty.';
            }
        }
        return errs;
    }, [events, bindings]);

    const hasErrors = Object.keys(rowErrors).length > 0;

    const handleSubmit = useCallback(async (e: SyntheticEvent) => {
        e.preventDefault();
        if (hasErrors) {
            errorMessage('Fix the highlighted rows before saving.');
            return;
        }
        const payload: EventBinding[] = events.map(evt => {
            const b = bindings[evt.key] ?? EMPTY_BINDING;
            return { event: evt.key, template: b.template, account: b.account };
        });
        setSubmitting(true);
        try {
            const response = await patch<{ message?: { events?: EventBinding[] } }>('admin/mail/settings', {
                events: payload,
            });
            const next = response?.message?.events ?? payload;
            onSaved(next);
            successMessage('Event bindings saved.');
        } catch (err: unknown) {
            let msg = 'Failed to save bindings';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [events, bindings, hasErrors, patch, successMessage, errorMessage, onSaved]);

    if (loading) {
        return <div className="text-sm text-gray-500 py-2">Loading events…</div>;
    }

    return (
        <form onSubmit={handleSubmit} className="space-y-5">
            <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Event → template &amp; account bindings</h3>
                <p className="text-sm text-gray-500">
                    Choose a template and a sending SMTP account for each system event. Both fields are required as a
                    pair — leave both empty to disable the binding.
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
                                    id={`template-${evt.key}`}
                                    label="Template"
                                    value={b.template}
                                    onChange={v => handleTemplateChange(evt.key, v)}
                                    options={templateOptions}
                                    placeholder="— No template —"
                                    selectClassName={rowErr ? 'border-red-400 dark:border-red-500' : ''}
                                />
                                <FormSelect
                                    id={`account-${evt.key}`}
                                    label="Account"
                                    value={b.account}
                                    onChange={v => handleAccountChange(evt.key, v)}
                                    options={accountOptions}
                                    placeholder="— No account —"
                                    selectClassName={rowErr ? 'border-red-400 dark:border-red-500' : ''}
                                />
                                {rowErr && (
                                    <div className="md:col-span-3 text-xs text-red-600 dark:text-red-400">
                                        {rowErr}
                                    </div>
                                )}
                            </div>
                        );
                    })}
                    {(templates.length === 0 || accounts.length === 0) && (
                        <p className="text-xs text-amber-600 dark:text-amber-400">
                            {templates.length === 0 && 'No active templates exist yet. '}
                            {accounts.length === 0 && 'No SMTP accounts exist yet. '}
                            Create them under Email templates / Email accounts before saving bindings.
                        </p>
                    )}
                </div>
            )}

            <div className="flex justify-end pt-2">
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailSettingsWrite, AppScopes.AdminMailSettings, AppScopes.SuperAdmin]}>
                    <Submit loading={submitting} loadingText="Saving…" label="Save bindings" disabled={hasErrors} />
                </ScopeBasedComponentAccess>
            </div>
        </form>
    );
};

export default EventBindingsSection;
