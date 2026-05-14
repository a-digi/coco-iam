import type { AdminProfile } from './types';

// profileDisplayName decides what to show in the top-bar user menu
// and on the profile page header. Fallback chain, highest priority
// first:
//
//   1. `"{first_name} {last_name}"` when either is non-empty
//   2. `username` when the profile hasn't been filled in yet
//   3. literal `"User"` as a final safety net
//
// Pure function — exported so both the UserMenu and the ProfilePage
// share the exact same rules. A change here updates both surfaces
// in lockstep.
export function profileDisplayName(profile: AdminProfile | null | undefined): string {
    if (!profile) return 'User';
    const first = (profile.first_name || '').trim();
    const last = (profile.last_name || '').trim();
    if (first || last) {
        return [first, last].filter(Boolean).join(' ');
    }
    const username = (profile.username || '').trim();
    if (username) return username;
    return 'User';
}
