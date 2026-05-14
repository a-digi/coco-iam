import React, { useMemo, useState, type SyntheticEvent } from 'react';
import { useSearchParams, Link, useNavigate } from 'react-router-dom';
import { postPublicApi } from '../../../api/client';
import { Submit } from '../../../Shared/Components/Button';
import { FormInput } from '../../../Shared/Components/Form';
import { useAuth } from '../Guard/useAuth';
import { EMPTY_RULE_SET, type RuleSet } from '../../UserRules/model/userRules';
import { validatePassword } from '../../../config/validation/userRules';
import type { AuthToken } from '../Guard/model/auth';
import { isSafeRedirectPath } from './isSafeRedirectPath';

const GENERIC_ERROR = 'Something went wrong. The activation link may be invalid, expired, or already used.';

type Step = 'verify' | 'set-password' | 'done';

interface VerifyResponse {
    message?: {
        ok?: boolean;
        rules?: RuleSet;
    };
}

interface ActivateResponse {
    message?: {
        token?: AuthToken;
        redirect_url?: string;
    };
}

const AdminActivatePage: React.FC = () => {
    const [params] = useSearchParams();
    const token = useMemo(() => (params.get('token') ?? '').trim(), [params]);
    const { login, authenticated } = useAuth();
    const navigate = useNavigate();

    const [step, setStep] = useState<Step>('verify');
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [confirm, setConfirm] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [rules, setRules] = useState<RuleSet>(EMPTY_RULE_SET);

    const missingToken = token === '';

    const handleVerify = async (e: SyntheticEvent) => {
        e.preventDefault();
        setError(null);
        if (missingToken) {
            setError(GENERIC_ERROR);
            return;
        }
        if (email.trim() === '') {
            setError('Email is required.');
            return;
        }
        setLoading(true);
        try {
            const resp = await postPublicApi<{ token: string; email: string }>(
                'activation/a/verify',
                { token, email: email.trim() },
            ) as VerifyResponse;
            if (resp?.message?.rules) setRules(resp.message.rules);
            setStep('set-password');
        } catch {
            setError(GENERIC_ERROR);
        } finally {
            setLoading(false);
        }
    };

    const passwordViolations = validatePassword(rules.password, password, { email });

    const handleActivate = async (e: SyntheticEvent) => {
        e.preventDefault();
        setError(null);
        if (password !== confirm) {
            setError('Passwords do not match.');
            return;
        }
        if (passwordViolations.length > 0) {
            setError(passwordViolations.join(' '));
            return;
        }
        setLoading(true);
        try {
            const resp = await postPublicApi<{ token: string; email: string; new_password: string }>(
                'activation/a/activate',
                { token, email: email.trim(), new_password: password },
            ) as ActivateResponse;

            const authToken = resp?.message?.token;
            if (authToken?.access_token) {
                login(authToken);
                const target = resp?.message?.redirect_url ?? '';
                navigate(isSafeRedirectPath(target) ? target : '/');
                return;
            }

            // Token absent means the account activated but has no ACL scopes yet.
            setStep('done');
        } catch {
            setError(GENERIC_ERROR);
        } finally {
            setLoading(false);
        }
    };

    if (authenticated) {
        return (
            <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-surface-950 px-6 py-12">
                <div className="w-full max-w-md bg-white dark:bg-surface-900 rounded-xl shadow-sm border border-gray-200 dark:border-surface-800 p-8 text-center">
                    <div className="mx-auto w-20 h-20 rounded-full bg-amber-100 dark:bg-amber-900/30 border border-amber-200 dark:border-amber-800 flex items-center justify-center mb-5">
                        <svg className="w-10 h-10 text-amber-600 dark:text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5} aria-hidden="true">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6A2.25 2.25 0 005.25 5.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15" />
                            <path strokeLinecap="round" strokeLinejoin="round" d="M12 12h9m0 0l-3-3m3 3l-3 3" />
                        </svg>
                    </div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">You're already signed in</h1>
                    <p className="mt-3 text-sm text-gray-500 dark:text-gray-400">
                        Activation links can only be used from a signed-out session. Please log out before activating a new account.
                    </p>
                    <div className="mt-6 space-y-3">
                        <Link to="/logout" className="block w-full text-center px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium transition-colors">
                            Log out
                        </Link>
                        <Link to="/" className="block text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors">
                            Back to dashboard
                        </Link>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-surface-950 px-6 py-12">
            <div className="w-full max-w-md bg-white dark:bg-surface-900 rounded-xl shadow-sm border border-gray-200 dark:border-surface-800 p-8">
                <div className="text-center mb-6">
                    <div className="mx-auto w-14 h-14 rounded-2xl bg-indigo-600/10 border border-indigo-200 dark:border-indigo-800 flex items-center justify-center mb-4">
                        <svg className="w-7 h-7 text-indigo-600 dark:text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M12 2l8 4v6c0 5-3.5 8.5-8 10-4.5-1.5-8-5-8-10V6l8-4z" />
                            <path strokeLinecap="round" strokeLinejoin="round" d="M9 12a3 3 0 116 0v2H9v-2zM10 14v3m4-3v3" />
                        </svg>
                    </div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">Activate your account</h1>
                    <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                        {step === 'verify' && 'Confirm the email address this invitation was sent to.'}
                        {step === 'set-password' && 'Choose a new password to finish activating your account.'}
                        {step === 'done' && 'Your account is ready.'}
                    </p>
                </div>

                {step === 'done' && (
                    <div className="space-y-4">
                        <div className="rounded-lg border border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20 p-4 text-sm text-green-800 dark:text-green-200">
                            Your account has been activated. Sign in with your new password.
                        </div>
                        <Link
                            to="/login"
                            className="block w-full text-center px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium transition-colors"
                        >
                            Go to login
                        </Link>
                    </div>
                )}

                {step === 'verify' && (
                    <form onSubmit={handleVerify} className="space-y-5">
                        <FormInput
                            id="email"
                            type="email"
                            label="Email address"
                            value={email}
                            onChange={setEmail}
                            required
                            autoComplete="email"
                            placeholder="you@example.com"
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
                            disabled={missingToken}
                        />

                        <div className="text-center">
                            <Link to="/login" className="text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors">
                                Back to login
                            </Link>
                        </div>
                    </form>
                )}

                {step === 'set-password' && (
                    <form onSubmit={handleActivate} className="space-y-5">
                        <FormInput
                            id="email-locked"
                            type="email"
                            label="Email address"
                            value={email}
                            onChange={setEmail}
                            readOnly
                            autoComplete="email"
                        />

                        <FormInput
                            id="new_password"
                            type="password"
                            label="New password"
                            value={password}
                            onChange={setPassword}
                            required
                            minLength={rules.password.min_length}
                            autoComplete="new-password"
                            description={`Minimum ${rules.password.min_length} characters.`}
                        />

                        {password.length > 0 && passwordViolations.length > 0 && (
                            <ul className="text-xs text-amber-600 dark:text-amber-400 list-disc ml-4 space-y-0.5">
                                {passwordViolations.map(v => <li key={v}>{v}</li>)}
                            </ul>
                        )}

                        <FormInput
                            id="confirm_password"
                            type="password"
                            label="Confirm password"
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
                            loadingText="Activating..."
                            label="Activate account"
                            className="w-full"
                        />

                        <div className="text-center">
                            <button
                                type="button"
                                onClick={() => { setStep('verify'); setPassword(''); setConfirm(''); setError(null); }}
                                className="text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors"
                            >
                                Use a different email
                            </button>
                        </div>
                    </form>
                )}
            </div>
        </div>
    );
};

export default AdminActivatePage;
