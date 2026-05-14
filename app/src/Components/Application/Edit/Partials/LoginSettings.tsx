import React, { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { publicAssetURL } from '../../../../api/client';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput, FormSelect } from '../../../../Shared/Components/Form';
import { Switch } from '../../../../Shared/Components/Switch';
import { Tabs, type TabData } from '../../../../Shared/Components/Tabs';
import { RichTextEditor, type RichTextDefaults } from '../../../../Shared/Components/RichText';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import { LoginTextPresetsModal } from './LoginTextPresetsModal';
import type { LoginTextPreset } from './LoginTextPresets';
import { ApplicationResource } from '../../model/application';

type TemplateKind = 'centered_1col' | 'split_login_left' | 'split_login_right';
type AssetKind = 'background' | 'logo' | 'other';

interface Settings {
  application_id: string;
  redirect_url: string;
  redirect_method: 'POST' | 'GET';
  redirect_secret: string;
  oauth_client_id: string | null;
  custom_headers: Record<string, string>;
  template_kind: TemplateKind;
  background_color: string;
  background_asset_id: string | null;
  background_gradient_from: string;
  background_gradient_to: string;
  background_gradient_angle: number;
  show_logo: boolean;
  page_title: string;
  brand_text: string;
  columns: ColumnSettings[];
  // Persisted toolbar state for each WYSIWYG editor, keyed by the
  // editor's id (e.g. "col.0.body", "col.1.title"). Per-editor so
  // picking a colour in one doesn't bleed into the others.
  rich_text_defaults: Record<string, RichTextDefaults>;
}

// Editor id namespace. Keep strings stable — they're the keys in the
// saved rich_text_defaults map.
const EID = {
  // Exactly one Title editor per column — fixed id.
  columnTitle: (index: number) => `col.${index}.title`,
  // Per-entry Content editor id so toolbar defaults stay with the
  // entry across add / remove.
  columnContent: (index: number, contentID: string) => `col.${index}.content.${contentID}`,
} as const;

// Role a visual column plays in a given template. 'form' columns
// only get background controls; 'text' columns additionally get the
// text-colour override + title/body editors. centered_1col has one
// implicit form column and no editable per-column tabs.
type ColumnRole = 'form' | 'text';

const columnRole = (kind: TemplateKind, index: number): ColumnRole | null => {
  if (kind === 'split_login_left') return index === 0 ? 'form' : 'text';
  if (kind === 'split_login_right') return index === 0 ? 'text' : 'form';
  return null;
};

// ColumnSettings mirrors the backend ColumnConfig. Background fields
// are nullable (unset → inherit wrapper). Side-panel text is one
// title plus an ordered list of content entries, each with a stable
// client-generated id so the RichTextEditor's toolbar defaults
// (keyed by editor id) follow an entry across add / remove.
interface ColumnSettings {
  column_index: number;
  background_color: string | null;
  background_asset_id: string | null;
  background_gradient_from: string | null;
  background_gradient_to: string | null;
  background_gradient_angle: number | null;
  text_color_override: string | null;
  text_block_title: string | null;
  text_contents: ColumnTextContent[];
}

interface ColumnTextContent {
  id: string;
  content: string;
}

interface OAuthClientOption {
  id: string;
  client_id: string;
  display_name: string;
  redirect_uris: string[];
  is_active: boolean;
}

