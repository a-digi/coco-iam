import React from 'react';
import SideBarMenu from '../../Components/Menu/Sidebar/Sidebar';
import { SidebarLogo } from '../../Components/Menu/Sidebar/SidebarLogo';
import { useSidebar } from './SidebarContextContext.ts';

interface SidebarProps {
  children?: React.ReactNode;
}

const Sidebar: React.FC<SidebarProps> = ({ children }) => {
  const { open } = useSidebar();

  return (
    <div className="relative h-full">
      <aside
        className={`
          absolute md:fixed top-0 left-0 h-full z-40
          bg-white dark:bg-surface-950
          border-r border-gray-200 dark:border-surface-700
          transition-all duration-300 ease-in-out overflow-hidden
          flex flex-col
          ${open
            ? 'translate-x-0 w-64 md:w-[300px] p-4'
            : '-translate-x-full p-4 md:translate-x-0 md:w-16 md:px-3 md:py-4'
          }
        `}
      >
        <SidebarLogo collapsed={!open} />
        <SideBarMenu />
        {children}

      </aside>
    </div>
  );
};

export default Sidebar;
