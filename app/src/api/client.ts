import { findAuthToken, saveAuthToken, logoutUser } from '../Components/Auth/Guard/model/auth';
import { renewToken } from './tokenRenew';

// Backend base URL. Sourced from VITE_API_BASE_URL at build
// time (see app/.env) with a localhost fallback so a missing
// env file doesn't brick the dev server. Every endpoint path
// is concatenated onto this, so a trailing slash is required.
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:2026/api/v1/';

// Origin of the API server — used for public, non-versioned endpoints
// (image serving under /p/media and /p/applications/assets).
// Derived from API_BASE_URL by stripping the /api/v1/ suffix.
export const API_ORIGIN = API_BASE_URL.replace(/\/api\/v1\/?$/, '');

// publicMediaURL resolves a root-folder media file for an application
// to its public URL by the internal UUID. Used in admin surfaces that
// already hold the UUID (Edit panels, MediaBrowser). The backend's
// /p/media dispatcher passes UUID-leading requests through unchanged,
// while the slug-trio form (see slugMediaURL) is resolved first.
export const publicMediaURL = (ownerID: string, filename: string): string =>
  `${API_ORIGIN}/p/media/${ownerID}/${filename}`;

// slugMediaURL builds the public, hot-linkable media URL from the
// admin-chosen slug trio (organization.organization_id,
// workspace.workspace_id, applications.client_id). Used by public
// surfaces (login page, emails) so end users never see UUIDs.
export const slugMediaURL = (
  orgSlug: string,
  wsSlug: string,
  clientID: string,
  filename: string,
): string =>
  `${API_ORIGIN}/p/media/${encodeURIComponent(orgSlug)}/${encodeURIComponent(wsSlug)}/${encodeURIComponent(clientID)}/${filename}`;

// publicAssetURL resolves a login-template asset id to its public URL.
// Lives at /p/applications/assets/<id> — outside /api/v1/.
export const publicAssetURL = (assetID: string): string =>
  `${API_ORIGIN}/p/applications/assets/${assetID}`;

// absolutePublicURL turns a relative /p/... path returned by the
// backend into an absolute URL pointing at the API origin.
export const absolutePublicURL = (relativePath: string): string => {
  if (!relativePath) return '';
  if (/^https?:\/\//.test(relativePath)) return relativePath;
  return `${API_ORIGIN}${relativePath.startsWith('/') ? '' : '/'}${relativePath}`;
};

export async function handleResponse(res: Response): Promise<unknown> {
  const contentType = res.headers.get('content-type') || '';
  let data: unknown = null;
  try {
    if (contentType.includes('application/json')) {
      data = await res.json();
    } else {
      data = await res.text();
    }
  } catch {
    // ignore
  }
  if (!res.ok) {
    let message: string | undefined;
    if (typeof data === 'string') {
      message = data;
    } else if (typeof data === 'object' && data !== null) {
      message = (data as { message?: string }).message || (data as { error?: string }).error;
    }
    message = message || res.statusText;

    throw new Error(message || `Request failed with status ${res.status}`);
  }
  return data;
}

export async function postMultipart(path: string, formData: FormData, init?: RequestInit) {
  // Multipart is special: we must NOT set Content-Type ourselves —
  // the browser adds `multipart/form-data; boundary=…` automatically
  // and an explicit Content-Type (as `buildHeaders` would set) would
  // clobber the boundary. But we still need the bearer token, plus
  // the same "renew if expired" dance `buildHeaders` does for JSON
  // requests, otherwise the server sees no credentials on uploads.
  const headers: Record<string, string> = {};
  let token = findAuthToken();
  if (token) {
    const now = Math.floor(Date.now() / 1000);
    if (token.expires_at < now) {
      try {
        token = await renewToken(token.access_token);
        saveAuthToken(token);
      } catch {
        logoutUser();
        throw new Error('Session abgelaufen. Bitte erneut anmelden.');
      }
    }
    if (token && token.access_token) {
      headers['Authorization'] = `Bearer ${token.access_token}`;
    }
  }
  // Merge any caller-supplied headers last so they can override /
  // supplement ours (e.g. custom tracing ids). Keep the
  // no-Content-Type rule by never injecting one.
  if (init?.headers) {
    if (Array.isArray(init.headers)) {
      init.headers.forEach(h => { headers[h[0]] = h[1]; });
    } else if (init.headers instanceof Headers) {
      init.headers.forEach((v, k) => { headers[k] = v; });
    } else {
      Object.assign(headers, init.headers);
    }
  }

  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    body: formData,
    ...init,
    headers,
  });
  return handleResponse(res);
}

export async function buildHeaders(initHeaders?: HeadersInit): Promise<HeadersInit> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  let token = findAuthToken();
  if (token) {
    const now = Math.floor(Date.now() / 1000);
    if (token.expires_at < now) {
      try {
        token = await renewToken(token.access_token);
        saveAuthToken(token);
      } catch {
        logoutUser();
        throw new Error('Session abgelaufen. Bitte erneut anmelden.');
      }
    }
    if (token && token.access_token) {
      headers['Authorization'] = `Bearer ${token.access_token}`;
    }
  }

  if (initHeaders) {
    if (Array.isArray(initHeaders)) {
      initHeaders.forEach(header => {
        headers[header[0]] = header[1];
      });
    } else if (initHeaders instanceof Headers) {
      initHeaders.forEach((value, key) => {
        headers[key] = value;
      });
    } else {
      Object.assign(headers, initHeaders);
    }
  }

  return headers;
}

export async function post<T = unknown>(path: string, data: T, init?: RequestInit) {
  const headers = await buildHeaders(init?.headers);
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(data),
    ...init,
  });
  return handleResponse(res);
}

export async function postPublicApi<T = unknown>(path: string, data: T, init?: RequestInit) {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    body: JSON.stringify(data),
    ...init,
  });

  return handleResponse(res);
}

export async function put<T = unknown>(path: string, data: T, init?: RequestInit) {
  const headers = await buildHeaders(init?.headers);
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'PUT',
    headers,
    body: JSON.stringify(data),
    ...init,
  });
  return handleResponse(res);
}

export async function patch<T = unknown>(path: string, data: T, init?: RequestInit) {
  const headers = await buildHeaders(init?.headers);
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'PATCH',
    headers,
    body: JSON.stringify(data),
    ...init,
  });
  return handleResponse(res);
}

export async function del(path: string, init?: RequestInit) {
  const headers = await buildHeaders(init?.headers);
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'DELETE',
    headers,
    ...init,
  });
  return handleResponse(res);
}