// newContentID returns a stable client id for a new content entry.
// Uses crypto.randomUUID when available; falls back to a short
// random string so older browsers still work.
const newContentID = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `c_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
};

interface SettingsResponse {
  message?: {
    settings: Settings;
    configured: boolean;
    organization_slug?: string;
    workspace_slug?: string;
    client_id?: string;
  };
}

interface AssetRow {
  id: string;
  mime_type: string;
  size_bytes: number;
  kind: AssetKind;
  created_at: string;
}

interface Props {
  applicationId: string;
  workspaceId: string;
}

const TEMPLATE_PREVIEWS: Array<{ kind: TemplateKind; label: string; diagram: React.ReactNode }> = [
  {
    kind: 'centered_1col',
    label: 'Centered (1 column)',
    diagram: (
      <svg viewBox="0 0 80 56" className="w-full h-full">
        <rect width="80" height="56" rx="4" fill="currentColor" className="text-gray-100 dark:text-surface-700" />
        <rect x="24" y="14" width="32" height="6" rx="2" fill="currentColor" className="text-indigo-400" />
        <rect x="24" y="24" width="32" height="4" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="24" y="32" width="32" height="4" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="24" y="40" width="32" height="5" rx="2" fill="currentColor" className="text-indigo-500" />
      </svg>
    ),
  },
  {
    kind: 'split_login_left',
    label: 'Login left, text right',
    diagram: (
      <svg viewBox="0 0 80 56" className="w-full h-full">
        <rect width="80" height="56" rx="4" fill="currentColor" className="text-gray-100 dark:text-surface-700" />
        <rect x="6" y="12" width="28" height="4" rx="1" fill="currentColor" className="text-indigo-400" />
        <rect x="6" y="20" width="28" height="4" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="6" y="28" width="28" height="4" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="6" y="36" width="28" height="5" rx="2" fill="currentColor" className="text-indigo-500" />
        <rect x="44" y="12" width="30" height="3" rx="1" fill="currentColor" className="text-gray-400 dark:text-surface-500" />
        <rect x="44" y="20" width="30" height="2" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="44" y="26" width="22" height="2" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="44" y="32" width="26" height="2" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
      </svg>
    ),
  },
  {
    kind: 'split_login_right',
    label: 'Text left, login right',
    diagram: (
      <svg viewBox="0 0 80 56" className="w-full h-full">
        <rect width="80" height="56" rx="4" fill="currentColor" className="text-gray-100 dark:text-surface-700" />
        <rect x="6" y="12" width="30" height="3" rx="1" fill="currentColor" className="text-gray-400 dark:text-surface-500" />
        <rect x="6" y="20" width="30" height="2" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="6" y="26" width="22" height="2" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="6" y="32" width="26" height="2" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="46" y="12" width="28" height="4" rx="1" fill="currentColor" className="text-indigo-400" />
        <rect x="46" y="20" width="28" height="4" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="46" y="28" width="28" height="4" rx="1" fill="currentColor" className="text-gray-300 dark:text-surface-500" />
        <rect x="46" y="36" width="28" height="5" rx="2" fill="currentColor" className="text-indigo-500" />
      </svg>
    ),
  },
];

// Small wrapper that turns null → null for the preview <img src>.
const assetPreviewURL = (assetID: string | null): string | null =>
  assetID ? publicAssetURL(assetID) : null;

// RichTextBinding bundles the props a single RichTextEditor needs to
// read + write its slot in settings.rich_text_defaults. The parent
// builds one per editor id and spreads it onto the editor.
interface RichTextBinding {
  defaults: RichTextDefaults | undefined;
  onDefaultsChange: (next: RichTextDefaults) => void;
}

export const LoginSettings: React.FC<Props> = ({ applicationId, workspaceId }) => {
  void workspaceId; // retained for prop-contract stability with the parent
  const { get, patch, post } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();

  const [settings, setSettings] = useState<Settings | null>(null);
  const [orgSlug, setOrgSlug] = useState('');
  const [wsSlug, setWsSlug] = useState('');
  const [clientId, setClientId] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await get<SettingsResponse>(
        `applications/{${ApplicationResource}}/{id:${applicationId}}/login-settings`
      );
      const body = response?.message;
      if (body?.settings) {
        // Backend may send `columns: null` when the app has no
        // overrides yet; normalise to an empty array. Likewise,
        // `rich_text_defaults` is `{}` when nothing's been saved — ensure
        // it's always an object so editor props stay stable.
        setSettings({
          ...body.settings,
          // Normalise nullables: missing columns / content entries
          // become empty arrays so downstream .find() / .map() etc.
          // stays safe.
          columns: (body.settings.columns ?? []).map(c => ({
            ...c,
            text_contents: c.text_contents ?? [],
          })),
          rich_text_defaults: body.settings.rich_text_defaults ?? {},
        });
      }
      setOrgSlug(body?.organization_slug ?? '');
      setWsSlug(body?.workspace_slug ?? '');
      setClientId(body?.client_id ?? '');
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to load settings');
    } finally {
      setLoading(false);
    }
  }, [get, applicationId, errorMessage]);

  useEffect(() => {
    void load();
  }, [load]);

  const save = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      // Centered templates have no visual columns — drop any
      // stale overrides so the backend removes the rows.
      const columns = settings.template_kind === 'centered_1col'
        ? []
        : settings.columns.filter(c => hasAnyColumnOverride(c));
      const payload = {
        redirect_url: settings.redirect_url,
        redirect_method: settings.redirect_method,
        redirect_secret: settings.redirect_secret,
        oauth_client_id: settings.oauth_client_id,
        custom_headers: settings.custom_headers,
        template_kind: settings.template_kind,
        background_color: settings.background_color,
        background_asset_id: settings.background_asset_id,
        background_gradient_from: settings.background_gradient_from,
        background_gradient_to: settings.background_gradient_to,
        background_gradient_angle: settings.background_gradient_angle,
        columns,
        show_logo: settings.show_logo,
        page_title: settings.page_title,
        brand_text: settings.brand_text,
        rich_text_defaults: settings.rich_text_defaults,
      };
      await patch(
        `applications/{${ApplicationResource}}/{id:${applicationId}}/login-settings`,
        payload
      );
      successMessage('Login template saved');
      await load();
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  const uploadAsset = async (file: File, kind: AssetKind): Promise<string | null> => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('kind', kind);
    try {
      const response = await post<{ message?: AssetRow }>(
        `applications/{${ApplicationResource}}/{id:${applicationId}}/login-template/assets`,
        formData
      );
      return response?.message?.id ?? null;
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : `Failed to upload ${kind}`);
      return null;
    }
  };

  if (loading || !settings) {
    return <div className="text-sm text-gray-500 p-4">Loading login template…</div>;
  }

  // richTextBindings returns the `defaults` / `onDefaultsChange` pair
  // for a single editor id. Updates replace only that slot in the
  // map so each editor's toolbar state is independent.
  const richTextBindings = (id: string): RichTextBinding => ({
    defaults: settings.rich_text_defaults[id],
    onDefaultsChange: next => {
      setSettings({
        ...settings,
        rich_text_defaults: { ...settings.rich_text_defaults, [id]: next },
      });
    },
  });

  // Public URL is always the three-segment slug form. When any slug
  // is missing (shouldn't happen in practice) we blank the URL — we'd
  // rather show nothing than a UUID-shaped URL that no longer works.
  const slugsReady = Boolean(orgSlug && wsSlug && clientId);
  const loginURL = slugsReady
    ? `/login/a/${encodeURIComponent(orgSlug)}/${encodeURIComponent(wsSlug)}/${encodeURIComponent(clientId)}`
    : '';
  // Absolute URL for copy-to-clipboard (so the link works when pasted
  // anywhere, not just when opened next to the admin UI).
  const absoluteLoginURL =
    loginURL && typeof window !== 'undefined'
      ? `${window.location.origin}${loginURL}`
      : loginURL;

  const copyLink = async () => {
    if (!absoluteLoginURL) {
      errorMessage('Login page URL is not ready yet');
      return;
    }
    try {
      await navigator.clipboard.writeText(absoluteLoginURL);
      successMessage('Login page URL copied to clipboard');
    } catch {
      errorMessage('Could not copy — your browser blocked clipboard access');
    }
  };

  return (
    <div className="space-y-8 mt-2">
      {/* Test link — opens the rendered public login page in a new tab */}
      <div className="flex items-center justify-between gap-3 p-3 rounded-lg bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-900/40">
        <div className="text-sm text-indigo-900 dark:text-indigo-200 min-w-0">
          <span className="font-semibold">Preview</span> — open this application's login page in a
          new tab or copy the link to share with testers.
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            type="button"
            onClick={() => void copyLink()}
            disabled={!slugsReady}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md border border-indigo-300 dark:border-indigo-700 text-indigo-700 dark:text-indigo-200 bg-white dark:bg-transparent hover:bg-indigo-100 dark:hover:bg-indigo-900/40 disabled:opacity-50 disabled:cursor-not-allowed"
            title={slugsReady ? absoluteLoginURL : 'Public URL unavailable'}
          >
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 0 1-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 0 1 1.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 0 0-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 0 1-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H9.75" />
            </svg>
            Copy link
          </button>
          <a
            href={loginURL || undefined}
            target="_blank"
            rel="noopener noreferrer"
            aria-disabled={!slugsReady}
            className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md bg-indigo-600 text-white hover:bg-indigo-500 ${
              slugsReady ? '' : 'opacity-50 pointer-events-none'
            }`}
          >
            Open login page
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
            </svg>
          </a>
        </div>
      </div>

      {/* Template settings organised in tabs so each concern is one
          click away instead of one long scroll. Preview banner
          above + Save button below stay outside the tabs so they're
          always accessible. */}
      <Tabs
        items={[
          { id: 'dispatch', title: 'Dispatch', content: <DispatchSection applicationId={applicationId} settings={settings} setSettings={setSettings} /> },
          { id: 'layout', title: 'Layout', content: <LayoutSection settings={settings} setSettings={setSettings} /> },
          {
            id: 'appearance',
            title: 'Appearance',
            content: (
              <AppearanceSection
                settings={settings}
                setSettings={setSettings}
                assetPreviewURL={assetPreviewURL}
                uploadAsset={uploadAsset}
                richTextBindings={richTextBindings}
              />
            ),
          },
          { id: 'branding', title: 'Branding', content: <BrandingSection settings={settings} setSettings={setSettings} /> },
        ]}
        initialActiveId="dispatch"
      />

      <p className="text-xs text-gray-500">
        Password recovery and registration switches live on the application's
        <em> Details </em> page — they apply to the whole application, not just
        the login template.
      </p>

      <div className="pt-2 border-t border-gray-100 dark:border-surface-700">
        <Submit type="button" onClick={() => void save()} label="Save login template" disabled={saving} />
      </div>
    </div>
  );
};

