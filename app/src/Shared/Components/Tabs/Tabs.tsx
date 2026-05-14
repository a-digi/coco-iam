import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { ScopeBasedComponentAccess } from '../Access/ScopeBasedComponentAccess';

export interface TabData {
    id: string;
    title: string;
    content?: React.ReactNode;
    disabled?: boolean;
    scopes?: string[];
    // If set, the tab renders as a router Link — clicking it navigates away
    // instead of selecting a local panel. Useful for "deep-dive" sections
    // that live on their own route (e.g. /workspaces/{id}/applications).
    href?: string;
}

export type TabsVariant = 'default' | 'pills';

export interface TabsProps {
    items: TabData[];
    initialActiveId?: string;
    className?: string;
    onChange?: (id: string) => void;
    variant?: TabsVariant;
}

const PILL_COLORS: { active: string; inactive: string }[] = [
    {
        active:   'bg-violet-500 text-white shadow-sm',
        inactive: 'bg-violet-100 text-violet-600 dark:bg-violet-900/30 dark:text-violet-300 hover:bg-violet-200 dark:hover:bg-violet-900/50',
    },
    {
        active:   'bg-blue-500 text-white shadow-sm',
        inactive: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-900/50',
    },
    {
        active:   'bg-rose-500 text-white shadow-sm',
        inactive: 'bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-300 hover:bg-rose-200 dark:hover:bg-rose-900/50',
    },
    {
        active:   'bg-amber-500 text-white shadow-sm',
        inactive: 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300 hover:bg-amber-200 dark:hover:bg-amber-900/50',
    },
    {
        active:   'bg-teal-500 text-white shadow-sm',
        inactive: 'bg-teal-100 text-teal-600 dark:bg-teal-900/30 dark:text-teal-300 hover:bg-teal-200 dark:hover:bg-teal-900/50',
    },
];

export const Tabs: React.FC<TabsProps> = ({
    items,
    initialActiveId,
    className = '',
    onChange,
    variant = 'default',
}) => {
    const isSelectable = (i: TabData) => !i.disabled && !i.href;
    const firstEnabled = items.find(isSelectable)?.id;

    // `selectedId` holds the user's last explicit selection. `activeId`
    // is derived on every render so a stale selection (e.g. when the
    // items list changes and the previously-selected tab disappears)
    // transparently falls back to the first selectable tab — no
    // useEffect-based state sync needed.
    const [selectedId, setSelectedId] = useState<string | undefined>(initialActiveId);
    const activeId = selectedId && items.some(i => i.id === selectedId && isSelectable(i))
        ? selectedId
        : firstEnabled;

    const handleSelect = (id: string) => {
        setSelectedId(id);
        onChange?.(id);
    };

    const activeItem = items.find(i => i.id === activeId);

    const isPills = variant === 'pills';

    return (
        <div className={`w-full ${className}`}>
            <div
                role="tablist"
                className={
                    isPills
                        ? 'flex flex-nowrap overflow-x-auto gap-2 pb-3 mb-2 scrollbar-hide'
                        : 'flex flex-wrap gap-1 border-b border-gray-200 dark:border-surface-900 mb-4'
                }
            >
                {items.map((item, idx) => {
                    const isActive = item.id === activeId;
                    const pillColor = PILL_COLORS[idx % PILL_COLORS.length];

                    const tabClasses = isPills
                        ? [
                            'shrink-0 px-4 py-2 rounded-full text-sm font-semibold transition-all duration-200 focus:outline-none whitespace-nowrap',
                            item.disabled
                                ? 'opacity-40 cursor-not-allowed bg-gray-100 text-gray-400'
                                : isActive
                                    ? pillColor.active
                                    : pillColor.inactive,
                          ].join(' ')
                        : [
                            'px-4 py-2 -mb-px text-sm font-medium transition-colors border-b-2 focus:outline-none',
                            item.disabled
                                ? 'border-transparent text-gray-300 dark:text-gray-600 cursor-not-allowed'
                                : item.href
                                    ? 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 hover:border-gray-300 dark:hover:border-gray-600'
                                    : isActive
                                        ? 'border-indigo-500 text-indigo-600 dark:text-indigo-400'
                                        : 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 hover:border-gray-300 dark:hover:border-gray-600',
                          ].join(' ');

                    const tabNode = item.href && !item.disabled ? (
                        <Link
                            key={item.id}
                            to={item.href}
                            role="tab"
                            id={`tab-${item.id}`}
                            className={tabClasses}
                        >
                            {item.title}
                        </Link>
                    ) : (
                        <button
                            key={item.id}
                            type="button"
                            role="tab"
                            aria-selected={isActive}
                            aria-controls={`tab-panel-${item.id}`}
                            id={`tab-${item.id}`}
                            disabled={item.disabled}
                            onClick={() => !item.disabled && handleSelect(item.id)}
                            className={tabClasses}
                        >
                            {item.title}
                        </button>
                    );

                    if (item.scopes && item.scopes.length > 0) {
                        return (
                            <ScopeBasedComponentAccess key={item.id} requiredScopes={item.scopes}>
                                {tabNode}
                            </ScopeBasedComponentAccess>
                        );
                    }

                    return tabNode;
                })}
            </div>

            {activeItem && activeItem.content !== undefined && (
                <div
                    role="tabpanel"
                    id={`tab-panel-${activeItem.id}`}
                    aria-labelledby={`tab-${activeItem.id}`}
                    className="py-2"
                >
                    {activeItem.content}
                </div>
            )}
        </div>
    );
};

export default Tabs;
