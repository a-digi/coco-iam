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

// AddAllowlistEntryModal mirrors BanIPModal.tsx's shape. Fields match
// IPAllowlistRequest exactly — exempts ip from rate limiting and bans
// entirely (the relief valve for legitimate shared-IP traffic like
// NAT/office egress).
export const AddAllowlistEntryModal: React.FC<Props> = ({ isOpen, onClose, onCreated }) => {
    const { post } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [ip, setIp] = useState('');
    const [note, setNote] = useState('');
    const [loading, setLoading] = useState(false);

    const canSubmit = ip.trim() !== '';

    const reset = () => {
        setIp('');
        setNote('');
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!canSubmit) return;
        setLoading(true);
        try {
            await post('admin/security/ip-allowlist', {
                ip: ip.trim(),
                note: note.trim(),
            });
            reset();
            onCreated();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to add IP to the allowlist.');
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            isOpen={isOpen}
            onClose={() => (loading ? undefined : onClose())}
            title="Add an IP to the allowlist"
            maxWidth="sm"
            closeOnBackdropClick={!loading}
        >
            <form onSubmit={handleSubmit} className="space-y-5">
                <FormInput
                    id="allowlist-ip"
                    label="IP address"
                    value={ip}
                    onChange={setIp}
                    placeholder="203.0.113.7"
                />
                <FormInput
                    id="allowlist-note"
                    label="Note"
                    value={note}
                    onChange={setNote}
                    placeholder="e.g. office egress"
                />
                <div className="flex justify-end gap-2 pt-2">
                    <Cancel onClick={() => (loading ? undefined : onClose())} disabled={loading} />
                    <Submit loading={loading} label="Add" disabled={!canSubmit || loading} />
                </div>
            </form>
        </Modal>
    );
};

export default AddAllowlistEntryModal;
