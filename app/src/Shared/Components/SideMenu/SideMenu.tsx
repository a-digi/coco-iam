import React, { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import ScopeBasedComponentAccess from '../Access/ScopeBasedComponentAccess';

/**
 * SideMenu is a left-nav-plus-content panel. Each entry has an id, a
 * label, and the content node that renders when the entry is active.
 * Active selection is local state — callers that want URL-routed
 * selection should use `react-router` directly instead.
 *
 * An entry with `scopes` is hidden from the nav when the current
 * caller doesn't hold one of those scopes (same predicate as
 * `ScopeBasedComponentAccess`). The content is automatically swapped
 * to the first visible entry when the active one is hidden — no
 * broken "empty panel" states.
 *
 * An entry may set `href` instead of `content`: it then renders as a
 * router `Link` (navigating away from the current page) rather than
 * as a button that swaps the active panel. Use this for "deep-dive"
 * sections that live on their own route.
 *
 * An entry may carry `children` to render a collapsible sub-list of
 * further SideMenuItems. Children default to expanded/visible; a
 * chevron on the parent row toggles their visibility. Parents may
 * themselves still carry `content` or `href` (the label click
 * navigates, the chevron collapses) or be "pure group headers" with
 * neither (the whole row toggles collapse).
 */
export interface SideMenuItem {
    id: string;
    label: string;
    content?: React.ReactNode;
    href?: string;
    scopes?: string[];
    // Nested items render indented under the parent and can be
    // hidden via the chevron on the parent row. Omitting this
    // field (or passing an empty array) preserves the flat
    // behaviour the component had before nested support existed.
    children?: SideMenuItem[];
}

interface Props {
    items: SideMenuItem[];
    initialActiveId?: string;
    // Narrow width column for the nav. Tailwind grid-cols utilities
    // don't accept dynamic values well, so we fall back to a short
    // list of preset widths.
    width?: 'sm' | 'md' | 'lg';
    ariaLabel?: string;
    className?: string;
}

const WIDTHS: Record<NonNullable<Props['width']>, string> = {
    sm: 'md:grid-cols-[180px_1fr]',
    md: 'md:grid-cols-[240px_1fr]',
    lg: 'md:grid-cols-[300px_1fr]',
};

// flattenPanelItems walks the tree and returns only the entries
// that own their own content panel — i.e. the nodes eligible for
// active-selection. Used both to compute the initial active id
// fallback and to resolve the currently-rendered panel node.
function flattenPanelItems(items: SideMenuItem[]): SideMenuItem[] {
    const out: SideMenuItem[] = [];
    const walk = (list: SideMenuItem[]) => {
        for (const it of list) {
            if (!it.href) out.push(it);
            if (it.children && it.children.length > 0) walk(it.children);
        }
    };
    walk(items);
    return out;
}

export const SideMenu: React.FC<Props> = ({
    items,
    initialActiveId,
    width = 'md',
    ariaLabel = 'Sections',
    className,
}) => {
    // Only items that own a content panel participate in active-selection.
    const panelItems = useMemo(() => flattenPanelItems(items), [items]);
    const firstPanelId = panelItems[0]?.id ?? '';

    // `selectedId` is the user's last explicit click. `activeId` is
    // derived on render so a stale selection (e.g. when the items list
    // changes and the previously-active entry disappears) transparently
    // falls back to `initialActiveId` / first panel — no useEffect.
    const [selectedId, setSelectedId] = useState<string | undefined>(initialActiveId);
    const activeId = selectedId && panelItems.some(i => i.id === selectedId)
        ? selectedId
        : (initialActiveId && panelItems.some(i => i.id === initialActiveId) ? initialActiveId : firstPanelId);

    const active = useMemo(
        () => panelItems.find(i => i.id === activeId) ?? panelItems[0],
        [panelItems, activeId],
    );

    // On mobile, all parents start collapsed so the nav doesn't
    // flood the screen. On desktop they start expanded. Uses a
    // lazy initializer (same pattern as SidebarContext) so it
    // reads window.innerWidth once at mount with no effect needed.
    const [collapsed, setCollapsed] = useState<Set<string>>(() => {
        if (typeof window !== 'undefined' && window.innerWidth < 768) {
            const ids = new Set<string>();
            const walk = (list: SideMenuItem[]) => {
                for (const it of list) {
                    if (it.children && it.children.length > 0) {
                        ids.add(it.id);
                        walk(it.children);
                    }
                }
            };
            walk(items);
            return ids;
        }
        return new Set<string>();
    });
    const toggleCollapsed = (id: string) => {
        setCollapsed(prev => {
            const next = new Set(prev);
            if (next.has(id)) {
                next.delete(id);
            } else {
                next.add(id);
            }
            return next;
        });
    };

    const wrapperCls = [
        'grid grid-cols-1 gap-6',
        WIDTHS[width],
        className,
    ].filter(Boolean).join(' ');

    return (
        <div className={wrapperCls}>
            <aside className="md:border-r md:border-gray-200 dark:md:border-surface-800 md:pr-4">
                <nav className="space-y-1" aria-label={ariaLabel}>
                    {items.map(item => (
                        <NavTreeEntry
                            key={item.id}
                            item={item}
                            activeId={active?.id}
                            depth={0}
                            collapsed={collapsed}
                            onToggleCollapsed={toggleCollapsed}
                            onSelect={setSelectedId}
                        />
                    ))}
                </nav>
            </aside>
            <main className="min-w-0">
                {active?.content}
            </main>
        </div>
    );
};

// -- NavTreeEntry --------------------------------------------------

interface NavTreeEntryProps {
    item: SideMenuItem;
    activeId: string | undefined;
    depth: number;
    collapsed: Set<string>;
    onToggleCollapsed: (id: string) => void;
    onSelect: (id: string) => void;
}

// NavTreeEntry renders one item and recursively its children.
// Recursion is terminated by the absence of `children`.
// Scope filtering applies at each level independently: a
// scope-gated parent hides its subtree entirely; a scope-gated
// child hides only itself.
const NavTreeEntry: React.FC<NavTreeEntryProps> = ({
    item, activeId, depth, collapsed, onToggleCollapsed, onSelect,
}) => {
    const hasChildren = !!item.children && item.children.length > 0;
    const isExpanded = !collapsed.has(item.id);
    const isActive = !item.href && item.id === activeId;
    // A "pure group" has no panel AND no href. Clicking anywhere
    // on its row toggles child visibility since there's nothing
    // else the click could do.
    const isPureGroup = hasChildren && !item.content && !item.href;

    const indentCls = depth === 0 ? '' : depth === 1 ? 'pl-6' : depth === 2 ? 'pl-10' : 'pl-14';
    const navClass = [
        'flex items-center w-full text-left px-3 py-2 rounded-md text-sm transition-colors',
        indentCls,
        isActive
            ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-200 font-medium'
            : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800',
    ].filter(Boolean).join(' ');

    const chevron = hasChildren ? (
        <button
            type="button"
            onClick={e => {
                e.stopPropagation();
                onToggleCollapsed(item.id);
            }}
            aria-label={isExpanded ? 'Hide children' : 'Show children'}
            aria-expanded={isExpanded}
            className="ml-auto shrink-0 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 px-1"
        >
            <CaretIcon open={isExpanded} />
        </button>
    ) : null;

    let node: React.ReactNode;
    if (item.href) {
        node = (
            <div className={navClass}>
                <Link to={item.href} className="flex-1 -mx-3 -my-2 px-3 py-2 rounded-md">
                    {item.label}
                </Link>
                {chevron}
            </div>
        );
    } else if (isPureGroup) {
        // No content + no href: the row is a collapsible group
        // header. Clicking the row itself toggles children.
        node = (
            <button
                type="button"
                onClick={() => onToggleCollapsed(item.id)}
                className={navClass}
                aria-expanded={isExpanded}
            >
                <span className="flex-1">{item.label}</span>
                <span className="ml-auto shrink-0 text-gray-400" aria-hidden="true">
                    <CaretIcon open={isExpanded} />
                </span>
            </button>
        );
    } else {
        node = (
            <div className={navClass}>
                <button
                    type="button"
                    onClick={() => onSelect(item.id)}
                    className="flex-1 text-left -mx-3 -my-2 px-3 py-2 rounded-md"
                >
                    {item.label}
                </button>
                {chevron}
            </div>
        );
    }

    const wrapped = item.scopes && item.scopes.length > 0 ? (
        <ScopeBasedComponentAccess requiredScopes={item.scopes}>
            <NavNodeWrapper>{node}</NavNodeWrapper>
        </ScopeBasedComponentAccess>
    ) : node;

    return (
        <>
            {wrapped}
            {hasChildren && isExpanded && (
                <div className="space-y-1">
                    {item.children!.map(child => (
                        <NavTreeEntry
                            key={child.id}
                            item={child}
                            activeId={activeId}
                            depth={depth + 1}
                            collapsed={collapsed}
                            onToggleCollapsed={onToggleCollapsed}
                            onSelect={onSelect}
                        />
                    ))}
                </div>
            )}
        </>
    );
};

// CaretIcon is the same chevron the main app sidebar
// (`Components/Menu/Sidebar/Sidebar.tsx`) uses for its
// group-open indicator — a 4x4 outlined caret that rotates 90°
// from pointing-right to pointing-down. Kept in sync so every
// collapsible nav surface in the app reads the same.
const CaretIcon: React.FC<{ open: boolean }> = ({ open }) => (
    <svg
        className="w-4 h-4 transition-transform duration-200 shrink-0"
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

// NavNodeWrapper absorbs the `accessMe` prop that
// ScopeBasedComponentAccess injects via cloneElement so it never
// reaches the underlying DOM element (plain <button> / <Link>),
// which would otherwise trigger a "React does not recognize the
// accessMe prop on a DOM element" warning.
const NavNodeWrapper: React.FC<{ accessMe?: boolean; children: React.ReactNode }> = ({
    accessMe: _accessMe,
    children,
}) => <>{children}</>;

export default SideMenu;
