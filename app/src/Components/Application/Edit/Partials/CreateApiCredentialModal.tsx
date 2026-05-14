import React, { useMemo, useState } from 'react';
import { Modal } from '../../../../Shared/Components/Modal';
import { Submit, Cancel } from '../../../../Shared/Components/Button';
import { FormInput } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { ApplicationResource } from '../../model/application';
import type { ApiCredential } from './ApiCredentials';

interface Props {
    isOpen: boolean;
    onClose: () => void;
    applicationId: string;
    onCreated: (created: CreatedCredential) => void;
}

export interface CreatedCredential {
    credential: ApiCredential;
    api_secret: string;
    clamped?: boolean;
}

interface CreateResponse {
    message?: CreatedCredential;
}

// ALL_PURPOSES mirrors the backend validatePurposes whitelist. Adding
// a new machine-auth purpose requires updating this list + the
// purpose/purpose.go constant in lockstep.
const ALL_PURPOSES: Array<{ id: string; label: string; description: string }> = [
    {
        id: 'security_key:read',
        label: 'Security key — read',
        description: 'Read this application’s public and private signing keys over /a/.../security-key.',
    },
];

const DEFAULT_EXPIRY_DAYS = 90;
const MAX_EXPIRY_DAYS = 365;

// addDaysAsDateInputValue returns an ISO-like `YYYY-MM-DD` for
// <input type="date">, `days` days after today. Time is stripped
// because the HTML date input ignores it.
const addDaysAsDateInputValue = (days: number, now = new Date()): string => {
    const d = new Date(now);
    d.setDate(d.getDate() + days);
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
};

// dateInputToExpiresAt turns a YYYY-MM-DD picker value into an
// RFC3339 timestamp at end-of-day local time. End-of-day rather than
// midnight so the credential is valid for the whole picked date.
const dateInputToExpiresAt = (value: string): string => {
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return '';
    d.setHours(23, 59, 59, 999);
    return d.toISOString();
};

export const CreateApiCredentialModal: React.FC<Props> = ({
    isOpen,
    onClose,
    applicationId,
    onCreated,
}) => {
    const { post } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const todayInputMin = useMemo(() => addDaysAsDateInputValue(1), []);
    const maxInput = useMemo(() => addDaysAsDateInputValue(MAX_EXPIRY_DAYS), []);
    const [label, setLabel] = useState('');
    const [purposes, setPurposes] = useState<string[]>([]);
    const [expiresDate, setExpiresDate] = useState<string>(addDaysAsDateInputValue(DEFAULT_EXPIRY_DAYS));
    const [loading, setLoading] = useState(false);

    const togglePurpose = (id: string) => {
        setPurposes(prev => (prev.includes(id) ? prev.filter(p => p !== id) : [...prev, id]));
    };

    const canSubmit = purposes.length > 0 && expiresDate !== '';

    const reset = () => {
        setLabel('');
        setPurposes([]);
        setExpiresDate(addDaysAsDateInputValue(DEFAULT_EXPIRY_DAYS));
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!canSubmit) return;
        setLoading(true);
        try {
            const resp = await post<CreateResponse>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/api-credentials`,
                {
                    label,
                    purposes,
                    expires_at: dateInputToExpiresAt(expiresDate),
                },
            );
            const body = resp?.message;
            if (!body || !body.credential || !body.api_secret) {
                throw new Error('Unexpected response shape');
            }
            reset();
            onCreated(body);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to create credential');
        } finally {
            setLoading(false);
        }
    };

    return (
        <Modal
            isOpen={isOpen}
            onClose={() => (loading ? undefined : onClose())}
            title="Create API credential"
            maxWidth="lg"
            closeOnBackdropClick={!loading}
        >
            <form onSubmit={handleSubmit} className="space-y-5">
                <FormInput
                    id="label"
                    label="Label"
                    value={label}
                    onChange={setLabel}
                    placeholder="e.g. Production signing service"
                />

                <div>
                    <div className="text-sm font-medium text-gray-800 dark:text-gray-100 mb-2">Purposes</div>
                    <p className="text-xs text-gray-500 mb-3">
                        Pick the specific actions this credential will be allowed to perform. Pick as few as possible.
                    </p>
                    <div className="space-y-2">
                        {ALL_PURPOSES.map(p => (
                            <label
                                key={p.id}
                                className="flex items-start gap-2 rounded-md border border-gray-200 dark:border-surface-700 px-3 py-2 cursor-pointer"
                            >
                                <input
                                    type="checkbox"
                                    className="mt-1 accent-indigo-600"
                                    checked={purposes.includes(p.id)}
                                    onChange={() => togglePurpose(p.id)}
                                />
                                <div>
                                    <div className="text-sm font-medium text-gray-800 dark:text-gray-100">{p.label}</div>
                                    <div className="text-xs text-gray-500">
                                        <code className="font-mono text-[0.7rem] mr-2">{p.id}</code>
                                        {p.description}
                                    </div>
                                </div>
                            </label>
                        ))}
                    </div>
                </div>

                <div>
                    <label htmlFor="expires" className="block text-sm font-medium text-gray-800 dark:text-gray-100 mb-1">
                        Expires on
                    </label>
                    <input
                        id="expires"
                        type="date"
                        value={expiresDate}
                        min={todayInputMin}
                        max={maxInput}
                        onChange={e => setExpiresDate(e.target.value)}
                        className="w-full h-10 px-3 rounded-md border border-gray-300 dark:border-surface-700 bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                        Maximum 1 year from today. Defaults to 90 days.
                    </p>
                </div>

                <div className="flex justify-end gap-2 pt-2">
                    <Cancel onClick={() => (loading ? undefined : onClose())} disabled={loading} />
                    <Submit
                        loading={loading}
                        label="Create"
                        disabled={!canSubmit || loading}
                    />
                </div>
            </form>
        </Modal>
    );
};

export default CreateApiCredentialModal;
