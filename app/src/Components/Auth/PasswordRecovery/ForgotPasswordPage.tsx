import React, { useState, type SyntheticEvent } from 'react';
import { Link } from 'react-router-dom';
import { postPublicApi } from '../../../api/client';
import { Submit } from '../../../Shared/Components/Button';
import { FormInput } from '../../../Shared/Components/Form';
import { useAuth } from '../Guard/useAuth';

/**
 * ForgotPasswordPage collects an email and pings the public
 * /recovery/request endpoint. The response is always 200 — we can't
 * confirm to the user that their email is known, since that'd leak
 * account existence. Same "we sent you instructions" message shows
 * either way.
 */
const ForgotPasswordPage: React.FC = () => {
    const { authenticated } = useAuth();

    const [email, setEmail] = useState('');
    const [loading, setLoading] = useState(false);
    const [submitted, setSubmitted] = useState(false);

    const handleSubmit = async (e: SyntheticEvent) => {
        e.preventDefault();
        if (email.trim() === '') return;
        setLoading(true);
        try {
            await postPublicApi<{ email: string }>('recovery/request', { email: email.trim() });
        } catch {
            // Swallow — we always show the neutral confirmation.
        } finally {
            setLoading(false);
            setSubmitted(true);
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
                        Password recovery is for locked-out users. You can change your password from your
                        account menu without going through email.
                    </p>
                    <div className="mt-6 space-y-3">
                        <Link
                            to="/account/change-password"
                            className="block w-full text-center px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium transition-colors"
                        >
                            Change password
                        </Link>
                        <Link
                            to="/"
                            className="block text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors"
                        >
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
                            <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75M6.75 10.5h10.5a2.25 2.25 0 012.25 2.25v6a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 18.75v-6a2.25 2.25 0 012.25-2.25z" />
                        </svg>
                    </div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">Forgot your password?</h1>
                    <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                        Enter the email address associated with your account and we'll send you a reset link.
                    </p>
                </div>

                {submitted ? (
                    <div className="space-y-4">
                        <div className="rounded-lg border border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20 p-4 text-sm text-green-800 dark:text-green-200">
                            If an account with that email exists, we've sent a reset link. Check your inbox
                            (and spam folder) — the link expires after a short while.
                        </div>
                        <Link
                            to="/login"
                            className="block w-full text-center px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium transition-colors"
                        >
                            Back to login
                        </Link>
                    </div>
                ) : (
                    <form onSubmit={handleSubmit} className="space-y-5">
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

                        <Submit
                            loading={loading}
                            loadingText="Sending..."
                            label="Send reset link"
                            className="w-full"
                        />

                        <div className="text-center">
                            <Link
                                to="/login"
                                className="text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors"
                            >
                                Back to login
                            </Link>
                        </div>
                    </form>
                )}
            </div>
        </div>
    );
};

export default ForgotPasswordPage;
