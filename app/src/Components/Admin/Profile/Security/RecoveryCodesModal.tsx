import React from 'react';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';

interface Props {
    codes: string[];
    onClose: () => void;
}

// One-time reveal for MFA recovery codes — modeled directly on
// ApiCredentials.tsx's CreatedSecretModal: a bespoke,
// backdrop-dismiss-resistant overlay (not the general-purpose
// dismissable Modal) with an amber "shown once" warning and a forced
// explicit acknowledgement button. The codes are never persisted or
// re-fetchable client-side — this is the only place they ever appear.
export const RecoveryCodesModal: React.FC<Props> = ({ codes, onClose }) => {
    const { successMessage, errorMessage } = useSnackBar();

    const copy = (value: string, label: string) => {
        void navigator.clipboard
            .writeText(value)
            .then(() => successMessage(`${label} copied.`))
            .catch(() => errorMessage(`Could not copy ${label}.`));
    };

    return (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" role="dialog" aria-modal="true">
            <div className="relative w-full sm:max-w-xl bg-white dark:bg-surface-800 rounded-xl shadow-2xl border border-gray-200 dark:border-surface-700">
                <div className="px-6 py-4 border-b border-gray-200 dark:border-surface-700">
                    <h3 className="text-lg font-bold text-gray-900 dark:text-gray-100">Save your recovery codes</h3>
                </div>
                <div className="px-6 py-4 space-y-4 text-sm text-gray-700 dark:text-gray-300">
                    <div className="rounded-md border border-amber-200 dark:border-amber-900/40 bg-amber-50 dark:bg-amber-900/20 px-3 py-2 text-amber-800 dark:text-amber-200">
                        This is the only time these <strong>recovery codes</strong> will be shown. Each one
                        can be used once, in place of a code from your authenticator app, if you lose access
                        to your device. Save them now — you will not be able to retrieve them later.
                    </div>
                    <div>
                        <div className="flex items-center justify-between mb-1">
                            <div className="text-xs font-semibold uppercase tracking-wide text-gray-500">Recovery codes</div>
                            <button
                                type="button"
                                onClick={() => copy(codes.join('\n'), 'Recovery codes')}
                                className="text-xs text-indigo-600 hover:underline"
                            >
                                Copy all
                            </button>
                        </div>
                        <div className="grid grid-cols-2 gap-2">
                            {codes.map(code => (
                                <div key={code} className="flex items-center gap-2">
                                    <code className="flex-1 px-2 py-1 rounded bg-gray-50 dark:bg-surface-900 font-mono text-[0.8rem] break-all">
                                        {code}
                                    </code>
                                    <button
                                        type="button"
                                        onClick={() => copy(code, 'Recovery code')}
                                        className="text-xs text-indigo-600 hover:underline"
                                    >
                                        Copy
                                    </button>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
                <div className="px-6 py-4 border-t border-gray-200 dark:border-surface-700 bg-gray-50 dark:bg-surface-900 rounded-b-xl flex justify-end">
                    <button
                        type="button"
                        onClick={onClose}
                        className="px-3 py-2 rounded-md bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-500"
                    >
                        I've saved my recovery codes
                    </button>
                </div>
            </div>
        </div>
    );
};

export default RecoveryCodesModal;
