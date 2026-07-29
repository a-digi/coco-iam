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

// Username-then-password, modeled after AppLoginPage.tsx's identical
// step pattern (that page already has the "identifier first, password
// next, with an eye-icon toggle" UX this mirrors — the Organization →
// Workspace → Application flow, by contrast, is just separate routed
// pages with no step state at all). Step 1 makes no backend call —
// admin login has no multi-method branching to discover (unlike
// AppLoginPage's auth-methods lookup), so there's nothing for a
// step-1 endpoint to accomplish beyond adding a username-enumeration
// surface. The existing admin/oauth/authenticate call still only
// fires once, from the password step.
type Step = 'identifier' | 'password' | 'mfa';

const AdminLogin: React.FC = () => {
  const [step, setStep] = useState<Step>('identifier');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [mfaToken, setMfaToken] = useState<string | null>(null);
  const [mfaCode, setMfaCode] = useState('');
  const [verifying, setVerifying] = useState(false);
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleContinue = (e: SyntheticEvent) => {
    e.preventDefault();
    if (!username.trim()) return;
    setError(null);
    setStep('password');
  };

  const backToIdentifier = () => {
    setStep('identifier');
    setPassword('');
    setShowPassword(false);
    setError(null);
  };

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
        setStep('mfa');
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
    setStep('password');
  };

  const eyeToggle = (
    <button
      type="button"
      onClick={() => setShowPassword(v => !v)}
      className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
      aria-label={showPassword ? 'Hide password' : 'Show password'}
      tabIndex={-1}
    >
      {showPassword ? (
        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88" />
        </svg>
      ) : (
        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
          <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
        </svg>
      )}
    </button>
  );

  return (
    <div className="min-h-screen grid grid-cols-1 md:grid-cols-2 bg-gray-50 dark:bg-surface-950">
      <div className="flex items-center justify-center px-6 py-12">
        {step === 'mfa' ? (
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
                autoFocus
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
        ) : step === 'password' ? (
          <form
            onSubmit={handleSubmit}
            className="w-full max-w-md space-y-6 bg-white dark:bg-surface-900 p-8"
          >
            <div className="text-center">
              <h2 className="text-2xl font-bold text-gray-800 dark:text-gray-100">Admin Login</h2>
              <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">Sign in to manage your workspaces.</p>
            </div>

            <div className="flex items-center justify-between gap-2 rounded-lg bg-gray-50 dark:bg-surface-800 border border-gray-200 dark:border-surface-700 px-3 py-2 text-sm text-gray-700 dark:text-gray-300">
              <div className="flex items-center gap-2 min-w-0">
                <svg className="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
                </svg>
                <span className="truncate font-medium">{username}</span>
              </div>
              <button
                type="button"
                onClick={backToIdentifier}
                className="text-xs font-medium text-indigo-600 dark:text-indigo-400 hover:underline shrink-0"
              >
                Not you?
              </button>
            </div>
            {/* Kept in the DOM (not just a visual label above) so
                password managers still pair the username with the
                password field for autofill, even though only one
                field is visible per step. */}
            <input type="text" value={username} autoComplete="username" readOnly hidden />

            <FormInput
                id="password"
                type={showPassword ? 'text' : 'password'}
                label="Password"
                value={password}
                onChange={setPassword}
                required
                autoComplete="current-password"
                autoFocus
                trailing={eyeToggle}
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
        ) : (
          <form
            onSubmit={handleContinue}
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
                autoFocus
            />

            <Submit
              label="Continue"
              className="w-full"
              disabled={!username.trim()}
            />
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
