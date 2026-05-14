import React from 'react';
import { useSidebar } from '../Sidebar/SidebarContextContext.ts';

interface TopBarProps {
  leftItems?: React.ReactNode;
  rightItems?: React.ReactNode;
  children?: React.ReactNode;
}

const TopBar: React.FC<TopBarProps> = ({ leftItems, rightItems }) => {
  const { open } = useSidebar();

  return (
    <div
      className={`h-16 bg-gray-50 dark:bg-surface-900 border-b border-gray-200 dark:border-surface-700 flex items-center px-1 m-0 fixed top-0 transition-all duration-300 z-30 ${open ? 'left-0 w-full md:left-[300px] md:w-[calc(100vw-300px)]' : 'left-0 w-full md:left-16 md:w-[calc(100vw-64px)]'
        }`}
    >
      <div className="flex flex-1 items-center justify-between w-full h-full">
        <div className="flex items-center gap-2 min-w-0">
          {leftItems}
        </div>
        <div className="flex items-center gap-2 min-w-0 justify-end">
          {rightItems}
        </div>
      </div>
    </div>
  );
};

export default TopBar;
