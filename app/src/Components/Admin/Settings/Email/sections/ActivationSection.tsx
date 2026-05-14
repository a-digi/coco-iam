import React, { useCallback, useState, type SyntheticEvent } from 'react';
import { Link } from 'react-router-dom';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../../Shared/Components/Button';
import ScopeBasedComponentAccess from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../../config/security/scopes';
import type { ActivationSettings } from '../model/emailSettings';
import { FormInput } from '../../../../../Shared/Components/Form';

interface Props {
    initial: ActivationSettings;
    onSaved: (next: ActivationSettings) => void;
}

/**
 * ActivationSection controls the activation-flow cadence knobs (link
 * lifetime + resend cooldown). The Base URL used when building the
 * activation link now lives under Admin Settings → General.
 */
export const ActivationSection: React.FC<Props> = ({ initial, onSaved }) => {
    const { patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [ttlHours, setTTLHours] = useState(initial.ttl_hours);
    const [cooldown, setCooldown] = useState(initial.resend_cooldown_seconds);
    const [submitting, setSubmitting] = useState(false);

    const handleSubmit = useCallback(async (e: SyntheticEvent) => {
        e.preventDefault();
        if (ttlHours < 1) {
            errorMessage('TTL must be at least 1 hour.');
            return;
        }
        if (cooldown < 0) {
            errorMessage('Resend cooldown cannot be negative.');
            return;
        }
        setSubmitting(true);
        try {
            const response = await patch<{ message?: { activation?: ActivationSettings } }>(
                'admin/mail/settings',
                {
                    activation: {
                        ttl_hours: ttlHours,
                        resend_cooldown_seconds: cooldown,
                    },
                },
            );
            const next = response?.message?.activation;
            if (next) {
                setTTLHours(next.ttl_hours);
                setCooldown(next.resend_cooldown_seconds);
                onSaved(next);
            }
            successMessage('Activation settings saved.');
        } catch (err: unknown) {
            let msg = 'Failed to save activation settings';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [ttlHours, cooldown, patch, successMessage, errorMessage, onSaved]);

    return (
        <form onSubmit={handleSubmit} className="space-y-5">
            <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Activation</h3>
                <p className="text-sm text-gray-500">
                    Controls the cadence of the activation flow. The link's base URL is shared with the rest of the
                    app and lives under{' '}
                    <Link to="/admin/settings/general" className="text-indigo-600 dark:text-indigo-400 underline">
                        Admin Settings → General
                    </Link>.
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
                    description="Default: 24. After this interval the activation link expires."
                />
                <FormInput
                    id="cooldown"
                    type="number"
                    label="Resend cooldown (seconds)"
                    value={cooldown}
                    onChange={v => setCooldown(parseInt(v, 10) || 0)}
                    min={0}
                    description="Default: 300. Minimum seconds between two resend-activation calls for the same user."
                />
            </div>

            <div className="flex justify-end pt-2">
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailSettingsWrite, AppScopes.AdminMailSettings, AppScopes.SuperAdmin]}>
                    <Submit loading={submitting} loadingText="Saving…" label="Save activation settings" />
                </ScopeBasedComponentAccess>
            </div>
        </form>
    );
};

export default ActivationSection;