// --- Tabbed sections -----------------------------------------------

const DispatchSection: React.FC<{
  applicationId: string;
  settings: Settings;
  setSettings: (s: Settings) => void;
}> = ({ applicationId, settings, setSettings }) => {
  const { get } = useHttpClient();
  const [oauthClients, setOauthClients] = useState<OAuthClientOption[]>([]);

  useEffect(() => {
    void (async () => {
      try {
        const resp = await get<{ message?: { clients?: OAuthClientOption[] } }>(
          `applications/{${ApplicationResource}}/{id:${applicationId}}/oauth-clients`
        );
        setOauthClients((resp?.message?.clients ?? []).filter(c => c.is_active));
      } catch {
        // non-fatal — dispatch section still works without the dropdown
      }
    })();
  }, [get, applicationId]);

  const selectedClient = oauthClients.find(c => c.id === settings.oauth_client_id) ?? null;

  const handleClientChange = (v: string) => {
    const client = oauthClients.find(c => c.id === v) ?? null;
    let redirect_url = settings.redirect_url;
    if (client && !client.redirect_uris.includes(redirect_url)) {
      redirect_url = client.redirect_uris[0] ?? '';
    }
    setSettings({ ...settings, oauth_client_id: v || null, redirect_url });
  };

  return (
    <section>
      <p className="text-sm text-gray-500 mb-3">
        Where authenticated credentials are forwarded. All three must be set for login to accept credentials.
      </p>

      {oauthClients.length > 0 && (
        <div className="mb-4">
          <FormSelect
            label="OAuth client (optional)"
            value={settings.oauth_client_id ?? ''}
            onChange={handleClientChange}
            options={[
              { label: 'None — manual URL', value: '' },
              ...oauthClients.map(c => ({ label: c.display_name || c.client_id, value: c.id })),
            ]}
          />
        </div>
      )}

      {selectedClient ? (
        <div className="rounded-lg border border-indigo-200 dark:border-indigo-800 bg-indigo-50/30 dark:bg-indigo-900/10 p-3 text-sm text-gray-600 dark:text-gray-400">
          After login, credentials will be dispatched to the first registered redirect URI
          of <strong>{selectedClient.display_name || selectedClient.client_id}</strong> using
          its client secret. No manual URL, method, or shared secret configuration is needed.
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormInput
            label="Redirect URL"
            value={settings.redirect_url}
            onChange={v => setSettings({ ...settings, redirect_url: v })}
            placeholder="https://example.com/auth/callback"
          />
          <FormSelect
            label="Method"
            value={settings.redirect_method}
            onChange={v => setSettings({ ...settings, redirect_method: v as 'POST' | 'GET' })}
            options={[
              { label: 'POST', value: 'POST' },
              { label: 'GET', value: 'GET' },
            ]}
          />
          <FormInput
            label="Shared secret (X-Login-Secret header)"
            value={settings.redirect_secret}
            onChange={v => setSettings({ ...settings, redirect_secret: v })}
            type="password"
            className="md:col-span-2"
          />
        </div>
      )}
    </section>
  );
};

