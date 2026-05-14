import React, { useCallback, useEffect, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { get } from '../../../api/get';
import { postPublicApi, absolutePublicURL } from '../../../api/client';
import type { PublicLoginConfig } from '../../AppLogin/model/loginTemplate';
import AppLoginRedirect from './AppLoginRedirect';

interface FetchResponse {
  message?: PublicLoginConfig;
}

// authenticate response shapes:
//   - manual dispatch: { redirect_url } — backend already called the callback
//   - oauth client:    { redirect_url, code } — frontend appends ?code= and navigates
interface AuthResponseShape {
  message?: { redirect_url?: string; code?: string };
}

/**
 * AppLoginPage renders the per-application login page from typed
 * settings — no HTML injection, no DOMPurify. Admins pick one of
 * three layouts; this component dispatches on `config.template_kind`.
 *
 * This flow is intentionally disjoint from the admin auth context
 * (useAuth / AuthProvider / AUTH_TOKEN_KEY in localStorage). The
 * admin login populates that context to drive the coco-iam admin UI;
 * workspace-application login does not — no bearer token is ever
 * stored by this component. On success the browser is handed off to
 * the target application via <AppLoginRedirect>.
 */
// safeReturnTo validates that a return_to value is a same-origin
// relative path starting with /a/ (the OAuth server prefix). Any
// other value is discarded to prevent open-redirect attacks.
function safeReturnTo(raw: string | null): string | null {
  if (!raw) return null;
  try {
    const decoded = decodeURIComponent(raw);
    // Must be a relative path — no scheme, no host.
    if (decoded.startsWith('/a/')) return decoded;
  } catch {
    // Malformed encoding — ignore.
  }
  return null;
}

const AppLoginPage: React.FC = () => {
  const { org = '', ws = '', app = '' } = useParams<{ org: string; ws: string; app: string }>();
  const [searchParams] = useSearchParams();
  const returnTo = safeReturnTo(searchParams.get('return_to'));

  const [config, setConfig] = useState<PublicLoginConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [redirectURL, setRedirectURL] = useState<string | null>(null);

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

  if (redirectURL) {
    return (
      <AppLoginRedirect redirectURL={redirectURL} applicationName={config?.application_name} />
    );
  }
  if (loading) {
    return (
      <div className="min-h-screen w-full flex items-center justify-center bg-gray-50 dark:bg-surface-900">
        <div className="flex flex-col items-center gap-3 text-gray-500">
          <div className="h-8 w-8 rounded-full border-2 border-gray-200 border-t-indigo-600 animate-spin" />
          <span className="text-sm">Loading…</span>
        </div>
      </div>
    );
  }
  if (notFound || !config) {
    return (
      <div className="min-h-screen w-full flex items-center justify-center bg-gray-50 dark:bg-surface-900 p-8">
        <div className="max-w-md text-center">
          <div className="text-5xl mb-3">🔒</div>
          <h1 className="text-2xl font-bold text-gray-800 dark:text-gray-100 mb-2">Login page not found</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            The URL you followed no longer points at a configured application. Please contact your administrator.
          </p>
        </div>
      </div>
    );
  }

  const handleLogin = async (username: string, password: string): Promise<string | null> => {
    try {
      const returnToParam = returnTo ? `&return_to=${encodeURIComponent(returnTo)}` : '';
      const resp = (await postPublicApi<{ username: string; password: string }>(
        `applications/authenticate?org=${encodeURIComponent(org)}&ws=${encodeURIComponent(ws)}&app=${encodeURIComponent(app)}${returnToParam}`,
        { username, password },
      )) as AuthResponseShape;
      const msg = resp?.message;
      const redirectBase = msg?.redirect_url;
      if (redirectBase) {
        // OAuth client dispatch: backend returns the base redirect URI
        // and the authorization code separately. The frontend appends
        // ?code= here so the /a/…/oauth/authorize backend route is
        // never in the browser navigation path.
        if (msg?.code) {
          const u = new URL(redirectBase);
          u.searchParams.set('code', msg.code);
          setRedirectURL(u.toString());
          return null;
        }
        // All other cases (return_to OAuth flow processed inline by
        // the backend, or manual server-to-server dispatch): redirect_url
        // is the final destination.
        setRedirectURL(redirectBase);
        return null;
      }
      return 'Invalid credentials';
    } catch (err: unknown) {
      return err instanceof Error ? err.message : 'Login failed';
    }
  };

  // Backend returns relative /p/... paths; turn them into absolute
  // URLs pointing at the API origin so they resolve regardless of
  // where the frontend is served from.
  const backgroundURL = config.background_url ? absolutePublicURL(config.background_url) : '';
  const logoURL = config.logo_url ? absolutePublicURL(config.logo_url) : '';

  const hasImage = Boolean(backgroundURL);
  const hasGradient = !hasImage && Boolean(config.background_gradient);
  // Background precedence (highest first): image, gradient, solid
  // color. Both image and gradient drop through background-image; the
  // solid color remains as a fallback so old browsers / blocked image
  // loads still render something legible.
  const wrapperStyle: React.CSSProperties = {
    backgroundColor: config.background_color,
    ...(hasImage
      ? {
          backgroundImage: `url(${backgroundURL})`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          backgroundRepeat: 'no-repeat',
        }
      : hasGradient
        ? { backgroundImage: config.background_gradient }
        : {}),
  };

  // columnStyle composes the style for one visual column from its
  // admin-set overrides. Unset fields fall through (no key written
  // to the style object) so the wrapper background shows through —
  // this mirrors the server-side inheritance semantics. Precedence:
  // image > gradient > colour, same rule as the wrapper.
  const columnStyle = (index: number): { style: React.CSSProperties; hasImage: boolean; textColor?: string } => {
    const col = config.columns?.find(c => c.column_index === index);
    if (!col) return { style: {}, hasImage: false };
    const style: React.CSSProperties = {};
    const url = col.background_url ? absolutePublicURL(col.background_url) : '';
    const colHasImage = Boolean(url);
    if (colHasImage) {
      style.backgroundImage = `url(${url})`;
      style.backgroundSize = 'cover';
      style.backgroundPosition = 'center';
      style.backgroundRepeat = 'no-repeat';
    } else if (col.background_gradient) {
      style.backgroundImage = col.background_gradient;
    } else if (col.background_color) {
      style.backgroundColor = col.background_color;
    }
    return { style, hasImage: colHasImage, textColor: col.text_color };
  };

  const leftColumn = columnStyle(0);
  const rightColumn = columnStyle(1);

  const branding = (
    <BrandHeader
      logoURL={config.show_logo && logoURL ? logoURL : undefined}
      title={config.page_title}
      brand={config.brand_text}
      fallbackName={config.application_name}
    />
  );

  const form = (
    <LoginForm
      onSubmit={handleLogin}
      configured={config.configured}
      allowRecovery={config.allow_recovery}
      allowRegistration={config.allow_registration}
      org={org}
      ws={ws}
      app={app}
    />
  );

  // Per-column side-panel text: one title + an ordered list of
  // HTML content strings. Empty title and empty contents → no panel.
  const columnText = (index: number) => {
    const col = config.columns?.find(c => c.column_index === index);
    return {
      title: col?.text_block_title ?? '',
      contents: col?.text_contents ?? [],
    };
  };
  const leftText = columnText(0);
  const rightText = columnText(1);

  return (
    <div className="relative min-h-screen w-full font-sans antialiased" style={wrapperStyle}>
      {/* Readability overlay — only when a background image is set.
          A subtle gradient scrim keeps white form cards and white
          side-panel text legible on any uploaded photo. */}
      {hasImage && (
        <div
          className="absolute inset-0 pointer-events-none"
          style={{
            background:
              'linear-gradient(135deg, rgba(15,23,42,0.55) 0%, rgba(15,23,42,0.35) 50%, rgba(15,23,42,0.55) 100%)',
          }}
        />
      )}

      {config.template_kind === 'centered_1col' && (
        <div className="relative min-h-screen w-full flex items-center justify-center p-6">
          <FormCard>
            {branding}
            <div className="mt-7">{form}</div>
          </FormCard>
        </div>
      )}

      {config.template_kind === 'split_login_left' && (
        <div className="relative min-h-screen w-full grid grid-cols-1 md:grid-cols-2">
          <div className="flex items-center justify-center p-6 md:p-12" style={leftColumn.style}>
            <FormCard>
              {branding}
              <div className="mt-7">{form}</div>
            </FormCard>
          </div>
          <div className="flex items-center justify-center p-6 md:p-12" style={rightColumn.style}>
            <TextPanelFrame hasImage={hasImage || rightColumn.hasImage} textColor={rightColumn.textColor}>
              <TextPanel title={rightText.title} contents={rightText.contents} />
            </TextPanelFrame>
          </div>
        </div>
      )}

      {config.template_kind === 'split_login_right' && (
        <div className="relative min-h-screen w-full grid grid-cols-1 md:grid-cols-2">
          <div className="flex items-center justify-center p-6 md:p-12 order-2 md:order-1" style={leftColumn.style}>
            <TextPanelFrame hasImage={hasImage || leftColumn.hasImage} textColor={leftColumn.textColor}>
              <TextPanel title={leftText.title} contents={leftText.contents} />
            </TextPanelFrame>
          </div>
          <div className="flex items-center justify-center p-6 md:p-12 order-1 md:order-2" style={rightColumn.style}>
            <FormCard>
              {branding}
              <div className="mt-7">{form}</div>
            </FormCard>
          </div>
        </div>
      )}
    </div>
  );
};

// ---------- Pieces --------------------------------------------------

const FormCard: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className="relative w-full max-w-sm">
    <div className="absolute -inset-2 bg-white/30 rounded-[28px] blur-xl pointer-events-none opacity-80" aria-hidden />
    <div className="relative bg-white/95 backdrop-blur-md rounded-2xl shadow-[0_30px_60px_-15px_rgba(15,23,42,0.35)] ring-1 ring-black/5 p-8 sm:p-9">
      {children}
    </div>
  </div>
);

