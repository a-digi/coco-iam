import React from 'react';

const paths: Record<string, string> = {
  dashboard:     'M3 3h7v7H3V3zm11 0h7v7h-7V3zM3 14h7v7H3v-7zm11 0h7v7h-7v-7z',
  workspaces:    'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
  organizations: 'M3 21h18M6 21V7a2 2 0 012-2h8a2 2 0 012 2v14M9 21V11m6 10V11M9 7h.01M15 7h.01',
  admin:         'M12 2l8 4v6c0 5-3.5 8.5-8 10-4.5-1.5-8-5-8-10V6l8-4z',
  users:         'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z',
  groups:        'M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2M9 11a4 4 0 100-8 4 4 0 000 8zm14 10v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75',
  queue:         'M4 6h16M4 12h16M4 18h7',
  observe:       'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
  settings:      'M12 15a3 3 0 100-6 3 3 0 000 6zm6.364-1.636a7 7 0 000-3.728l1.979-1.979a9.955 9.955 0 000-2.314l-1.979-1.979a7 7 0 000-3.728L20.343 2.343a9.955 9.955 0 00-2.314 0L16.05 4.322a7 7 0 00-3.728 0L10.343 2.343a9.955 9.955 0 00-2.314 0L6.05 4.322a7 7 0 00-3.728 0L.343 6.3a9.955 9.955 0 000 2.314l1.979 1.979a7 7 0 000 3.728L.343 16.3a9.955 9.955 0 000 2.314l1.979 1.979a7 7 0 003.728 0l1.979 1.979a9.955 9.955 0 002.314 0l1.979-1.979a7 7 0 003.728 0l1.979 1.979a9.955 9.955 0 002.314 0l1.979-1.979z',
  security:      'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z',
};

interface MenuIconProps {
  name?: string;
  className?: string;
}

export const MenuIcon: React.FC<MenuIconProps> = ({ name, className = 'w-4 h-4' }) => {
  const d = name ? paths[name] : undefined;
  if (!d) return null;
  return (
    <svg
      className={`${className} shrink-0`}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={d} />
    </svg>
  );
};

export default MenuIcon;
