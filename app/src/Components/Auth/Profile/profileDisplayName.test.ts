import { describe, it, expect } from 'vitest';
import { profileDisplayName } from './profileDisplayName';
import type { AdminProfile } from './types';

const base: AdminProfile = {
    id: 'uuid',
    username: 'alice',
    email: 'alice@example.com',
    is_super_admin: false,
    is_active: true,
    first_name: '',
    last_name: '',
    phone: '',
    locale: '',
    timezone: '',
    avatar_url: '',
};

describe('profileDisplayName', () => {
    it('returns full name when both first and last are set', () => {
        expect(profileDisplayName({ ...base, first_name: 'Alice', last_name: 'Parker' }))
            .toBe('Alice Parker');
    });

    it('uses first name alone when last is empty', () => {
        expect(profileDisplayName({ ...base, first_name: 'Alice' })).toBe('Alice');
    });

    it('uses last name alone when first is empty', () => {
        expect(profileDisplayName({ ...base, last_name: 'Parker' })).toBe('Parker');
    });

    it('trims whitespace from names', () => {
        // A stray leading/trailing space typed in the form
        // shouldn't show up in the header — the display name
        // collapses whitespace that the user didn't intend.
        expect(profileDisplayName({ ...base, first_name: '  Alice  ', last_name: '  Parker  ' }))
            .toBe('Alice Parker');
    });

    it('falls back to username when both names are empty', () => {
        // Fresh admin who hasn't filled in their profile yet — we
        // still need something identifiable in the top bar.
        expect(profileDisplayName({ ...base, username: 'alice' })).toBe('alice');
    });

    it('falls back to literal "User" when everything is empty', () => {
        expect(profileDisplayName({ ...base, username: '' })).toBe('User');
    });

    it('treats whitespace-only names as empty', () => {
        expect(profileDisplayName({ ...base, first_name: '   ', last_name: '' })).toBe('alice');
    });

    it('returns "User" for null / undefined input', () => {
        expect(profileDisplayName(null)).toBe('User');
        expect(profileDisplayName(undefined)).toBe('User');
    });
});
