import { useState, type ReactNode } from 'react';
import { ThemeProvider } from './ThemeContext';
import { SidebarProvider } from './Sidebar/SidebarContext';
import { BreadcrumbProvider } from './Breadcrumb/BreadcrumbProvider';
import { BreadcrumbBar } from './Breadcrumb/BreadcrumbBar';
import Sidebar from './Sidebar/Sidebar';
import TopBar from './TopBar/TopBar';
import Content from './Content/Content';
import MenuToggleButton from '../Components/Menu/Sidebar/MenuToggleButton';
import { useAuth } from '../Components/Auth/Guard/useAuth';
import { LayoutContext } from './LayoutContextContext';
import { useSidebar } from './Sidebar/SidebarContextContext.ts';
import { useTheme } from './ThemeContextContext.ts';
import UserMenu from '../Shared/Components/User/Menu/UserMenu';

export const LayoutProvider = ({ children }: { children: ReactNode }) => {
  const [sidebarContent, setSidebarContent] = useState<ReactNode>(null);
  const [topbarContent, setTopbarContent] = useState<ReactNode>(null);
  const [mainContent, setMainContent] = useState<ReactNode>(null);

  return (
    <ThemeProvider>
      <SidebarProvider>
        <BreadcrumbProvider>
          <LayoutContext.Provider value={{ setSidebarContent, setTopbarContent, setMainContent }}>
            <LayoutContent mainContent={mainContent} topbarContent={topbarContent} sidebarContent={sidebarContent}>
              {children}
            </LayoutContent>
          </LayoutContext.Provider>
        </BreadcrumbProvider>
      </SidebarProvider>
    </ThemeProvider>
  );
};

function LayoutContent({ mainContent, topbarContent, sidebarContent, children }: {
  mainContent: React.ReactNode;
  topbarContent: React.ReactNode;
  sidebarContent: React.ReactNode;
  children: React.ReactNode;
}) {
  const { open, setOpen } = useSidebar();
  const { theme } = useTheme();
  const { authenticated } = useAuth();

  if (!authenticated) {
    return <Content>{mainContent || children}</Content>;
  }

  return (
    <div className={`${theme === 'dark' ? 'dark' : ''} flex flex-col h-screen w-screen m-0 p-0 bg-white text-black dark:bg-surface-900 dark:text-white overflow-hidden`}>
      <TopBar leftItems={<><MenuToggleButton />{topbarContent}</>} rightItems={<><TopbarThemeSwitcher /><UserMenu /></>} />
      <div className="flex flex-1 m-0 p-0 relative mt-16 h-[calc(100vh-64px)] overflow-hidden">
        {/* Mobile Sidebar Overlay */}
        {open && (
          <div
            className="fixed inset-0 z-30 bg-black/50 md:hidden"
            onClick={() => setOpen(false)}
            aria-hidden="true"
          />
        )}
        <Sidebar>{sidebarContent}</Sidebar>
        <div
          className={`flex-1 h-full overflow-y-auto transition-all duration-300 w-full ${open ? 'md:ml-[300px]' : 'md:ml-16'}`}
        >
          <BreadcrumbBar />
          <Content>{mainContent || children}</Content>
        </div>
      </div>
    </div>
  );
}

function TopbarThemeSwitcher() {
  const { theme, toggleTheme } = useTheme();
  return (
    <div
      onClick={toggleTheme}
      aria-label="Toggle dark mode"
      className="mr-4 px-3 py-1 cursor-pointer text-gray-900 dark:text-gray-100 transition-colors"
    >
      {theme === 'dark' ? <svg xmlns="http://www.w3.org/2000/svg" xmlnsXlink="http://www.w3.org/1999/xlink" aria-hidden="true" role="img" className="iconify iconify--tabler" width="1em" height="1em" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 3h.393a7.5 7.5 0 0 0 7.92 12.446A9 9 0 1 1 12 2.992z"></path></svg> : <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" role="img" className="iconify iconify--tabler" width="1em" height="1em" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 12m-4 0a4 4 0 1 0 8 0a4 4 0 1 0 -8 0M3 12h1m16 0h1M12 3v1m0 16v1M5.6 5.6l.7 .7m11.4 11.4l.7 .7M5.6 18.4l.7 -.7m11.4 -11.4l.7 -.7"></path></svg>}
    </div>
  );
}
