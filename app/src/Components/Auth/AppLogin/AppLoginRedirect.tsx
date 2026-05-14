import React, { useEffect } from 'react';

interface Props {
  /** Absolute URL where the browser should land after a successful
   *  workspace-application login. Always set when this component
   *  mounts — the parent only renders it after the authenticate call
   *  succeeds. */
  redirectURL: string;
  /** Display name of the target application, shown in the progress
   *  copy ("Redirecting to <name>…"). Optional — falls back to a
   *  generic phrasing when missing. */
  applicationName?: string;
}

/**
 * AppLoginRedirect is the success-state view of the workspace
 * application login flow. Key design choices:
 *
 *   - No bearer token is ever held by this component or stored in
 *     localStorage. The app-scoped JWT was delivered server-to-server
 *     by the backend's dispatchRedirect call; the browser only sees
 *     the destination URL.
 *   - This flow is deliberately independent from the admin auth
 *     context (useAuth / AuthProvider). Admin login populates
 *     AUTH_TOKEN_KEY in localStorage to drive the coco-iam admin UI;
 *     workspace-application login does not touch that context.
 */
const AppLoginRedirect: React.FC<Props> = ({ redirectURL, applicationName }) => {
  useEffect(() => {
    if (!redirectURL) return;
    // window.location.assign triggers a full navigation and drops
    // this SPA, which is the point — we're handing control to the
    // target application.
    window.location.assign(redirectURL);
  }, [redirectURL]);

  const target = applicationName?.trim() || 'your application';

  return (
    <div className="min-h-screen w-full flex items-center justify-center bg-gray-50 dark:bg-surface-900 p-6">
      <div className="w-full max-w-sm text-center">
        <div className="mx-auto h-12 w-12 rounded-full border-2 border-gray-200 border-t-indigo-600 animate-spin" />
        <h1 className="mt-5 text-lg font-semibold text-gray-900 dark:text-gray-100">
          Signed in — redirecting you to {target}…
        </h1>
        <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
          If this page doesn't navigate automatically,{' '}
          <a className="text-indigo-600 hover:underline" href={redirectURL}>
            continue here
          </a>
          .
        </p>
      </div>
    </div>
  );
};

export default AppLoginRedirect;
