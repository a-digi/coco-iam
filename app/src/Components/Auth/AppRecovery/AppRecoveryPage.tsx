import React, { useCallback, useEffect, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { get } from '../../../api/get';
import { postPublicApi, absolutePublicURL } from '../../../api/client';
import type { PublicLoginConfig } from '../../AppLogin/model/loginTemplate';

type Mode = 'request' | 'reset';

interface FetchResponse {
  message?: PublicLoginConfig;
}

interface Props {
  mode: Mode;
}

/**
 * AppRecoveryPage renders either the recovery-request form (enter
 * email) or the reset form (enter new password). Shares the login
 * template's background/logo/title/brand for visual consistency;
 * layout is always centered 1-column (recovery doesn't need a
 * two-column text panel).
 *
 * The backend enforces `allow_recovery`: request is silent on
 * success, reset returns a generic 400 if recovery is off.
 */
const AppRecoveryPage: React.FC<Props> = ({ mode }) => {
  const { org = '', ws = '', app = '' } = useParams<{ org: string; ws: string; app: string }>();
  const [params] = useSearchParams();
  const token = params.get('token') ?? '';
  const emailFromUrl = params.get('email') ?? '';

  const [config, setConfig] = useState<PublicLoginConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [done, setDone] = useState(false);

  const fetchConfig = useCallback(async () => {
    if (!org || !ws || !app) {
      setNotFound(true);
      setLoading(false);
      return;
    }
    try {
      const resp = (await get(
        `public/applications/login-template?org=${encodeURIComponent(org)}&ws=${encodeURIComponent(ws)}&app=${encodeURIComponent(app)}`,
      )) as FetchResponse;
      if (resp?.message?.template_kind) {
        setConfig(resp.message);
      } else {
        setNotFound(true);
      }
    } catch {
      setNotFound(true);
    } finally {
      setLoading(false);
    }
  }, [org, ws, app]);

  useEffect(() => {
    void fetchConfig();
  }, [fetchConfig]);

  if (loading) return <div className="p-8 text-gray-500">Loading…</div>;
  if (notFound || !config) return <div className="p-8 text-gray-500">Page not found.</div>;

  const backgroundURL = config.background_url ? absolutePublicURL(config.background_url) : '';
  const logoURL = config.logo_url ? absolutePublicURL(config.logo_url) : '';

  const wrapperStyle: React.CSSProperties = {
    backgroundColor: config.background_color,
    ...(backgroundURL
      ? {
          backgroundImage: `url(${backgroundURL})`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
        }
      : {}),
  };

  return (
    <div className="min-h-screen w-full" style={wrapperStyle}>
      <div className="min-h-screen w-full flex items-center justify-center p-6">
        <div className="w-full max-w-sm bg-white/95 backdrop-blur rounded-2xl shadow-xl p-8">
          <div className="flex flex-col items-center text-center gap-2">
            {config.show_logo && logoURL && (
              <img src={logoURL} alt="" className="h-14 w-14 object-contain" />
            )}
            {config.page_title && <h1 className="text-xl font-bold text-gray-900">{config.page_title}</h1>}
            {config.brand_text && <p className="text-sm text-gray-500">{config.brand_text}</p>}
          </div>

          <div className="mt-6">
            {mode === 'request' ? (
              <RequestForm org={org} ws={ws} app={app} done={done} setDone={setDone} />
            ) : (
              <ResetForm
                org={org}
                ws={ws}
                app={app}
                token={token}
                emailFromUrl={emailFromUrl}
                done={done}
                setDone={setDone}
              />
            )}
          </div>

          <div className="mt-6 text-center text-xs">
            <Link
              to={`/login/a/${encodeURIComponent(org)}/${encodeURIComponent(ws)}/${encodeURIComponent(app)}`}
              className="text-indigo-600 hover:underline"
            >
              ← Back to sign in
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
};

const RequestForm: React.FC<{
  org: string;
  ws: string;
  app: string;
  done: boolean;
  setDone: (v: boolean) => void;
}> = ({ org, ws, app, done, setDone }) => {
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);

  if (done) {
    return (
      <div className="text-sm text-gray-700 text-center">
        If an account exists for that address, a reset link is on its way.
      </div>
    );
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await postPublicApi(
        `public/applications/recover/request?org=${encodeURIComponent(org)}&ws=${encodeURIComponent(ws)}&app=${encodeURIComponent(app)}`,
        { email },
      );
    } catch {
      // Silent by design — the endpoint returns 200 regardless.
    } finally {
      setBusy(false);
      setDone(true);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-3">
      <div>
        <label className="block text-xs font-medium text-gray-600 mb-1" htmlFor="coco-email">
          Email
        </label>
        <input
          id="coco-email"
          type="email"
          value={email}
          onChange={e => setEmail(e.target.value)}
          required
          autoComplete="email"
          className="w-full px-3 py-2 rounded-md border border-gray-300 text-gray-900 focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
      </div>
      <button
        type="submit"
        disabled={busy}
        className="w-full py-2 rounded-md bg-indigo-600 text-white font-semibold hover:bg-indigo-500 disabled:opacity-60"
      >
        {busy ? 'Sending…' : 'Send reset link'}
      </button>
    </form>
  );
};

const ResetForm: React.FC<{
  org: string;
  ws: string;
  app: string;
  token: string;
  emailFromUrl: string;
  done: boolean;
  setDone: (v: boolean) => void;
}> = ({ org, ws, app, token, emailFromUrl, done, setDone }) => {
  const [email, setEmail] = useState(emailFromUrl);
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (done) {
    return (
      <div className="text-sm text-gray-700 text-center">
        Password updated. You can now sign in with your new password.
      </div>
    );
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await postPublicApi(
        `public/applications/recover/reset?org=${encodeURIComponent(org)}&ws=${encodeURIComponent(ws)}&app=${encodeURIComponent(app)}`,
        { token, email, password },
      );
      setDone(true);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to reset password');
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-3">
      <div>
        <label className="block text-xs font-medium text-gray-600 mb-1" htmlFor="coco-reset-email">
          Email
        </label>
        <input
          id="coco-reset-email"
          type="email"
          value={email}
          onChange={e => setEmail(e.target.value)}
          required
          autoComplete="email"
          className="w-full px-3 py-2 rounded-md border border-gray-300 text-gray-900 focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
      </div>
      <div>
        <label className="block text-xs font-medium text-gray-600 mb-1" htmlFor="coco-new-password">
          New password
        </label>
        <input
          id="coco-new-password"
          type="password"
          value={password}
          onChange={e => setPassword(e.target.value)}
          required
          autoComplete="new-password"
          className="w-full px-3 py-2 rounded-md border border-gray-300 text-gray-900 focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
      </div>
      {error && (
        <div className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-md p-2">
          {error}
        </div>
      )}
      <button
        type="submit"
        disabled={busy}
        className="w-full py-2 rounded-md bg-indigo-600 text-white font-semibold hover:bg-indigo-500 disabled:opacity-60"
      >
        {busy ? 'Saving…' : 'Reset password'}
      </button>
    </form>
  );
};

export default AppRecoveryPage;
