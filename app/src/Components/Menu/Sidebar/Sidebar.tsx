import React, { useState } from 'react';
import type { MenuItem } from '../type/type';
import { defaultMenuItems } from '../../../config/menu/menu.ts';
import { useMenu } from '../useMenu.tsx';
import { useSidebar } from '../../../Layout/Sidebar/SidebarContextContext.ts';
import { useAuth } from '../../Auth/Guard/useAuth.ts';
import { parseJwt } from '../../../config/security/jtw.ts';
import { AppScopes } from '../../../config/security/scopes.ts';
import { useHttpClient } from '../../../api/http/useHttpClient.ts';
import { MenuIcon } from './MenuIcon';

const CaretIcon = ({ open }: { open: boolean }) => (
  <svg
    className="ml-2 w-4 h-4 transition-transform duration-200 shrink-0 text-gray-700 dark:text-white"
    style={{ transform: open ? 'rotate(90deg)' : 'rotate(0deg)' }}
    fill="none"
    stroke="currentColor"
    strokeWidth={2}
    viewBox="0 0 24 24"
    aria-hidden="true"
  >
    <path d="M9 5l7 7-7 7" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

const SideBarMenu: React.FC = () => {
  const [openMenus, setOpenMenus] = useState<{ [key: string]: boolean }>({});
  const { menuItems, setMenuItems } = useMenu();
  const { open } = useSidebar();
  const { authToken } = useAuth();
  const { get } = useHttpClient();
  // `null` = not yet fetched; a number = known count. We treat "not
  // yet fetched" as "hide the conditional items" so a brief flicker
  // on first paint can't put the user into a broken state (clicking
  // Workspaces before we know the org count).
  const [orgCount, setOrgCount] = useState<number | null>(null);

  // Fetch the current org count once per auth session. The menu
  // re-filters whenever this changes, so creating the first org on
  // /organizations reveals Workspaces on the next auth refresh or
  // full page load. Fresh-data-on-every-click isn't needed here.
  React.useEffect(() => {
    if (!authToken) {
      setOrgCount(null);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const resp = await get<{ message?: unknown }>(`organizations/{res:organizations}`);
        if (cancelled) return;
        const data = resp?.message ?? resp ?? [];
        setOrgCount(Array.isArray(data) ? data.length : 0);
      } catch {
        // On error (network blip, scope denied) play it safe: treat
        // as zero so conditional items stay hidden rather than
        // dangling.
        if (!cancelled) setOrgCount(0);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [authToken, get]);

  React.useEffect(() => {
    if (!authToken) {
      setMenuItems([]);
      return;
    }

    const payload = parseJwt(authToken.access_token);

    if (!payload) {
      setMenuItems([]);
      return;
    }

    let userScopes: string[] = [];
    if (Array.isArray(payload.scopes)) {
      userScopes = payload.scopes;
    } else if (typeof payload.scope === 'string') {
      userScopes = payload.scope.split(' ').filter(Boolean);
    }

    const isSuperAdmin = userScopes.includes(AppScopes.SuperAdmin);
    const hasOrganizations = typeof orgCount === 'number' && orgCount > 0;

    const hasAccess = (item: MenuItem): boolean => {
      if (isSuperAdmin) return true;
      if (!item.accessScopes || item.accessScopes.length === 0) return true;
      if (item.accessScopes.includes(AppScopes.UserMe)) return true;
      return item.accessScopes.some(scope => userScopes.includes(scope));
    };

    const filterMenu = (items: MenuItem[]): MenuItem[] => {
      const filtered: MenuItem[] = [];
      for (const item of items) {
        // Organization-gated entries: hidden until the org count is
        // known and greater than zero. Applied before the scope
        // check so the filter short-circuits cleanly.
        if (item.hideWhenNoOrganizations && !hasOrganizations) {
          continue;
        }
        const itemAllowed = hasAccess(item);
        let newChildren: MenuItem[] | undefined;
        let childrenAllowed = false;

        if (item.children) {
          newChildren = filterMenu(item.children);
          childrenAllowed = newChildren.length > 0;
        }

        if (item.children) {
          if (childrenAllowed) {
            filtered.push({ ...item, children: newChildren, href: itemAllowed ? item.href : undefined });
          }
        } else {
          if (itemAllowed) filtered.push({ ...item });
        }
      }
      return filtered;
    };

    setMenuItems(filterMenu(defaultMenuItems));
  }, [authToken, orgCount, setMenuItems]);

  const handleToggle = (name: string) => {
    setOpenMenus(prev => ({ ...prev, [name]: !prev[name] }));
  };

  // Collapsed sidebar: top-level icon buttons with tooltip
  if (!open) {
    return (
      <nav>
        <ul className="flex flex-col items-center gap-1">
          {menuItems.map(item => (
            <li key={item.name} className="w-full">
              {item.href ? (
                <a
                  href={item.href}
                  title={item.name}
                  className="flex items-center justify-center w-full h-9 rounded-lg text-gray-500 hover:text-indigo-600 hover:bg-indigo-50 dark:text-gray-400 dark:hover:text-indigo-400 dark:hover:bg-surface-800 transition-colors"
                >
                  <MenuIcon name={item.icon} className="w-6 h-6" />
                </a>
              ) : (
                <button
                  type="button"
                  title={item.name}
                  onClick={() => handleToggle(item.name)}
                  className="flex items-center justify-center w-full h-9 rounded-lg text-gray-500 hover:text-indigo-600 hover:bg-indigo-50 dark:text-gray-400 dark:hover:text-indigo-400 dark:hover:bg-surface-800 transition-colors"
                >
                  <MenuIcon name={item.icon} className="w-6 h-6" />
                </button>
              )}
            </li>
          ))}
        </ul>
      </nav>
    );
  }

  // Open sidebar: icon + label, full tree
  const renderMenu = (items: MenuItem[], parentKey = '', depth = 0) => (
    <ul className="space-y-1">
      {items.map(item => {
        const hasChildren = item.children && item.children.length > 0;
        const key = parentKey + item.name;
        return (
          <li key={key}>
            <div className="flex items-center">
              {item.href ? (
                <a
                  href={item.href}
                  className="flex items-center gap-2.5 flex-1 px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-surface-800 transition-colors text-[0.9375rem]"
                >
                  {depth === 0 && <MenuIcon name={item.icon} className="w-5 h-5" />}
                  {item.name}
                </a>
              ) : (
                <button
                  onClick={() => hasChildren && handleToggle(key)}
                  className="flex items-center gap-2.5 flex-1 px-3 py-2 rounded-lg text-gray-700 dark:text-gray-200 font-semibold text-left hover:bg-gray-100 dark:hover:bg-surface-800 transition-colors text-[0.9375rem]"
                >
                  {depth === 0 && <MenuIcon name={item.icon} className="w-5 h-5" />}
                  {item.name}
                </button>
              )}
              {hasChildren && (
                <button
                  type="button"
                  className="ml-auto px-1 focus:outline-none"
                  onClick={() => handleToggle(key)}
                  aria-label={`Toggle ${item.name}`}
                >
                  <CaretIcon open={!!openMenus[key]} />
                </button>
              )}
            </div>
            {hasChildren && openMenus[key] && (
              <div className="ml-4 mt-1 border-l border-gray-100 dark:border-surface-700 pl-2">
                {renderMenu(item.children!, key, depth + 1)}
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );

  return (
    <aside className="h-full flex flex-col">
      <nav>
        {renderMenu(menuItems)}
      </nav>
    </aside>
  );
};

export default SideBarMenu;
