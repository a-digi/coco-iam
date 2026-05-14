import React from 'react';

interface SidebarLogoProps {
  collapsed: boolean;
}

export const SidebarLogo: React.FC<SidebarLogoProps> = ({ collapsed }) => (
  <div className={`flex items-center mb-8 ${collapsed ? 'justify-center' : 'gap-3 px-3'}`}>
    <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-indigo-500 to-indigo-700 flex items-center justify-center shrink-0 shadow-sm">
      <svg className="w-7 h-7" fill="none" viewBox="0 0 24 24" stroke="white" strokeWidth={1.75}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 2l8 4v6c0 5-3.5 8.5-8 10-4.5-1.5-8-5-8-10V6l8-4z" />
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12a3 3 0 116 0v2H9v-2zM10 14v3m4-3v3" />
      </svg>
    </div>

    {!collapsed && (
      <div className="leading-none select-none">
        <div className="text-lg font-bold text-gray-900 dark:text-white tracking-tight">
          coco-iam
        </div>
        <div className="text-[0.625rem] font-semibold uppercase tracking-[0.18em] text-indigo-500 dark:text-indigo-400 mt-1">
          Identity &amp; Access
        </div>
      </div>
    )}
  </div>
);

export default SidebarLogo;
