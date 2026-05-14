import React, { useCallback, useEffect, useState, type SyntheticEvent } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../Shared/Components/Button';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { FormInput, FormTextarea } from '../../../../Shared/Components/Form';
import {
    type GeneralSettings,
    type GeneralSettingsPatch,
    EMPTY_GENERAL,
    ROBOTS_PRESETS,
} from './model/generalSettings';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export interface GeneralSettingsManagerProps {
    className?: string;
}

/**
 * GeneralSettingsManager owns the Admin Settings → General screen.
 * Hosts app-wide identity/branding + the base URL that link-builders
 * (activation, password recovery, …) consume.
 */
export const GeneralSettingsManager: React.FC<GeneralSettingsManagerProps> = ({
    className = '',
}) => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Settings' }, { label: 'General' }]);
    const { get, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [form, setForm] = useState<GeneralSettings>(EMPTY_GENERAL);
    const [loading, setLoading] = useState(true);
    const [submitting, setSubmitting] = useState(false);

    const fetchSettings = useCallback(async () => {
        setLoading(true);
        try {
            const response = await get<{ message?: GeneralSettings }>('admin/settings/general');
            const data = response?.message;
            if (data) setForm(data);
        } catch (err: unknown) {
            let msg = 'Failed to load general settings';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void fetchSettings();
    }, [fetchSettings]);

    const update = useCallback((patchObj: Partial<GeneralSettings>) => {
        setForm(prev => ({ ...prev, ...patchObj }));
    }, []);

    const baseURLError = (() => {
        const trimmed = form.base_url.trim();
        if (trimmed === '') return 'Required — link-builders (activation, password recovery) need it.';
        if (!/^https?:\/\//i.test(trimmed)) return 'Must start with http:// or https://';
        return '';
    })();

    const handleSubmit = useCallback(async (e: SyntheticEvent) => {
        e.preventDefault();
        if (baseURLError) {
            errorMessage(baseURLError);
            return;
        }
        const body: GeneralSettingsPatch = {
            base_url: form.base_url.trim().replace(/\/+$/, ''),
            page_title: form.page_title.trim(),
            description: form.description.trim(),
            robots: form.robots.trim(),
        };
        setSubmitting(true);
        try {
            const response = await patch<{ message?: GeneralSettings }>('admin/settings/general', body);
            if (response?.message) setForm(response.message);
            successMessage('General settings saved.');
        } catch (err: unknown) {
            let msg = 'Failed to save general settings';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [form, baseURLError, patch, successMessage, errorMessage]);

    if (loading) {
        return <div className={className}><div className="text-sm text-gray-500 py-2">Loading general settings…</div></div>;
    }

    return (
        <form onSubmit={handleSubmit} className={`space-y-6 ${className}`}>
            <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">General</h3>
                <p className="text-sm text-gray-500">
                    App-wide identity and the base URL used by every link-builder. These values are shared across
                    the product — the mail engine, activation flow, and public pages all read them from here.
                </p>
            </div>

            <FormInput
                id="base_url"
                type="url"
                label="Base URL"
                value={form.base_url}
                onChange={v => update({ base_url: v })}
                placeholder={import.meta.env.VITE_FRONTEND_URL ?? 'http://localhost:5173'}
                description={baseURLError ? undefined : 'Used as the prefix for activation links and any future public links. No trailing slash.'}
                error={baseURLError || undefined}
            />

            <FormInput
                id="page_title"
                label="Page title"
                value={form.page_title}
                onChange={v => update({ page_title: v })}
                placeholder="coco-iam"
                description="Product / instance name shown in the browser tab, invite emails, and the login screen."
            />

            <FormTextarea
                id="description"
                label="Description"
                value={form.description}
                onChange={v => update({ description: v })}
                rows={3}
                placeholder={`Short description shown in the HTML <meta name="description"> tag.`}
            />

            <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Robots metadata
                </label>
                <div className="flex gap-2 flex-wrap">
                    <select
                        value={ROBOTS_PRESETS.find(p => p.value === form.robots) ? form.robots : ''}
                        onChange={e => update({ robots: e.target.value })}
                        className="px-3 py-2 border border-gray-300 dark:border-surface-700 rounded-lg bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                    >
                        <option value="">— Custom —</option>
                        {ROBOTS_PRESETS.map(p => (
                            <option key={p.value} value={p.value}>{p.label}</option>
                        ))}
                    </select>
                    <input
                        type="text"
                        value={form.robots}
                        onChange={e => update({ robots: e.target.value })}
                        placeholder="index, follow"
                        className="flex-1 min-w-[240px] px-4 py-2 border border-gray-300 dark:border-surface-700 rounded-lg bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                    />
                </div>
                <p className="text-xs text-gray-500 mt-1">
                    Content of the <code>&lt;meta name=&quot;robots&quot;&gt;</code> tag. Leave blank for browser / crawler defaults.
                </p>
            </div>

            <div className="flex justify-end pt-2">
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminSettingsGeneralWrite, AppScopes.AdminSettingsGeneral, AppScopes.SuperAdmin]}>
                    <Submit loading={submitting} loadingText="Saving…" label="Save general settings" disabled={baseURLError !== ''} />
                </ScopeBasedComponentAccess>
            </div>
        </form>
    );
};

export default GeneralSettingsManager;
