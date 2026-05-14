import React, { useState } from 'react';

export type AlertVariant = 'info' | 'success' | 'warning' | 'error' | 'tip';

export interface AlertProps {
  variant: AlertVariant;
  title?: string;
  children: React.ReactNode;
  dismissible?: boolean;
  className?: string;
}

const STYLES: Record<AlertVariant, {
  wrapper: string;
  icon: string;
  title: string;
  body: string;
  dismiss: string;
  svg: React.ReactNode;
}> = {
  info: {
    wrapper: 'bg-blue-50 border-blue-200 dark:bg-blue-950/40 dark:border-blue-800',
    icon:    'text-blue-500 dark:text-blue-400',
    title:   'text-blue-800 dark:text-blue-200',
    body:    'text-blue-700 dark:text-blue-300',
    dismiss: 'text-blue-400 hover:text-blue-600 dark:text-blue-500 dark:hover:text-blue-300',
    svg: (
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
    ),
  },
  success: {
    wrapper: 'bg-emerald-50 border-emerald-200 dark:bg-emerald-950/40 dark:border-emerald-800',
    icon:    'text-emerald-500 dark:text-emerald-400',
    title:   'text-emerald-800 dark:text-emerald-200',
    body:    'text-emerald-700 dark:text-emerald-300',
    dismiss: 'text-emerald-400 hover:text-emerald-600 dark:text-emerald-500 dark:hover:text-emerald-300',
    svg: (
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    ),
  },
  warning: {
    wrapper: 'bg-amber-50 border-amber-200 dark:bg-amber-950/40 dark:border-amber-800',
    icon:    'text-amber-500 dark:text-amber-400',
    title:   'text-amber-800 dark:text-amber-200',
    body:    'text-amber-700 dark:text-amber-300',
    dismiss: 'text-amber-400 hover:text-amber-600 dark:text-amber-500 dark:hover:text-amber-300',
    svg: (
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
    ),
  },
  error: {
    wrapper: 'bg-red-50 border-red-200 dark:bg-red-950/40 dark:border-red-800',
    icon:    'text-red-500 dark:text-red-400',
    title:   'text-red-800 dark:text-red-200',
    body:    'text-red-700 dark:text-red-300',
    dismiss: 'text-red-400 hover:text-red-600 dark:text-red-500 dark:hover:text-red-300',
    svg: (
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
    ),
  },
  tip: {
    wrapper: 'bg-violet-50 border-violet-200 dark:bg-violet-950/40 dark:border-violet-800',
    icon:    'text-violet-500 dark:text-violet-400',
    title:   'text-violet-800 dark:text-violet-200',
    body:    'text-violet-700 dark:text-violet-300',
    dismiss: 'text-violet-400 hover:text-violet-600 dark:text-violet-500 dark:hover:text-violet-300',
    svg: (
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M12 18v-5.25m0 0a6.01 6.01 0 001.5-.189m-1.5.189a6.01 6.01 0 01-1.5-.189m3.75 7.478a12.06 12.06 0 01-4.5 0m3.75 2.355a3.375 3.375 0 01-3 0m3.75-12.75a3.375 3.375 0 00-3.75 0m3.75 0v.938m-3.75-.938v.938m0 0a3.375 3.375 0 000 2.625m0-2.625a3.375 3.375 0 013.75 0" />
    ),
  },
};

export const Alert: React.FC<AlertProps> = ({ variant, title, children, dismissible = false, className = '' }) => {
  const [dismissed, setDismissed] = useState(false);
  if (dismissed) return null;

  const s = STYLES[variant];

  return (
    <div className={`flex gap-3 rounded-xl border px-4 py-3.5 ${s.wrapper} ${className}`} role="alert">
      <svg
        className={`w-5 h-5 shrink-0 mt-0.5 ${s.icon}`}
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={1.75}
        aria-hidden="true"
      >
        {s.svg}
      </svg>

      <div className="flex-1 min-w-0">
        {title && (
          <div className={`text-sm font-semibold mb-0.5 ${s.title}`}>{title}</div>
        )}
        <div className={`text-sm leading-relaxed ${s.body}`}>{children}</div>
      </div>

      {dismissible && (
        <button
          type="button"
          aria-label="Dismiss"
          onClick={() => setDismissed(true)}
          className={`shrink-0 mt-0.5 transition-colors ${s.dismiss}`}
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      )}
    </div>
  );
};

export default Alert;