const LayoutSection: React.FC<{
  settings: Settings;
  setSettings: (s: Settings) => void;
}> = ({ settings, setSettings }) => (
  <section>
    <p className="text-sm text-gray-500 mb-3">Pick one of three presets. No custom HTML.</p>
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
      {TEMPLATE_PREVIEWS.map(t => {
        const active = settings.template_kind === t.kind;
        return (
          <button
            key={t.kind}
            type="button"
            onClick={() => setSettings({ ...settings, template_kind: t.kind })}
            className={`flex flex-col items-stretch gap-2 rounded-xl border p-3 transition-colors text-left ${
              active
                ? 'border-indigo-500 ring-2 ring-indigo-200 dark:ring-indigo-900/60 bg-indigo-50/40 dark:bg-indigo-900/10'
                : 'border-gray-200 dark:border-surface-700 hover:border-gray-300 dark:hover:border-surface-600'
            }`}
          >
            <div className="aspect-[80/56] rounded-md overflow-hidden">{t.diagram}</div>
            <span className="text-sm font-medium text-gray-800 dark:text-gray-200">{t.label}</span>
          </button>
        );
      })}
    </div>
  </section>
);

const AppearanceSection: React.FC<{
  settings: Settings;
  setSettings: (s: Settings) => void;
  assetPreviewURL: (id: string | null) => string | null;
  uploadAsset: (file: File, kind: AssetKind) => Promise<string | null>;
  richTextBindings: (id: string) => RichTextBinding;
}> = ({ settings, setSettings, assetPreviewURL, uploadAsset, richTextBindings }) => (
  <section className="space-y-6">
    <div>
      <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-1">Global</h4>
      <p className="text-sm text-gray-500 mb-3">
        Pick a color, configure a gradient, or upload an image. Precedence (highest first): image, gradient, solid color.
      </p>
      <div className="flex flex-wrap items-end gap-4">
        <div className="flex items-center gap-2">
          <input
            type="color"
            value={settings.background_color}
            onChange={e => setSettings({ ...settings, background_color: e.target.value })}
            className="h-10 w-16 rounded border border-gray-200 dark:border-surface-700 bg-transparent"
          />
          <span className="text-sm font-mono text-gray-600 dark:text-gray-400">
            {settings.background_color}
          </span>
        </div>
        <label className="cursor-pointer">
          <span className="inline-flex items-center px-3 py-2 text-sm font-medium rounded-md border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-800 hover:bg-gray-50 dark:hover:bg-surface-700">
            Upload image
          </span>
          <input
            type="file"
            accept="image/*"
            className="hidden"
            onChange={async e => {
              const file = e.target.files?.[0];
              if (!file) return;
              const id = await uploadAsset(file, 'background');
              if (id) setSettings({ ...settings, background_asset_id: id });
              e.target.value = '';
            }}
          />
        </label>
        {settings.background_asset_id && (
          <div className="flex items-center gap-3">
            <img
              src={assetPreviewURL(settings.background_asset_id) ?? ''}
              alt=""
              className="h-12 w-20 object-cover rounded border border-gray-200 dark:border-surface-700"
            />
            <button
              type="button"
              onClick={() => setSettings({ ...settings, background_asset_id: null })}
              className="text-sm text-red-600 hover:underline"
            >
              Remove
            </button>
          </div>
        )}
      </div>

      <GradientEditor settings={settings} setSettings={setSettings} />
    </div>

    {/* Per-column overrides — only visible for split templates. */}
    {settings.template_kind !== 'centered_1col' && (
      <div className="pt-4 border-t border-gray-100 dark:border-surface-700">
        <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-1">Per-column</h4>
        <p className="text-sm text-gray-500 mb-3">
          Override the wrapper background on either side. Unset fields inherit the global settings above.
        </p>
        <Tabs
          items={buildColumnTabs(settings, {
            assetPreviewURL,
            uploadAsset,
            setSettings,
            richTextBindings,
          })}
          initialActiveId="col-0"
        />
      </div>
    )}
  </section>
);