const TextPanelFrame: React.FC<{
  hasImage: boolean;
  textColor?: string;
  children: React.ReactNode;
}> = ({ hasImage, textColor, children }) => {
  // Explicit admin-set textColor wins; otherwise fall back to the
  // auto-contrast rule the component used before per-column support.
  const style: React.CSSProperties = textColor ? { color: textColor } : {};
  const fallbackClass = textColor
    ? ''
    : hasImage
      ? 'text-white'
      : 'text-gray-800 dark:text-gray-100';
  return (
    <div className={`max-w-md ${fallbackClass}`} style={style}>
      {children}
    </div>
  );
};

const BrandHeader: React.FC<{
  logoURL?: string;
  title: string;
  brand: string;
  fallbackName: string;
}> = ({ logoURL, title, brand, fallbackName }) => {
  const displayTitle = title || fallbackName;
  return (
    <div className="flex flex-col items-center text-center">
      {logoURL && (
        <div className="mb-4 h-16 w-16 rounded-2xl bg-white shadow-sm ring-1 ring-gray-100 flex items-center justify-center overflow-hidden">
          <img src={logoURL} alt="" className="max-h-12 max-w-12 object-contain" />
        </div>
      )}
      {displayTitle && (
        <h1 className="text-[1.375rem] font-semibold text-gray-900 leading-tight">
          {displayTitle}
        </h1>
      )}
      {brand && (
        <p className="mt-1.5 text-[0.875rem] text-gray-500">{brand}</p>
      )}
    </div>
  );
};

