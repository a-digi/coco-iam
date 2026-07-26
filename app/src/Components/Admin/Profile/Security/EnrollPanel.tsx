import React, { useCallback, useEffect, useState } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';

interface EnrollResponse {
    secret: string;
    provisioning_uri: string;
}

interface ConfirmResponse {
    recovery_codes: string[];
}

interface Props {
    onEnrolled: (recoveryCodes: string[]) => void;
    onCancel: () => void;
}

// EnrollPanel starts a new TOTP enrollment on mount (POST .../enroll),
// shows the QR code + manual-entry secret, and confirms the admin's
// authenticator app produces a matching code (POST .../confirm). On
// success, hands the one-time recovery codes up to the parent, which
// is responsible for displaying them (RecoveryCodesModal).
export const EnrollPanel: React.FC<Props> = ({ onEnrolled, onCancel }) => {
    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [loading, setLoading] = useState(true);
    const [secret, setSecret] = useState('');
    const [provisioningUri, setProvisioningUri] = useState('');
    const [code, setCode] = useState('');
    const [confirming, setConfirming] = useState(false);

    const startEnroll = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await post<{ message: EnrollResponse }>('admin/users/me/mfa/enroll', {});
            setSecret(resp.message.secret);
            setProvisioningUri(resp.message.provisioning_uri);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to start enrollment.');
            onCancel();
        } finally {
            setLoading(false);
        }
    }, [post, errorMessage, onCancel]);

    useEffect(() => {
        void startEnroll();
    }, [startEnroll]);

    const copySecret = () => {
        void navigator.clipboard
            .writeText(secret)
            .then(() => successMessage('Secret copied.'))
            .catch(() => errorMessage('Could not copy secret.'));
    };

    const confirm = async () => {
        if (!code.trim()) return;
        setConfirming(true);
        try {
            const resp = await post<{ message: ConfirmResponse }>('admin/users/me/mfa/confirm', { code: code.trim() });
            onEnrolled(resp.message.recovery_codes);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Invalid code.');
        } finally {
            setConfirming(false);
        }
    };

    if (loading) {
        return <p className="text-sm text-gray-500 dark:text-gray-400">Generating your secret…</p>;
    }

    return (
        <div className="space-y-4">
            <div>
                <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    1. Scan this QR code with your authenticator app
                </p>
                <div className="inline-block p-3 bg-white rounded-lg border border-gray-200 dark:border-surface-700">
                    <QRCodeSVG value={provisioningUri} size={176} />
                </div>
            </div>

            <div>
                <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Or enter this code manually
                </p>
                <div className="flex items-center gap-2">
                    <code className="px-2 py-1 rounded bg-gray-50 dark:bg-surface-900 font-mono text-[0.8rem] break-all">
                        {secret}
                    </code>
                    <button type="button" onClick={copySecret} className="text-xs text-indigo-600 hover:underline">
                        Copy
                    </button>
                </div>
            </div>

            <div className="max-w-xs">
                <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    2. Enter the 6-digit code it shows
                </p>
                <FormInput
                    id="mfa-enroll-code"
                    value={code}
                    onChange={setCode}
                    placeholder="123456"
                    autoComplete="one-time-code"
                />
            </div>

            <div className="flex items-center gap-3">
                <Submit type="button" onClick={() => void confirm()} loading={confirming} label="Confirm" />
                <button
                    type="button"
                    onClick={onCancel}
                    className="text-xs font-medium text-gray-600 dark:text-gray-300 underline"
                >
                    Cancel
                </button>
            </div>
        </div>
    );
};

export default EnrollPanel;