const BrandingSection: React.FC<{
  settings: Settings;
  setSettings: (s: Settings) => void;
}> = ({ settings, setSettings }) => (
  <section className="space-y-6">
    <div>
      <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-1">Logo</h4>
      <div className="flex items-center gap-4 mt-2">
        <Switch
          checked={settings.show_logo}
          onChange={v => setSettings({ ...settings, show_logo: v })}
          label="Show logo on login page"
        />
        <span className="text-xs text-gray-500">
          Upload the logo itself from the Edit panel. It's stored as{' '}
          <code className="text-[0.75rem]">logo.&lt;ext&gt;</code> in the media library.
        </span>
      </div>
    </div>

    <div className="pt-4 border-t border-gray-100 dark:border-surface-700">
      <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-3">Text</h4>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <FormInput
          label="Page title"
          value={settings.page_title}
          onChange={v => setSettings({ ...settings, page_title: v })}
          placeholder="Shown below the logo"
        />
        <FormInput
          label="Brand"
          value={settings.brand_text}
          onChange={v => setSettings({ ...settings, brand_text: v })}
          placeholder="Shown below the title"
        />
      </div>
    </div>
  </section>
);

// GradientEditor mutates the three gradient fields in-place on the
// parent's `settings`. Enable/disable is derived from whether either
// stop is set — there's no explicit "on/off" toggle stored.
const GradientEditor: React.FC<{
  settings: Settings;
  setSettings: (s: Settings) => void;
}> = ({ settings, setSettings }) => {
  const enabled =
    settings.background_gradient_from !== '' && settings.background_gradient_to !== '';
  const angle = settings.background_gradient_angle || 135;
  const from = settings.background_gradient_from || '#6366f1';
  const to = settings.background_gradient_to || '#ec4899';
  const previewCSS = `linear-gradient(${angle}deg, ${from} 0%, ${to} 100%)`;

  const toggle = () => {
    if (enabled) {
      setSettings({
        ...settings,
        background_gradient_from: '',
        background_gradient_to: '',
        background_gradient_angle: 135,
      });
    } else {
      setSettings({
        ...settings,
        background_gradient_from: from,
        background_gradient_to: to,
        background_gradient_angle: angle,
      });
    }
  };

  return (
    <div className="mt-5 pt-4 border-t border-gray-100 dark:border-surface-700">
      <div className="flex items-center justify-between mb-3">
        <div>
          <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-100">Gradient</h4>
          <p className="text-xs text-gray-500 mt-0.5">
            Linear gradient between two stops. Shown when no background image is uploaded.
          </p>
        </div>
        <button
          type="button"
          onClick={toggle}
          className={`text-xs font-medium px-3 py-1.5 rounded-md border transition-colors ${
            enabled
              ? 'border-red-200 text-red-600 hover:bg-red-50'
              : 'border-indigo-200 text-indigo-600 hover:bg-indigo-50'
          }`}
        >
          {enabled ? 'Remove gradient' : 'Use gradient'}
        </button>
      </div>

      {enabled && (
        <div className="flex flex-wrap items-end gap-5">
          <div>
            <label className="block text-[0.7rem] font-semibold text-gray-500 uppercase tracking-wide mb-1">
              From
            </label>
            <div className="flex items-center gap-2">
              <input
                type="color"
                value={from}
                onChange={e =>
                  setSettings({ ...settings, background_gradient_from: e.target.value })
                }
                className="h-10 w-14 rounded border border-gray-200 dark:border-surface-700 bg-transparent"
              />
              <span className="text-xs font-mono text-gray-600 dark:text-gray-400">{from}</span>
            </div>
          </div>
          <div>
            <label className="block text-[0.7rem] font-semibold text-gray-500 uppercase tracking-wide mb-1">
              To
            </label>
            <div className="flex items-center gap-2">
              <input
                type="color"
                value={to}
                onChange={e =>
                  setSettings({ ...settings, background_gradient_to: e.target.value })
                }
                className="h-10 w-14 rounded border border-gray-200 dark:border-surface-700 bg-transparent"
              />
              <span className="text-xs font-mono text-gray-600 dark:text-gray-400">{to}</span>
            </div>
          </div>
          <div className="min-w-[160px]">
            <label className="block text-[0.7rem] font-semibold text-gray-500 uppercase tracking-wide mb-1">
              Angle ({angle}°)
            </label>
            <input
              type="range"
              min={0}
              max={360}
              step={5}
              value={angle}
              onChange={e =>
                setSettings({
                  ...settings,
                  background_gradient_angle: Number(e.target.value),
                })
              }
              className="w-full accent-indigo-600"
            />
          </div>
          <div
            className="h-14 w-32 rounded-md border border-gray-200 dark:border-surface-700"
            style={{ backgroundImage: previewCSS }}
            aria-label="Gradient preview"
          />
        </div>
      )}
    </div>
  );
};

// --- per-column helpers -----------------------------------------------

// hasAnyColumnOverride returns true when at least one override on the
// column is explicitly set. An all-empty column is equivalent to
// "inherit wrapper" and should be dropped from the save payload. A
// non-empty title or any non-empty content entry counts as an
// override.
const hasAnyColumnOverride = (c: ColumnSettings): boolean => {
  if ((c.background_color !== null && c.background_color !== '') ||
    c.background_asset_id !== null ||
    (c.background_gradient_from !== null && c.background_gradient_from !== '') ||
    (c.background_gradient_to !== null && c.background_gradient_to !== '') ||
    c.background_gradient_angle !== null ||
    (c.text_color_override !== null && c.text_color_override !== '') ||
    (c.text_block_title !== null && c.text_block_title !== '' && c.text_block_title !== '<br>')) {
    return true;
  }
  return c.text_contents.some(e => hasContent(e.content));
};

