import type { Workspace } from '../../../Workspace/model/workspace';
import type { Application } from '../../../Application/model/application';

export interface AppOption {
    id: string;
    label: string;
}

// flattenWorkspaceApps turns a list of workspaces and a per-workspace
// list of applications into the flat, display-ready option list used
// by the "Send to application after activation" dropdown.
//
// Pure function — no I/O, no React. Keeping it out of the component
// lets the mapping be unit-tested without mocking useHttpClient. The
// input is whatever the form's useEffect fetched.
export function flattenWorkspaceApps(
    workspaces: ReadonlyArray<Workspace>,
    appsByWorkspaceID: Readonly<Record<string, ReadonlyArray<Application>>>,
): AppOption[] {
    const out: AppOption[] = [];
    for (const ws of workspaces) {
        const apps = appsByWorkspaceID[ws.id] ?? [];
        for (const app of apps) {
            out.push({
                id: app.id,
                label: `${ws.title} › ${app.title}`,
            });
        }
    }
    return out;
}