const TextPanel: React.FC<{ title: string; contents: string[] }> = ({ title, contents }) => {
  const titleHTML = title && title !== '<br>' ? title : '';
  const visibleContents = contents.filter(c => c && c !== '<br>');
  if (!titleHTML && visibleContents.length === 0) return null;
  // Title + contents are authored via the RichTextEditor in the admin
  // UI and stored as HTML. Only admins can author this content, so
  // we render it verbatim — see the security note in
  // Shared/Components/RichText/RichTextEditor.tsx.
  return (
    <div>
      {titleHTML && (
        <h2
          className="text-4xl font-bold leading-tight mb-6 tracking-tight"
          dangerouslySetInnerHTML={{ __html: titleHTML }}
        />
      )}
      {visibleContents.length > 0 && (
        <div className="space-y-4 text-lg leading-relaxed opacity-90">
          {visibleContents.map((c, i) => (
            <div key={i} dangerouslySetInnerHTML={{ __html: c }} />
          ))}
        </div>
      )}
    </div>
  );
};


// ---------- Form ----------------------------------------------------

interface LoginFormProps {
  onSubmit: (username: string, password: string) => Promise<string | null>;
  configured: boolean;
  allowRecovery: boolean;
  allowRegistration: boolean;
  org: string;
  ws: string;
  app: string;
}

// Step 1 hands off the identifier to the backend `auth-methods`
// endpoint, which returns the ordered list of auth factors the visitor
// should be offered. Today this may be 'password' alone or
// 'password' + 'oauth' when external providers are configured.
type AuthMethod = 'password' | 'oauth';

// OAuthProviderListing mirrors the backend's auth-methods row
// per configured external IdP. display_name is the label
// rendered on the button; authorize_url is the handler path
// that starts the handshake.
interface OAuthProviderListing {
  provider: string;
  display_name: string;
  authorize_url: string;
}