const hasContent = (s: string): boolean => s !== '' && s !== '<br>';

const emptyColumn = (index: number): ColumnSettings => ({
  column_index: index,
  background_color: null,
  background_asset_id: null,
  background_gradient_from: null,
  background_gradient_to: null,
  background_gradient_angle: null,
  text_color_override: null,
  text_block_title: null,
  text_contents: [],
});

const getOrEmptyColumn = (s: Settings, index: number): ColumnSettings => {
  const existing = s.columns.find(c => c.column_index === index);
  return existing ?? emptyColumn(index);
};

const applyColumnChange = (s: Settings, index: number, next: ColumnSettings): Settings => {
  const others = s.columns.filter(c => c.column_index !== index);
  // Only store the row when at least one field is set — keeps the
  // admin state aligned with what the save payload will send.
  if (!hasAnyColumnOverride(next)) {
    return { ...s, columns: others };
  }
  return { ...s, columns: [...others, next].sort((a, b) => a.column_index - b.column_index) };
};

const resetColumn = (s: Settings, index: number): Settings => ({
  ...s,
  columns: s.columns.filter(c => c.column_index !== index),
});

// buildColumnTabs produces the TabData array for the per-column
// background editor. One tab per visual column; the bullet suffix on
// a label signals the column is currently overriding the wrapper.
const buildColumnTabs = (
  settings: Settings,
  deps: {
    assetPreviewURL: (id: string | null) => string | null;
    uploadAsset: (file: File, kind: AssetKind) => Promise<string | null>;
    setSettings: (next: Settings) => void;
    richTextBindings: (id: string) => RichTextBinding;
  },
): TabData[] => {
  const mkTab = (index: number, sideLabel: string): TabData | null => {
    const role = columnRole(settings.template_kind, index);
    if (!role) return null;
    const column = getOrEmptyColumn(settings, index);
    const active = hasAnyColumnOverride(column);
    // Label tells the admin which role this column plays in the
    // chosen template so they know what kind of content to put
    // there. Bullet suffix signals an active override.
    const baseLabel = `${sideLabel} (${role})`;
    return {
      id: `col-${index}`,
      title: active ? `${baseLabel} •` : baseLabel,
      content: (
        <ColumnBackgroundPanel
          title={baseLabel}
          role={role}
          column={column}
          columnIndex={index}
          assetPreviewURL={deps.assetPreviewURL}
          uploadAsset={deps.uploadAsset}
          onChange={next => deps.setSettings(applyColumnChange(settings, index, next))}
          onReset={() => deps.setSettings(resetColumn(settings, index))}
          richTextBindings={deps.richTextBindings}
        />
      ),
    };
  };
  return [mkTab(0, 'Left'), mkTab(1, 'Right')].filter(
    (t): t is TabData => t !== null,
  );
};

// --- ColumnBackgroundPanel --------------------------------------------

