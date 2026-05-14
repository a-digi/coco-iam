import { describe, it, expect } from 'vitest';
import { flattenWorkspaceApps } from './flattenWorkspaceApps';
import type { Workspace } from '../../../Workspace/model/workspace';
import type { Application } from '../../../Application/model/application';

// Small factory helpers so test bodies only describe what's
// interesting. Unused fields (description, createdAt, isActive) take
// safe defaults.
const ws = (id: string, title: string): Workspace => ({
    id,
    workspaceId: id,
    title,
    description: '',
    organizationId: 'org-1',
    createdAt: '',
    isActive: true,
});

const app = (id: string, title: string, workspaceId: string): Application => ({
    id,
    workspaceId,
    clientId: id,
    title,
    description: '',
    createdAt: '',
    isActive: true,
    allowRecovery: false,
    allowRegistration: false,
});

describe('flattenWorkspaceApps', () => {
    it('returns an empty list when there are no workspaces', () => {
        expect(flattenWorkspaceApps([], {})).toEqual([]);
    });

    it('returns an empty list when workspaces have no apps', () => {
        const workspaces = [ws('ws-1', 'Prod'), ws('ws-2', 'Staging')];
        expect(flattenWorkspaceApps(workspaces, {})).toEqual([]);
    });

    it('prefixes each app label with its workspace title', () => {
        const workspaces = [ws('ws-1', 'Prod')];
        const apps = { 'ws-1': [app('app-1', 'Admin', 'ws-1')] };
        expect(flattenWorkspaceApps(workspaces, apps)).toEqual([
            { id: 'app-1', label: 'Prod › Admin' },
        ]);
    });

    it('preserves input ordering across workspaces and apps', () => {
        const workspaces = [ws('ws-1', 'Prod'), ws('ws-2', 'Staging')];
        const apps = {
            'ws-1': [app('a1', 'Alpha', 'ws-1'), app('a2', 'Beta', 'ws-1')],
            'ws-2': [app('a3', 'Gamma', 'ws-2')],
        };
        expect(flattenWorkspaceApps(workspaces, apps)).toEqual([
            { id: 'a1', label: 'Prod › Alpha' },
            { id: 'a2', label: 'Prod › Beta' },
            { id: 'a3', label: 'Staging › Gamma' },
        ]);
    });

    it('omits workspaces whose entry is missing from the map', () => {
        // The fetch path may partially fail — some workspaces can
        // have an entry, others none. Missing entries should render
        // as no options, not as crashes.
        const workspaces = [ws('ws-1', 'Prod'), ws('ws-2', 'Staging')];
        const apps = { 'ws-1': [app('a1', 'Alpha', 'ws-1')] };
        // ws-2 has no entry in the map.
        expect(flattenWorkspaceApps(workspaces, apps)).toEqual([
            { id: 'a1', label: 'Prod › Alpha' },
        ]);
    });

    it('does not include apps whose workspace is not in the list', () => {
        // Defensive: if the fetcher ever returns apps keyed by a
        // workspace id the admin can't see, the flattener must not
        // leak those into the dropdown.
        const workspaces = [ws('ws-1', 'Prod')];
        const apps = {
            'ws-1': [app('a1', 'Alpha', 'ws-1')],
            'ws-hidden': [app('a2', 'ShouldNotShow', 'ws-hidden')],
        };
        const out = flattenWorkspaceApps(workspaces, apps);
        expect(out.map(o => o.id)).toEqual(['a1']);
    });
});
