import React, { useState } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { SubmitSmall } from '../../../../../Shared/Components/Button/SubmitSmall';

interface SendActivationButtonProps {
    userId: string;
    isActive: boolean;
    onSent?: () => void;
}

export const SendActivationButton: React.FC<SendActivationButtonProps> = ({ userId, isActive, onSent }) => {
    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const [loading, setLoading] = useState(false);

    if (isActive) return null;

    const handleSend = async () => {
        setLoading(true);
        try {
            await post(`admin/users/{id:${userId}}/send-activation`, {});
            successMessage('Activation email sent successfully!');
            onSent?.();
        } catch (err: unknown) {
            let msg = 'Failed to send activation email';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    };

    return (
        <SubmitSmall onClick={() => void handleSend()} disabled={loading}>
            {loading ? 'Sending...' : 'Send Activation Email'}
        </SubmitSmall>
    );
};

export default SendActivationButton;