const ColumnBackgroundPanel: React.FC<{
  title: string;
  // Form columns render background-only controls; text columns
  // additionally expose the text-colour override + text block list.
  role: ColumnRole;
  column: ColumnSettings;
  columnIndex: number;
  assetPreviewURL: (id: string | null) => string | null;
  uploadAsset: (file: File, kind: AssetKind) => Promise<string | null>;
  onChange: (next: ColumnSettings) => void;
  onReset: () => void;
  // Factory that returns a RichTextEditor binding for a given editor
  // id. Each text block builds its own ids via EID.columnTitle /
  // EID.columnBody so toolbar defaults stay per-editor.
  richTextBindings: (id: string) => RichTextBinding;
}> = ({ title, role, column, columnIndex, assetPreviewURL, uploadAsset, onChange, onReset, richTextBindings }) => {
  const gradientEnabled =
    column.background_gradient_from !== null && column.background_gradient_to !== null;
  const from = column.background_gradient_from ?? '#6366f1';
  const to = column.background_gradient_to ?? '#ec4899';
  const angle = column.background_gradient_angle ?? 135;

  const active = hasAnyColumnOverride(column);

  // Preset picker state lives per-column: the picker itself, and a
  // staged preset held between picker and confirmation. We only ask
  // for confirmation when the admin would be overwriting existing
  // authored text — otherwise the apply is non-destructive.
  const [presetOpen, setPresetOpen] = useState(false);
  const [pendingPreset, setPendingPreset] = useState<LoginTextPreset | null>(null);

  const hasExistingText =
    (column.text_block_title !== null &&
      column.text_block_title !== '' &&
      column.text_block_title !== '<br>') ||
    column.text_contents.some(e => hasContent(e.content));

  const applyPreset = (preset: LoginTextPreset) => {
    onChange({
      ...column,
      text_block_title: preset.title,
      text_contents: preset.contents.map(content => ({
        id: newContentID(),
        content,
      })),
    });
  };

  const handlePresetPick = (preset: LoginTextPreset) => {
    setPresetOpen(false);
    if (hasExistingText) {
      setPendingPreset(preset);
    } else {
      applyPreset(preset);
    }
  };

  return (
    <div
      className={`rounded-lg border p-4 ${
        active
          ? 'border-indigo-200 dark:border-indigo-800 bg-indigo-50/30 dark:bg-indigo-900/10'
          : 'border-gray-200 dark:border-surface-700'
      }`}
    >
      <div className="flex items-center justify-between mb-3">
        <div>
          <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-100">{title}</h4>
          <p className="text-xs text-gray-500 mt-0.5">
            {active ? 'Overriding wrapper background' : 'Inheriting wrapper background'}
          </p>
        </div>
        {active && (
          <button
            type="button"
            onClick={onReset}
            className="text-xs font-medium text-red-600 hover:text-red-700 hover:underline"
          >
            Reset to inherit
          </button>
        )}
      </div>

      <div className="flex flex-wrap items-end gap-4">
        {/* Colour */}
        <div>
          <label className="block text-[0.7rem] font-semibold text-gray-500 uppercase tracking-wide mb-1">
            Color
          </label>
          <div className="flex items-center gap-2">
            <input
              type="color"
              value={column.background_color ?? '#f9fafb'}
              onChange={e => onChange({ ...column, background_color: e.target.value })}
              className="h-10 w-14 rounded border border-gray-200 dark:border-surface-700 bg-transparent"
            />
            {column.background_color && (
              <>
                <span className="text-xs font-mono text-gray-600 dark:text-gray-400">
                  {column.background_color}
                </span>
                <button
                  type="button"
                  onClick={() => onChange({ ...column, background_color: null })}
                  className="text-[0.7rem] text-red-600 hover:underline"
                >
                  clear
                </button>
              </>
            )}
          </div>
        </div>

        {/* Image upload */}
        <label className="cursor-pointer">
          <span className="inline-flex items-center px-3 py-2 text-sm font-medium rounded-md border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-800 hover:bg-gray-50 dark:hover:bg-surface-700">
            Upload image
          </span>
          <input
            type="file"
            accept="image/*"
            className="hidden"
            onChange={async e => {
              const file = e.target.files?.[0];
              if (!file) return;
              const id = await uploadAsset(file, 'background');
              if (id) onChange({ ...column, background_asset_id: id });
              e.target.value = '';
            }}
          />
        </label>
        {column.background_asset_id && (
          <div className="flex items-center gap-3">
            <img
              src={assetPreviewURL(column.background_asset_id) ?? ''}
              alt=""
              className="h-12 w-20 object-cover rounded border border-gray-200 dark:border-surface-700"
            />
            <button
              type="button"
              onClick={() => onChange({ ...column, background_asset_id: null })}
              className="text-sm text-red-600 hover:underline"
            >
              Remove
            </button>
          </div>
        )}
      </div>

      {/* Gradient */}
      <div className="mt-4 pt-3 border-t border-gray-100 dark:border-surface-700">
        <div className="flex items-center justify-between mb-3">
          <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-200 uppercase tracking-widest">
            Gradient
          </h5>
          <button
            type="button"
            onClick={() => {
              if (gradientEnabled) {
                onChange({
                  ...column,
                  background_gradient_from: null,
                  background_gradient_to: null,
                  background_gradient_angle: null,
                });
              } else {
                onChange({
                  ...column,
                  background_gradient_from: from,
                  background_gradient_to: to,
                  background_gradient_angle: angle,
                });
              }
            }}
            className={`text-xs font-medium px-3 py-1.5 rounded-md border transition-colors ${
              gradientEnabled
                ? 'border-red-200 text-red-600 hover:bg-red-50'
                : 'border-indigo-200 text-indigo-600 hover:bg-indigo-50'
            }`}
          >
            {gradientEnabled ? 'Remove gradient' : 'Use gradient'}
          </button>
        </div>
        {gradientEnabled && (
          <div className="flex flex-wrap items-end gap-4">
            <div>
              <label className="block text-[0.7rem] font-semibold text-gray-500 uppercase tracking-wide mb-1">
                From
              </label>
              <div className="flex items-center gap-2">
                <input
                  type="color"
                  value={from}
                  onChange={e => onChange({ ...column, background_gradient_from: e.target.value })}
                  className="h-10 w-14 rounded border border-gray-200 dark:border-surface-700 bg-transparent"
                />
                <span className="text-xs font-mono text-gray-600 dark:text-gray-400">{from}</span>
              </div>
            </div>
            <div>
              <label className="block text-[0.7rem] font-semibold text-gray-500 uppercase tracking-wide mb-1">
                To
              </label>
              <div className="flex items-center gap-2">
                <input
                  type="color"
                  value={to}
                  onChange={e => onChange({ ...column, background_gradient_to: e.target.value })}
                  className="h-10 w-14 rounded border border-gray-200 dark:border-surface-700 bg-transparent"
                />
                <span className="text-xs font-mono text-gray-600 dark:text-gray-400">{to}</span>
              </div>
            </div>
            <div className="min-w-[160px]">
              <label className="block text-[0.7rem] font-semibold text-gray-500 uppercase tracking-wide mb-1">
                Angle ({angle}°)
              </label>
              <input
                type="range"
                min={0}
                max={360}
                step={5}
                value={angle}
                onChange={e =>
                  onChange({ ...column, background_gradient_angle: Number(e.target.value) })
                }
                className="w-full accent-indigo-600"
              />
            </div>
            <div
              className="h-14 w-32 rounded-md border border-gray-200 dark:border-surface-700"
              style={{ backgroundImage: `linear-gradient(${angle}deg, ${from} 0%, ${to} 100%)` }}
              aria-label="Gradient preview"
            />
          </div>
        )}
      </div>

      {/* Form columns contain the login card and render no side-panel
          text, so they only expose background controls. Text
          columns below additionally get the text-colour override +
          title/body editors. */}
      {role === 'text' && (
        <>
          {/* Suggested template picker — one-click population of the
              column's title + content list. When the column already
              has authored text a confirm step protects it. */}
          <div className="mt-4 pt-3 border-t border-gray-100 dark:border-surface-700">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-200 uppercase tracking-widest">
                  Content template
                </h5>
                <p className="text-[0.7rem] text-gray-500 mt-0.5">
                  Fill this column with a ready-made title and content blocks. You can edit everything afterwards.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setPresetOpen(true)}
                className="inline-flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md border border-indigo-200 text-indigo-600 hover:bg-indigo-50 dark:border-indigo-800 dark:hover:bg-indigo-900/30 shrink-0"
              >
                <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z" />
                </svg>
                Use a suggested template
              </button>
            </div>
          </div>

          <LoginTextPresetsModal
            isOpen={presetOpen}
            onClose={() => setPresetOpen(false)}
            onApply={handlePresetPick}
          />
          <ConfirmModal
            isOpen={pendingPreset !== null}
            onClose={() => setPendingPreset(null)}
            onConfirm={() => {
              if (pendingPreset) applyPreset(pendingPreset);
              setPendingPreset(null);
            }}
            title="Replace column content?"
            message={
              <>
                This column already has a title or content entries. Applying
                <strong> {pendingPreset?.label} </strong>
                will overwrite them.
              </>
            }
            confirmLabel="Replace"
            cancelLabel="Keep existing"
            variant="danger"
          />

          {/* Text colour override */}
          <div className="mt-4 pt-3 border-t border-gray-100 dark:border-surface-700">
            <label className="flex items-center gap-3">
              <span className="text-xs font-semibold text-gray-700 dark:text-gray-200 uppercase tracking-widest">
                Text color
              </span>
              <input
                type="color"
                value={column.text_color_override ?? '#ffffff'}
                onChange={e => onChange({ ...column, text_color_override: e.target.value })}
                className="h-8 w-12 rounded border border-gray-200 dark:border-surface-700 bg-transparent"
              />
              {column.text_color_override && (
                <>
                  <span className="text-xs font-mono text-gray-600 dark:text-gray-400">
                    {column.text_color_override}
                  </span>
                  <button
                    type="button"
                    onClick={() => onChange({ ...column, text_color_override: null })}
                    className="text-[0.7rem] text-red-600 hover:underline"
                  >
                    clear
                  </button>
                </>
              )}
            </label>
            <p className="text-[0.7rem] text-gray-500 mt-1">
              Wins over the auto light/dark rule on image-backed columns. Leave empty to auto-pick.
            </p>
          </div>

          {/* Side-panel text — one title at the top, then an ordered
              list of content areas. The "+" button appends a new
              content area; each area has its own trash icon. */}
          <div className="mt-4 pt-3 border-t border-gray-100 dark:border-surface-700">
            <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-200 uppercase tracking-widest mb-3">
              Title
            </h5>
            <RichTextEditor
              label=""
              value={column.text_block_title ?? ''}
              onChange={html =>
                onChange({
                  ...column,
                  text_block_title: html === '' || html === '<br>' ? null : html,
                })
              }
              placeholder="Column title"
              minHeightClass="min-h-[2.5rem]"
              {...richTextBindings(EID.columnTitle(columnIndex))}
            />
          </div>

          <div className="mt-4 pt-3 border-t border-gray-100 dark:border-surface-700">
            <div className="flex items-center justify-between mb-3">
              <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-200 uppercase tracking-widest">
                Content
              </h5>
              <button
                type="button"
                onClick={() =>
                  onChange({
                    ...column,
                    text_contents: [
                      ...column.text_contents,
                      { id: newContentID(), content: '' },
                    ],
                  })
                }
                className="inline-flex items-center gap-1 text-xs font-medium px-2.5 py-1 rounded-md border border-indigo-200 text-indigo-600 hover:bg-indigo-50 dark:border-indigo-800 dark:hover:bg-indigo-900/30"
                title="Add content area"
              >
                <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
                </svg>
                Add content
              </button>
            </div>

            {column.text_contents.length === 0 ? (
              <p className="text-xs text-gray-500 italic">
                No content areas yet. Click <strong>Add content</strong> to create one.
              </p>
            ) : (
              <div className="space-y-4">
                {column.text_contents.map((entry, i) => (
                  <div
                    key={entry.id}
                    className="rounded-md border border-gray-200 dark:border-surface-700 p-3"
                  >
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-[0.7rem] font-semibold text-gray-500 uppercase tracking-widest">
                        Content {i + 1}
                      </span>
                      <button
                        type="button"
                        onClick={() =>
                          onChange({
                            ...column,
                            text_contents: column.text_contents.filter(e => e.id !== entry.id),
                          })
                        }
                        className="inline-flex items-center justify-center h-6 w-6 rounded text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30"
                        title="Remove content"
                      >
                        <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M1 7h22M9 7V4a1 1 0 011-1h4a1 1 0 011 1v3" />
                        </svg>
                      </button>
                    </div>
                    <RichTextEditor
                      label=""
                      value={entry.content}
                      onChange={html =>
                        onChange({
                          ...column,
                          text_contents: column.text_contents.map(e =>
                            e.id === entry.id ? { ...e, content: html } : e,
                          ),
                        })
                      }
                      placeholder="Content"
                      {...richTextBindings(EID.columnContent(columnIndex, entry.id))}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
};

export default LoginSettings;
