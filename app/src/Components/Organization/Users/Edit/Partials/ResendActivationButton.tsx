import React, { useState } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { SubmitSmall } from '../../../../../Shared/Components/Button/SubmitSmall';
import { OrganizationUserResource } from '../../../model/organizationUser';

interface ResendActivationButtonProps {
    userId: string;
    activationPending: boolean;
    isActive: boolean;
    onSent?: () => void;
}

export const ResendActivationButton: React.FC<ResendActivationButtonProps> = ({
    userId,
    activationPending,
    isActive,
    onSent,
}) => {
    const { post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const [loading, setLoading] = useState(false);

    if (!activationPending && isActive) return null;

    const handleSend = async () => {
        setLoading(true);
        try {
            await post(
                `organizations/{${OrganizationUserResource}}/{id:${userId}}/resend-activation`,
                {},
            );
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
            {loading ? 'Sending…' : 'Resend Activation Email'}
        </SubmitSmall>
    );
};

export default ResendActivationButton;
