import React, { useState, type SyntheticEvent } from 'react';
import { post } from '../../../../api/client';
import { useAuth } from '../../Guard/useAuth';
import { Link, useNavigate } from 'react-router-dom';
import type { AuthResponse } from '../../Guard/model/auth';
import { Submit } from '../../../../Shared/Components/Button';
import { FeatureBullet } from '../../../../Shared/Components/Checkbox/FeatureBullet/FeatureBullet';
import { FormInput } from '../../../../Shared/Components/Form';

// Shape of the admin/oauth/authenticate response when the admin has
// TOTP MFA enabled — a 202 carrying a short-lived, narrowly-scoped
// mfa_token instead of a full access token. See plan/admin-mfa-totp/plan.md.
interface MfaRequiredMessage {
  mfa_required?: boolean;
  mfa_token?: string;
}

const AdminLogin: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [mfaToken, setMfaToken] = useState<string | null>(null);
  const [mfaCode, setMfaCode] = useState('');
  const [verifying, setVerifying] = useState(false);
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: SyntheticEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const data = { username, password };
      const response = await post('admin/oauth/authenticate', data) as AuthResponse & {
        message: MfaRequiredMessage;
      };

      if (response?.message?.mfa_required && response.message.mfa_token) {
        setMfaToken(response.message.mfa_token);
      } else if (response && response.message && response.message.access_token) {
        login(response.message);
        navigate('/');
      } else {
        setError('Invalid response from server');
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message || 'Login failed');
      } else {
        setError('Login failed');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleVerifyMfa = async (e: SyntheticEvent) => {
    e.preventDefault();
    if (!mfaToken) return;
    setError(null);
    setVerifying(true);
    try {
      const response = await post(
        'admin/oauth/verify-mfa',
        { code: mfaCode },
        { headers: { Authorization: `Bearer ${mfaToken}` } },
      ) as AuthResponse;

      if (response && response.message && response.message.access_token) {
        login(response.message);
        navigate('/');
      } else {
        setError('Invalid response from server');
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message || 'Verification failed' : 'Verification failed');
    } finally {
      setVerifying(false);
    }
  };

  const backToLogin = () => {
    setMfaToken(null);
    setMfaCode('');
    setError(null);
  };

  return (
    <div className="min-h-screen grid grid-cols-1 md:grid-cols-2 bg-gray-50 dark:bg-surface-950">
      <div className="flex items-center justify-center px-6 py-12">
        {mfaToken ? (
          <form
            onSubmit={handleVerifyMfa}
            className="w-full max-w-md space-y-6 bg-white dark:bg-surface-900 p-8"
          >
            <div className="text-center">
              <h2 className="text-2xl font-bold text-gray-800 dark:text-gray-100">Two-factor authentication</h2>
              <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                Enter the 6-digit code from your authenticator app, or one of your recovery codes.
              </p>
            </div>

            <FormInput
                id="mfa-code"
                label="Code"
                value={mfaCode}
                onChange={setMfaCode}
                placeholder="123456"
                required
                autoComplete="one-time-code"
            />

            {error && (
              <div className="text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded p-2 text-center">
                {error}
              </div>
            )}

            <Submit
              loading={verifying}
              loadingText="Verifying..."
              label="Verify"
              className="w-full"
            />

            <div className="flex justify-start">
              <button
                type="button"
                onClick={backToLogin}
                className="text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors"
              >
                Back to login
              </button>
            </div>
          </form>
        ) : (
          <form
            onSubmit={handleSubmit}
            className="w-full max-w-md space-y-6 bg-white dark:bg-surface-900 p-8"
          >
            <div className="text-center">
              <h2 className="text-2xl font-bold text-gray-800 dark:text-gray-100">Admin Login</h2>
              <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">Sign in to manage your workspaces.</p>
            </div>

            <FormInput
                id="username"
                label="Username"
                value={username}
                onChange={setUsername}
                required
                autoComplete="username"
            />

            <FormInput
                id="password"
                type="password"
                label="Password"
                value={password}
                onChange={setPassword}
                required
                autoComplete="current-password"
            />

            {error && (
              <div className="text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded p-2 text-center">
                {error}
              </div>
            )}

            <Submit
              loading={loading}
              loadingText="Logging in..."
              label="Login"
              className="w-full"
            />

            <div className="flex justify-start">
              <Link
                to="/forgot-password"
                className="text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors"
              >
                Forgot password?
              </Link>
            </div>
          </form>
        )}
      </div>

      <div className="hidden md:flex relative items-center justify-center bg-gradient-to-br from-indigo-600 via-indigo-500 to-purple-600 overflow-hidden">
        <div
          aria-hidden="true"
          className="absolute inset-0 opacity-20"
          style={{
            backgroundImage:
              'radial-gradient(circle at 20% 20%, rgba(255,255,255,0.4), transparent 40%), radial-gradient(circle at 80% 70%, rgba(255,255,255,0.3), transparent 35%)',
          }}
        />
        <div className="relative z-10 max-w-md px-10 text-white">
          <div className="flex items-center gap-3 mb-8">
            <div className="w-14 h-14 rounded-2xl bg-white/15 backdrop-blur-sm border border-white/30 flex items-center justify-center">
              <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 2l8 4v6c0 5-3.5 8.5-8 10-4.5-1.5-8-5-8-10V6l8-4z" />
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12a3 3 0 116 0v2H9v-2zM10 14v3m4-3v3" />
              </svg>
            </div>
            <div>
              <div className="text-2xl font-bold leading-none">coco-iam</div>
              <div className="text-xs uppercase tracking-widest text-indigo-100">Identity &amp; Access</div>
            </div>
          </div>

          <h1 className="text-3xl font-semibold leading-tight">
            One place for every identity, workspace, and permission.
          </h1>
          <p className="mt-4 text-indigo-100">
            Scope-based access, organisation-aware users, and first-class applications — built for teams that need
            precise control without the ceremony.
          </p>

          <ul className="mt-8 space-y-3 text-sm text-indigo-50">
            <FeatureBullet>Fine-grained scopes per application</FeatureBullet>
            <FeatureBullet>Organisation &amp; workspace isolation</FeatureBullet>
            <FeatureBullet>Durable task queue for lifecycle events</FeatureBullet>
          </ul>
        </div>
      </div>
    </div>
  );
};

export default AdminLogin;
