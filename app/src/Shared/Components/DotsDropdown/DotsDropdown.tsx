import React, { useRef, useState, useEffect } from 'react';
import { Link } from 'react-router-dom';

export interface DotsDropdownItem {
  label: string;
  icon?: React.ReactNode;
  onClick?: () => void;
  href?: string;
  variant?: 'default' | 'danger';
}

interface DotsDropdownProps {
  items: DotsDropdownItem[];
  align?: 'left' | 'right';
}

export const DotsDropdown: React.FC<DotsDropdownProps> = ({ items, align = 'right' }) => {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  return (
    <div ref={ref} className="relative inline-block">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        aria-label="Actions"
        className="flex items-center justify-center w-8 h-8 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 dark:text-gray-500 dark:hover:text-gray-200 dark:hover:bg-surface-700 transition-colors"
      >
        <svg viewBox="0 0 24 24" fill="currentColor" className="w-5 h-5">
          <circle cx="12" cy="5" r="1.5" />
          <circle cx="12" cy="12" r="1.5" />
          <circle cx="12" cy="19" r="1.5" />
        </svg>
      </button>

      {open && (
        <div
          className={`
            absolute z-50 mt-1 min-w-[11rem] py-1
            bg-white dark:bg-surface-800
            border border-gray-100 dark:border-surface-700
            rounded-xl shadow-lg
            ${align === 'right' ? 'right-0' : 'left-0'}
          `}
        >
          {items.map((item, i) => {
            const base = `
              flex items-center gap-2.5 w-full px-4 py-2 text-sm transition-colors
              ${item.variant === 'danger'
                ? 'text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20'
                : 'text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-surface-700'
              }
            `;
            const content = (
              <>
                {item.icon && <span className="w-4 h-4 shrink-0">{item.icon}</span>}
                {item.label}
              </>
            );

            if (item.href) {
              return (
                <Link key={i} to={item.href} className={base} onClick={() => setOpen(false)}>
                  {content}
                </Link>
              );
            }
            return (
              <button
                key={i}
                type="button"
                className={base}
                onClick={() => { setOpen(false); item.onClick?.(); }}
              >
                {content}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
};

export default DotsDropdown;