interface MethodsResponse {
  message?: {
    methods?: AuthMethod[];
    oauth_providers?: OAuthProviderListing[];
  };
}

const LoginForm: React.FC<LoginFormProps> = ({
  onSubmit,
  configured,
  allowRecovery,
  allowRegistration,
  org,
  ws,
  app,
}) => {
  const [step, setStep] = useState<'identifier' | 'password'>('identifier');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [methods, setMethods] = useState<AuthMethod[]>([]);
  const [oauthProviders, setOauthProviders] = useState<OAuthProviderListing[]>([]);
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (!configured) {
    return (
      <div className="rounded-xl bg-amber-50 border border-amber-200 p-4">
        <div className="flex items-start gap-3">
          <svg className="h-5 w-5 text-amber-500 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
          </svg>
          <div className="text-sm text-amber-800">
            <div className="font-semibold mb-0.5">Not configured</div>
            This login page is not fully configured yet. Please contact the administrator.
          </div>
        </div>
      </div>
    );
  }

  const submitIdentifier = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = username.trim();
    if (!trimmed) return;
    setError(null);
    setBusy(true);
    try {
      const resp = (await postPublicApi<{ identifier: string }>(
        `public/applications/auth-methods?org=${encodeURIComponent(org)}&ws=${encodeURIComponent(ws)}&app=${encodeURIComponent(app)}`,
        { identifier: trimmed },
      )) as MethodsResponse;
      // Response is intentionally uniform regardless of whether the
      // identifier matches a known user — prevents enumeration. For
      // now the list is always ['password']; SSO routing will extend
      // it here without changing the rest of the form.
      const list: AuthMethod[] =
        resp?.message?.methods && resp.message.methods.length > 0
          ? resp.message.methods
          : ['password'];
      setMethods(list);
      setOauthProviders(resp?.message?.oauth_providers ?? []);
      setStep('password');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setBusy(false);
    }
  };

  const submitPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setBusy(true);
    const err = await onSubmit(username, password);
    setBusy(false);
    if (err) setError(err);
  };

  const resetToIdentifier = () => {
    setStep('identifier');
    setPassword('');
    setError(null);
  };

  const errorBanner = error ? (
    <div className="rounded-lg bg-red-50 border border-red-200 px-3 py-2 text-sm text-red-700 flex items-start gap-2">
      <svg className="h-4 w-4 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
      </svg>
      {error}
    </div>
  ) : null;

  if (step === 'identifier') {
    return (
      <form onSubmit={submitIdentifier} className="space-y-4">
        <Field
          id="coco-username"
          label="Username or email"
          value={username}
          onChange={setUsername}
          type="text"
          autoComplete="username"
          icon={
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
            </svg>
          }
        />

        {errorBanner}

        <button
          type="submit"
          disabled={busy || !username.trim()}
          className="group relative w-full py-2.5 rounded-lg bg-gradient-to-b from-indigo-600 to-indigo-700 text-white font-semibold text-[0.9375rem] shadow-sm hover:from-indigo-500 hover:to-indigo-600 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-70 transition-all"
        >
          <span className="inline-flex items-center justify-center gap-2">
            {busy && (
              <span className="h-4 w-4 rounded-full border-2 border-white/40 border-t-white animate-spin" />
            )}
            {busy ? 'Checking…' : 'Continue'}
          </span>
        </button>

        {allowRegistration && (
          <div className="pt-2 text-center text-[0.8125rem]">
            <Link
              to={`/register/a/${encodeURIComponent(org)}/${encodeURIComponent(ws)}/${encodeURIComponent(app)}`}
              className="text-gray-600 hover:text-indigo-600 hover:underline"
            >
              Create an account →
            </Link>
          </div>
        )}
      </form>
    );
  }

  // step === 'password'. Methods may include 'password' plus the
  // external-IdP 'oauth' option; render provider buttons when
  // present. The browser navigates directly to the backend's
  // authorize endpoint — state + PKCE are server-side — so the
  // frontend just computes the return_url and 302s.
  const hasPassword = methods.includes('password');
  const hasOAuth = methods.includes('oauth') && oauthProviders.length > 0;
  const startOAuth = (p: OAuthProviderListing) => {
    const currentReturn = window.location.origin + window.location.pathname + window.location.search;
    window.location.href = `${p.authorize_url}?return_url=${encodeURIComponent(currentReturn)}`;
  };

  return (
    <form onSubmit={submitPassword} className="space-y-4">
      <IdentifierSummary identifier={username} onChange={resetToIdentifier} />

      {hasOAuth && (
        <div className="space-y-2">
          {oauthProviders.map(p => (
            <button
              key={p.provider}
              type="button"
              onClick={() => startOAuth(p)}
              className="w-full py-2.5 px-4 rounded-lg border border-gray-300 bg-white text-gray-800 font-medium text-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 transition-colors"
            >
              Continue with {p.display_name}
            </button>
          ))}
          {hasPassword && (
            <div className="relative py-2">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-gray-200" />
              </div>
              <div className="relative flex justify-center text-xs">
                <span className="bg-white px-2 text-gray-500">or sign in with password</span>
              </div>
            </div>
          )}
        </div>
      )}

      {hasPassword && (
        <Field
          id="coco-password"
          label="Password"
          value={password}
          onChange={setPassword}
          type={showPassword ? 'text' : 'password'}
          autoComplete="current-password"
          autoFocus
          icon={
            <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
            </svg>
          }
          trailing={
            <button
              type="button"
              onClick={() => setShowPassword(v => !v)}
              className="text-gray-400 hover:text-gray-600 transition-colors"
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
          }
        />
      )}

      {errorBanner}

      {hasPassword && (
        <button
          type="submit"
          disabled={busy}
          className="group relative w-full py-2.5 rounded-lg bg-gradient-to-b from-indigo-600 to-indigo-700 text-white font-semibold text-[0.9375rem] shadow-sm hover:from-indigo-500 hover:to-indigo-600 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-70 transition-all"
        >
          <span className="inline-flex items-center justify-center gap-2">
            {busy && (
              <span className="h-4 w-4 rounded-full border-2 border-white/40 border-t-white animate-spin" />
            )}
            {busy ? 'Signing in…' : 'Sign in'}
          </span>
        </button>
      )}

      {allowRecovery && (
        <div className="pt-2 text-center text-[0.8125rem]">
          <Link
            to={`/recover/a/${encodeURIComponent(org)}/${encodeURIComponent(ws)}/${encodeURIComponent(app)}`}
            className="text-indigo-600 hover:text-indigo-700 hover:underline font-medium"
          >
            Forgot password?
          </Link>
        </div>
      )}
    </form>
  );
};

