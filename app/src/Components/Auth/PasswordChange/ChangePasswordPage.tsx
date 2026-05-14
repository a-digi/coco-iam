import React, { useEffect, useState, type SyntheticEvent } from 'react';
import { Link } from 'react-router-dom';
import { post } from '../../../api/client';
import { get } from '../../../api/get';
import { Submit } from '../../../Shared/Components/Button';
import { FormInput } from '../../../Shared/Components/Form';
import { EMPTY_RULE_SET, type RuleSet } from '../../UserRules/model/userRules';
import { validatePassword } from '../../../config/validation/userRules';

const GENERIC_ERROR = 'Something went wrong. Please try again.';

type Step = 'verify' | 'set-password' | 'done';

const ChangePasswordPage: React.FC = () => {
    const [step, setStep] = useState<Step>('verify');
    const [currentPassword, setCurrentPassword] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [confirm, setConfirm] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [rules, setRules] = useState<RuleSet>(EMPTY_RULE_SET);

    // Fetch the rule set that applies to the current user once on mount.
    // Silent on failure — backend still enforces on the change endpoint.
    useEffect(() => {
        let cancelled = false;
        (async () => {
            try {
                const resp = await get('account/user-rules') as { message?: RuleSet };
                if (!cancelled && resp?.message) setRules(resp.message);
            } catch {
                // ignore — defaults are fine for the preview UI
            }
        })();
        return () => { cancelled = true; };
    }, []);

    const passwordViolations = validatePassword(rules.password, newPassword);

    const handleVerify = async (e: SyntheticEvent) => {
        e.preventDefault();
        setError(null);
        if (currentPassword === '') {
            setError('Please enter your current password.');
            return;
        }
        setLoading(true);
        try {
            await post<{ current_password: string }>('account/password/verify', {
                current_password: currentPassword,
            });
            setStep('set-password');
        } catch {
            setError(GENERIC_ERROR);
        } finally {
            setLoading(false);
        }
    };

    const handleChange = async (e: SyntheticEvent) => {
        e.preventDefault();
        setError(null);
        if (newPassword === currentPassword) {
            setError('New password must differ from the current one.');
            return;
        }
        if (newPassword !== confirm) {
            setError('Passwords do not match.');
            return;
        }
        if (passwordViolations.length > 0) {
            setError(passwordViolations.join(' '));
            return;
        }
        setLoading(true);
        try {
            await post<{ current_password: string; new_password: string }>(
                'account/password/change',
                {
                    current_password: currentPassword,
                    new_password: newPassword,
                },
            );
            setStep('done');
        } catch (err: unknown) {
            // Password-format errors come back verbatim from the backend
            // (too short, same as old); auth failures collapse to a
            // generic string server-side. Either way, just show what
            // the server sent — the server decides what to leak.
            let msg = GENERIC_ERROR;
            if (err instanceof Error && err.message) msg = err.message;
            setError(msg);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-surface-950 px-6 py-12">
            <div className="w-full max-w-md bg-white dark:bg-surface-900 rounded-xl shadow-sm border border-gray-200 dark:border-surface-800 p-8">
                <div className="text-center mb-6">
                    <div className="mx-auto w-14 h-14 rounded-2xl bg-indigo-600/10 border border-indigo-200 dark:border-indigo-800 flex items-center justify-center mb-4">
                        <svg className="w-7 h-7 text-indigo-600 dark:text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75M6.75 10.5h10.5a2.25 2.25 0 012.25 2.25v6a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 18.75v-6a2.25 2.25 0 012.25-2.25z" />
                        </svg>
                    </div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">Change your password</h1>
                    <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                        {step === 'verify' && 'Confirm your current password before choosing a new one.'}
                        {step === 'set-password' && 'Choose a new password. You will stay signed in.'}
                        {step === 'done' && 'Your password has been updated.'}
                    </p>
                </div>

                {step === 'done' && (
                    <div className="space-y-4">
                        <div className="rounded-lg border border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20 p-4 text-sm text-green-800 dark:text-green-200">
                            Your password was changed successfully.
                        </div>
                        <Link
                            to="/"
                            className="block w-full text-center px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium transition-colors"
                        >
                            Back to dashboard
                        </Link>
                    </div>
                )}

                {step === 'verify' && (
                    <form onSubmit={handleVerify} className="space-y-5">
                        <FormInput
                            id="current_password"
                            type="password"
                            label="Current password"
                            value={currentPassword}
                            onChange={setCurrentPassword}
                            required
                            autoComplete="current-password"
                        />

                        {error && (
                            <div className="rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-3 text-sm text-red-700 dark:text-red-300">
                                {error}
                            </div>
                        )}

                        <Submit
                            loading={loading}
                            loadingText="Verifying..."
                            label="Continue"
                            className="w-full"
                        />

                        <div className="text-center">
                            <Link
                                to="/"
                                className="text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors"
                            >
                                Cancel
                            </Link>
                        </div>
                    </form>
                )}

                {step === 'set-password' && (
                    <form onSubmit={handleChange} className="space-y-5">
                        <FormInput
                            id="current_password_locked"
                            type="password"
                            label="Current password"
                            value={currentPassword}
                            onChange={setCurrentPassword}
                            readOnly
                            autoComplete="current-password"
                        />

                        <FormInput
                            id="new_password"
                            type="password"
                            label="New password"
                            value={newPassword}
                            onChange={setNewPassword}
                            required
                            minLength={rules.password.min_length}
                            autoComplete="new-password"
                            description={`Minimum ${rules.password.min_length} characters. Must differ from the current password.`}
                        />

                        {newPassword.length > 0 && passwordViolations.length > 0 && (
                            <ul className="text-xs text-amber-600 dark:text-amber-400 list-disc ml-4 space-y-0.5">
                                {passwordViolations.map(v => <li key={v}>{v}</li>)}
                            </ul>
                        )}

                        <FormInput
                            id="confirm_password"
                            type="password"
                            label="Confirm new password"
                            value={confirm}
                            onChange={setConfirm}
                            required
                            minLength={8}
                            autoComplete="new-password"
                        />

                        {error && (
                            <div className="rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-3 text-sm text-red-700 dark:text-red-300">
                                {error}
                            </div>
                        )}

                        <Submit
                            loading={loading}
                            loadingText="Updating..."
                            label="Change password"
                            className="w-full"
                        />

                        <div className="text-center">
                            <button
                                type="button"
                                onClick={() => {
                                    setStep('verify');
                                    setNewPassword('');
                                    setConfirm('');
                                    setError(null);
                                }}
                                className="text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors"
                            >
                                Back
                            </button>
                        </div>
                    </form>
                )}
            </div>
        </div>
    );
};

export default ChangePasswordPage;
