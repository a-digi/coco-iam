export interface ProfileMenuEntry {
    label: string;
    href: string;
}

export const DEFAULT_PROFILE_MENU: ProfileMenuEntry[] = [
    {
        label: 'General',
        href: '/profile',
    },
    {
        label: 'Security',
        href: '/profile/security',
    },
];