// IdentifierSummary is the "chip" shown in step 2 that tells the
// visitor which identifier they entered, with a one-click path back to
// step 1 to correct it.
const IdentifierSummary: React.FC<{ identifier: string; onChange: () => void }> = ({
  identifier,
  onChange,
}) => (
  <div className="flex items-center justify-between gap-2 rounded-lg bg-gray-50 border border-gray-200 px-3 py-2 text-sm text-gray-700">
    <div className="flex items-center gap-2 min-w-0">
      <svg className="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
      </svg>
      <span className="truncate font-medium">{identifier}</span>
    </div>
    <button
      type="button"
      onClick={onChange}
      className="text-xs font-medium text-indigo-600 hover:text-indigo-700 hover:underline shrink-0"
    >
      Not you?
    </button>
  </div>
);

// ---------- Field ---------------------------------------------------

interface FieldProps {
  id: string;
  label: string;
  value: string;
  onChange: (v: string) => void;
  type: string;
  autoComplete: string;
  icon: React.ReactNode;
  trailing?: React.ReactNode;
  autoFocus?: boolean;
}

const Field: React.FC<FieldProps> = ({
  id,
  label,
  value,
  onChange,
  type,
  autoComplete,
  icon,
  trailing,
  autoFocus,
}) => (
  <div>
    <label className="block text-[0.75rem] font-semibold text-gray-600 uppercase tracking-wide mb-1.5" htmlFor={id}>
      {label}
    </label>
    <div className="relative">
      <span className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400 pointer-events-none">
        {icon}
      </span>
      <input
        id={id}
        type={type}
        value={value}
        onChange={e => onChange(e.target.value)}
        autoComplete={autoComplete}
        autoFocus={autoFocus}
        required
        className={`w-full h-11 rounded-lg border border-gray-200 bg-white text-gray-900 text-[0.9375rem] pl-10 ${
          trailing ? 'pr-10' : 'pr-3'
        } focus:outline-none focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/10 transition-all`}
      />
      {trailing && (
        <span className="absolute right-3 top-1/2 -translate-y-1/2">{trailing}</span>
      )}
    </div>
  </div>
);

export default AppLoginPage;
