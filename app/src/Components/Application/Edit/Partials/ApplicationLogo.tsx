import React, { useCallback, useEffect, useState } from 'react';
import { API_BASE_URL, publicMediaURL, slugMediaURL } from '../../../../api/client';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { findAuthToken } from '../../../Auth/Guard/model/auth';
import { ApplicationResource } from '../../model/application';
import { useApplicationSlugs } from '../../hooks/useApplicationSlugs';

interface MediaFile {
  id: string;
  filename: string;
  mime_type: string;
  folder_id: string | null;
}

interface ListingResponse {
  message?: {
    folders: unknown[];
    files: MediaFile[];
  };
}

interface Props {
  applicationId: string;
}

// Only image types we allow for a logo. Helps the backend mime check.
const ALLOWED = ['image/png', 'image/jpeg', 'image/svg+xml', 'image/webp', 'image/gif'];

// Pulls the extension from a filename. Falls back to 'png' when there's no dot.
const extFromName = (name: string): string => {
  const m = /\.([a-zA-Z0-9]+)$/.exec(name);
  return (m?.[1] ?? 'png').toLowerCase();
};

/**
 * ApplicationLogo is a self-contained logo uploader that sits on the
 * Edit form of an application. It uploads and deletes instantly (no
 * form submission) and uses the general media subsystem — the file
 * lands at the app's root folder with filename `logo.<ext>` so it's
 * recognisable in the MediaBrowser too.
 */
export const ApplicationLogo: React.FC<Props> = ({ applicationId }) => {
  const { get } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();

  const [logo, setLogo] = useState<MediaFile | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const mediaBase = `applications/{${ApplicationResource}}/{id:${applicationId}}/media`;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = (await get(mediaBase)) as ListingResponse;
      const files = response?.message?.files ?? [];
      const found = files.find(
        f => f.folder_id === null && /^logo\.[a-zA-Z0-9]+$/.test(f.filename)
      );
      setLogo(found ?? null);
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to load logo');
    } finally {
      setLoading(false);
    }
  }, [get, mediaBase, errorMessage]);

  useEffect(() => {
    void load();
  }, [load]);

  const deleteLogo = async (opts: { quiet?: boolean } = {}): Promise<boolean> => {
    if (!logo) return true;
    const token = findAuthToken();
    const res = await window.fetch(`${API_BASE_URL}${mediaBase}/files/{id:${logo.id}}`, {
      method: 'DELETE',
      headers: token ? { Authorization: `Bearer ${token.access_token}` } : undefined,
    });
    if (!res.ok) {
      const txt = await res.text();
      errorMessage(txt || `Failed to remove logo (${res.status})`);
      return false;
    }
    if (!opts.quiet) successMessage('Logo removed');
    setLogo(null);
    return true;
  };

  const handleFile = async (file: File) => {
    if (!ALLOWED.includes(file.type)) {
      errorMessage('Logo must be an image (PNG, JPEG, SVG, WEBP, or GIF)');
      return;
    }
    setBusy(true);
    try {
      // Replace: if there's an existing logo, delete it first so we
      // don't hit the (owner_id, folder_id, filename) unique constraint.
      if (logo) {
        const ok = await deleteLogo({ quiet: true });
        if (!ok) {
          setBusy(false);
          return;
        }
      }
      const ext = extFromName(file.name);
      const renamed = new File([file], `logo.${ext}`, { type: file.type });
      const form = new FormData();
      form.append('file', renamed);

      const token = findAuthToken();
      const res = await window.fetch(`${API_BASE_URL}${mediaBase}/files`, {
        method: 'POST',
        headers: token ? { Authorization: `Bearer ${token.access_token}` } : undefined,
        body: form,
      });
      if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || `Upload failed (${res.status})`);
      }
      successMessage('Logo uploaded');
      await load();
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to upload logo');
    } finally {
      setBusy(false);
    }
  };

  const slugs = useApplicationSlugs(applicationId);
  const logoURL = logo
    ? slugs
      ? slugMediaURL(slugs.organization_id, slugs.workspace_id, slugs.client_id, logo.filename)
      : publicMediaURL(applicationId, logo.filename)
    : null;

  return (
    <div className="p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
      <h3 className="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-2">Logo</h3>
      <p className="text-xs text-gray-500 mb-3">
        Stored in the media library as <code className="text-[0.75rem]">logo.&lt;ext&gt;</code> at
        the application's root folder. Uploading replaces the current logo immediately — no form
        save needed.
      </p>

      {loading ? (
        <div className="text-xs text-gray-500">Loading…</div>
      ) : (
        <div className="flex items-center gap-4 flex-wrap">
          {logoURL ? (
            <img
              src={logoURL}
              alt=""
              className="h-16 w-16 object-contain rounded-lg bg-white border border-gray-200 dark:border-surface-700"
            />
          ) : (
            <div className="h-16 w-16 rounded-lg bg-white border border-dashed border-gray-300 dark:border-surface-600 flex items-center justify-center text-xs text-gray-400">
              no logo
            </div>
          )}

          <div className="flex items-center gap-2">
            <label className="cursor-pointer">
              <span className="inline-flex items-center px-3 py-2 text-sm font-medium rounded-md border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-800 hover:bg-gray-50 dark:hover:bg-surface-700">
                {busy ? 'Working…' : logo ? 'Replace' : 'Upload'}
              </span>
              <input
                type="file"
                accept={ALLOWED.join(',')}
                className="hidden"
                disabled={busy}
                onChange={async e => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  await handleFile(file);
                  e.target.value = '';
                }}
              />
            </label>
            {logo && (
              <button
                type="button"
                onClick={() => void deleteLogo()}
                disabled={busy}
                className="px-3 py-2 text-sm font-medium rounded-md border border-red-200 text-red-600 hover:bg-red-50 disabled:opacity-60"
              >
                Remove
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default ApplicationLogo;
