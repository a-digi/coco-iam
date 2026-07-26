import React, { useState } from 'react';
import { Modal } from '../../../../Shared/Components/Modal/Modal';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';

interface RecoveryCodesResponse {
    recovery_codes: string[];
}

interface Props {
    action: 'disable' | 'regenerate';
    onCancel: () => void;
    onDisabled: () => void;
    onRegenerated: (recoveryCodes: string[]) => void;
}

// DisableMfaModal re-verifies the admin's current password before either
// disabling TOTP entirely (DELETE .../mfa) or regenerating recovery codes
// (POST .../mfa/recovery-codes) — both require the same re-auth step so a
// hijacked, still-logged-in session can't turn off the second factor.
export const DisableMfaModal: React.FC<Props> = ({ action, onCancel, onDisabled, onRegenerated }) => {
    const { del, post } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [password, setPassword] = useState('');
    const [submitting, setSubmitting] = useState(false);

    const submit = async () => {
        if (!password.trim()) return;
        setSubmitting(true);
        try {
            if (action === 'disable') {
                await del('admin/users/me/mfa', { body: JSON.stringify({ password }) });
                onDisabled();
            } else {
                const resp = await post<{ message: RecoveryCodesResponse }>(
                    'admin/users/me/mfa/recovery-codes',
                    { password },
                );
                onRegenerated(resp.message.recovery_codes);
            }
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Incorrect password.');
        } finally {
            setSubmitting(false);
        }
    };

    const title = action === 'disable'
        ? 'Disable two-factor authentication'
        : 'Regenerate recovery codes';
    const description = action === 'disable'
        ? 'Enter your current password to confirm disabling two-factor authentication.'
        : 'Enter your current password to confirm generating new recovery codes. Your existing codes will stop working immediately.';
    const confirmLabel = action === 'disable' ? 'Disable' : 'Regenerate';

    return (
        <Modal isOpen onClose={onCancel} title={title} closeOnBackdropClick={false}>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">{description}</p>
            <div className="max-w-xs">
                <FormInput
                    id="mfa-reauth-password"
                    label="Current password"
                    type="password"
                    value={password}
                    onChange={setPassword}
                    autoComplete="current-password"
                />
            </div>
            <div className="mt-4 flex items-center gap-3">
                <Submit type="button" onClick={() => void submit()} loading={submitting} label={confirmLabel} />
                <button
                    type="button"
                    onClick={onCancel}
                    className="text-xs font-medium text-gray-600 dark:text-gray-300 underline"
                >
                    Cancel
                </button>
            </div>
        </Modal>
    );
};

export default DisableMfaModal;
