import React, { useState } from 'react';
import { Modal } from '../../../../Shared/Components/Modal';
import { Submit, Cancel } from '../../../../Shared/Components/Button';
import { FormInput } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';

interface Props {
    isOpen: boolean;
    onClose: () => void;
    onCreated: () => void;
}

const DEFAULT_DURATION_MINUTES = 60;

// BanIPModal mirrors CreateApiCredentialModal.tsx's shape — a manual
// ban, tier "manual" on the backend (indistinguishable from an
// auto-ban once created). Fields match IPBanRequest exactly.
export const BanIPModal: React.FC<Props> = ({ isOpen, onClose, onCreated }) => {
    const { post } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [ip, setIp] = useState('');
    const [reason, setReason] = useState('');
    const [durationMinutes, setDurationMinutes] = useState(String(DEFAULT_DURATION_MINUTES));
    const [loading, setLoading] = useState(false);

    const canSubmit = ip.trim() !== '' && Number(durationMinutes) > 0;

    const reset = () => {
        setIp('');
        setReason('');
        setDurationMinutes(String(DEFAULT_DURATION_MINUTES));
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!canSubmit) return;
        setLoading(true);
        try {
            await post('admin/security/ip-bans', {
                ip: ip.trim(),
                reason: reason.trim(),
                duration_minutes: Number(durationMinutes),
            });
            reset();
            onCreated();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to ban IP.');
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            isOpen={isOpen}
            onClose={() => (loading ? undefined : onClose())}
            title="Manually ban an IP"
            maxWidth="sm"
            closeOnBackdropClick={!loading}
        >
            <form onSubmit={handleSubmit} className="space-y-5">
                <FormInput
                    id="ban-ip"
                    label="IP address"
                    value={ip}
                    onChange={setIp}
                    placeholder="203.0.113.7"
                />
                <FormInput
                    id="ban-reason"
                    label="Reason"
                    value={reason}
                    onChange={setReason}
                    placeholder="e.g. suspicious activity"
                />
                <FormInput
                    id="ban-duration"
                    label="Duration (minutes)"
                    type="number"
                    value={durationMinutes}
                    onChange={setDurationMinutes}
                    min={1}
                />
                <div className="flex justify-end gap-2 pt-2">
                    <Cancel onClick={() => (loading ? undefined : onClose())} disabled={loading} />
                    <Submit loading={loading} label="Ban" disabled={!canSubmit || loading} />
                </div>
            </form>
        </Modal>
    );
};

export default BanIPModal;
