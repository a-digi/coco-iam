import { useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { ApplicationResource } from '../model/application';

// Admin-chosen identifier trio for an application. Admin UIs use these
// to build slug-based public URLs (media, login page, …) without
// leaking UUIDs into the browser.
export interface ApplicationSlugs {
  organization_id: string;
  workspace_id: string;
  client_id: string;
}

interface SlugsResponse {
  message?: ApplicationSlugs;
}

/**
 * useApplicationSlugs fetches the (organization_id, workspace_id,
 * client_id) trio for the application row identified by `applicationId`
 * (UUID). Returns `null` while loading or when the lookup fails — the
 * caller can fall back to the UUID-based URL form in that case.
 */
export function useApplicationSlugs(applicationId: string): ApplicationSlugs | null {
  const { get } = useHttpClient();
  const [slugs, setSlugs] = useState<ApplicationSlugs | null>(null);

  useEffect(() => {
    if (!applicationId) return;
    // `cancelled` guards against a late response overwriting state
    // after the caller unmounts or the applicationId changes.
    let cancelled = false;
    (async () => {
      try {
        const resp = await get<SlugsResponse>(
          `applications/{${ApplicationResource}}/{id:${applicationId}}/slugs`,
        );
        if (cancelled) return;
        const body = resp?.message;
        if (body?.organization_id && body?.workspace_id && body?.client_id) {
          setSlugs(body);
        } else {
          setSlugs(null);
        }
      } catch {
        if (!cancelled) setSlugs(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [applicationId, get]);

  // When applicationId is empty, short-circuit on render rather than
  // via a synchronous setState inside the effect — that keeps the
  // effect purely asynchronous and avoids the set-state-in-effect
  // cascading-render warning.
  return applicationId ? slugs : null;
}
