import React from 'react';
import {useSidebar} from '../../../Layout/Sidebar/SidebarContextContext.ts';

const MenuToggleButton: React.FC = () => {
  const { open, toggle } = useSidebar();
  return (
    <button
      className="w-5 h-5 flex items-center justify-center bg-transparent text-gray-700 dark:text-gray-200 focus:outline-none transition-transform duration-300"
      onClick={toggle}
      aria-label={open ? 'Close sidebar' : 'Open sidebar'}
    >
      {/* Always show the hamburger icon, just smaller */}
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        className="w-4 h-4"
      >
        <rect y="3" width="16" height="1.5" rx="0.75" className="fill-current" />
        <rect y="7" width="16" height="1.5" rx="0.75" className="fill-current" />
        <rect y="11" width="16" height="1.5" rx="0.75" className="fill-current" />
      </svg>
    </button>
  );
};

export default MenuToggleButton;
