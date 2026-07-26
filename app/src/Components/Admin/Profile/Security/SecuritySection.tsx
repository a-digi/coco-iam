import React, { useCallback, useEffect, useState } from 'react';
import { Submit } from '../../../../Shared/Components/Button';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import EnrollPanel from './EnrollPanel';
import RecoveryCodesModal from './RecoveryCodesModal';
import DisableMfaModal from './DisableMfaModal';

interface MfaStatus {
    enabled: boolean;
    enrolled_at?: string;
    recovery_codes_remaining: number;
}

type ViewMode = 'status' | 'enrolling' | 'disabling' | 'regenerating';

// SecuritySection is the "Security" tab of the profile section
// (rendered inside ProfilePageShell — see routes.tsx). Reads
// GET /api/v1/admin/users/me/mfa on mount and renders the
// enable/disable flow. The enroll/disable/regenerate sub-flows are
// built out in later steps of plan/admin-mfa-totp/frontend-plan.md —
// this step wires the status fetch and the two top-level views.
export const SecuritySection: React.FC = () => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [status, setStatus] = useState<MfaStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [mode, setMode] = useState<ViewMode>('status');
    const [pendingRecoveryCodes, setPendingRecoveryCodes] = useState<string[] | null>(null);

    const loadStatus = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<{ message: MfaStatus }>('admin/users/me/mfa');
            setStatus(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load MFA status.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void loadStatus();
    }, [loadStatus]);

    if (loading && !status) {
        return <div className="text-sm text-gray-500 dark:text-gray-400 p-4">Loading security settings…</div>;
    }

    if (!status) {
        return null;
    }

    return (
        <div className="max-w-2xl">
            <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-1">
                Two-factor authentication
            </h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                Require a code from an authenticator app (Google Authenticator, Authy, 1Password, etc.)
                in addition to your password when signing in.
            </p>

            {mode === 'status' && !status.enabled && (
                <div className="rounded-lg border border-gray-200 dark:border-surface-700 p-4">
                    <div className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                        <span className="inline-block w-2 h-2 rounded-full bg-gray-400" />
                        Not enabled
                    </div>
                    <Submit
                        type="button"
                        onClick={() => setMode('enrolling')}
                        label="Enable two-factor authentication"
                    />
                </div>
            )}

            {mode === 'status' && status.enabled && (
                <div className="rounded-lg border border-gray-200 dark:border-surface-700 p-4">
                    <div className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        <span className="inline-block w-2 h-2 rounded-full bg-green-500" />
                        Enabled
                    </div>
                    {status.enrolled_at && (
                        <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">
                            Since {new Date(status.enrolled_at).toLocaleDateString()}
                        </p>
                    )}
                    <p className="text-xs text-gray-500 dark:text-gray-400 mb-4">
                        {status.recovery_codes_remaining} recovery code{status.recovery_codes_remaining === 1 ? '' : 's'} remaining
                    </p>
                    <div className="flex items-center gap-2">
                        <button
                            type="button"
                            onClick={() => setMode('regenerating')}
                            className="inline-flex items-center px-3 py-1.5 text-xs font-medium rounded-md border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-800 hover:bg-gray-50 dark:hover:bg-surface-700"
                        >
                            Regenerate recovery codes
                        </button>
                        <button
                            type="button"
                            onClick={() => setMode('disabling')}
                            className="px-3 py-1.5 text-xs font-medium rounded-md border border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/50 dark:text-red-300 dark:hover:bg-red-900/30"
                        >
                            Disable
                        </button>
                    </div>
                </div>
            )}

            {mode === 'enrolling' && (
                <div className="rounded-lg border border-gray-200 dark:border-surface-700 p-4">
                    <EnrollPanel
                        onEnrolled={codes => {
                            setPendingRecoveryCodes(codes);
                            setMode('status');
                            void loadStatus();
                        }}
                        onCancel={() => setMode('status')}
                    />
                </div>
            )}

            {(mode === 'disabling' || mode === 'regenerating') && (
                <DisableMfaModal
                    action={mode === 'disabling' ? 'disable' : 'regenerate'}
                    onCancel={() => setMode('status')}
                    onDisabled={() => {
                        setMode('status');
                        void loadStatus();
                    }}
                    onRegenerated={codes => {
                        setPendingRecoveryCodes(codes);
                        setMode('status');
                        void loadStatus();
                    }}
                />
            )}

            {pendingRecoveryCodes && (
                <RecoveryCodesModal
                    codes={pendingRecoveryCodes}
                    onClose={() => setPendingRecoveryCodes(null)}
                />
            )}
        </div>
    );
};

export default SecuritySection;
