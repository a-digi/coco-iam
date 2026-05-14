import React from 'react';
import { useBreadcrumbItems } from '../../Layout/Breadcrumb/useBreadcrumb';
import ScopeBasedComponentAccess from '../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../config/security/scopes';
import { StatsSection } from './Sections/StatsSection';
import { RegistrationsSection } from './Sections/RegistrationsSection';
import { TopOrgsSection } from './Sections/TopOrgsSection';
import { QueueSection } from './Sections/QueueSection';
import { RecentUsersSection } from './Sections/RecentUsersSection';
import { FailedTasksSection } from './Sections/FailedTasksSection';
import { VerticalSlider } from '../../Shared/Components/VerticalSlider';
import { Tabs } from '../../Shared/Components/Tabs/Tabs';

const Dashboard: React.FC = () => {
  useBreadcrumbItems([{ label: 'Dashboard' }]);

  return (
    <div className="space-y-6 p-6">
      <ScopeBasedComponentAccess
        requiredScopes={[AppScopes.AdminDashboardStatsRead, AppScopes.SuperAdmin]}
      >
        <StatsSection />
      </ScopeBasedComponentAccess>

      {/* Mobile: vertical slider for charts */}
      <div className="md:hidden">
        <VerticalSlider
          slides={[
            <ScopeBasedComponentAccess
              key="registrations"
              requiredScopes={[AppScopes.AdminDashboardRegistrationsRead, AppScopes.SuperAdmin]}
            >
              <RegistrationsSection />
            </ScopeBasedComponentAccess>,
            <ScopeBasedComponentAccess
              key="queue"
              requiredScopes={[AppScopes.AdminDashboardQueueRead, AppScopes.SuperAdmin]}
            >
              <QueueSection />
            </ScopeBasedComponentAccess>,
          ]}
        />
      </div>

      {/* Desktop: two-column layout for charts */}
      <div className="hidden md:flex md:items-stretch md:gap-6 md:h-[460px]">
        <div className="md:flex-1 md:min-w-0 md:flex md:flex-col">
          <ScopeBasedComponentAccess
            requiredScopes={[AppScopes.AdminDashboardRegistrationsRead, AppScopes.SuperAdmin]}
          >
            <RegistrationsSection />
          </ScopeBasedComponentAccess>
        </div>
        <div className="md:w-[30%] md:shrink-0 md:flex md:flex-col">
          <ScopeBasedComponentAccess
            requiredScopes={[AppScopes.AdminDashboardQueueRead, AppScopes.SuperAdmin]}
          >
            <QueueSection />
          </ScopeBasedComponentAccess>
        </div>
      </div>

      {/* Mobile: tabs for table sections */}
      <div className="md:hidden">
        <Tabs
          variant="pills"
          items={[
            {
              id: 'top-orgs',
              title: 'Top Organizations by Users',
              content: (
                <ScopeBasedComponentAccess
                  requiredScopes={[AppScopes.AdminDashboardTopOrgsRead, AppScopes.SuperAdmin]}
                >
                  <TopOrgsSection />
                </ScopeBasedComponentAccess>
              ),
            },
            {
              id: 'recent-users',
              title: 'Recent Users',
              content: (
                <ScopeBasedComponentAccess
                  requiredScopes={[AppScopes.AdminDashboardRecentUsersRead, AppScopes.SuperAdmin]}
                >
                  <RecentUsersSection />
                </ScopeBasedComponentAccess>
              ),
            },
            {
              id: 'failed-tasks',
              title: 'Recent Failed Tasks',
              content: (
                <ScopeBasedComponentAccess
                  requiredScopes={[AppScopes.AdminDashboardFailedTasksRead, AppScopes.SuperAdmin]}
                >
                  <FailedTasksSection />
                </ScopeBasedComponentAccess>
              ),
            },
          ]}
        />
      </div>

      {/* Desktop: three-column grid */}
      <div className="hidden md:grid md:grid-cols-3 md:gap-6 md:items-start">
        <div className="min-w-0">
          <ScopeBasedComponentAccess
            requiredScopes={[AppScopes.AdminDashboardTopOrgsRead, AppScopes.SuperAdmin]}
          >
            <TopOrgsSection />
          </ScopeBasedComponentAccess>
        </div>
        <div className="min-w-0">
          <ScopeBasedComponentAccess
            requiredScopes={[AppScopes.AdminDashboardRecentUsersRead, AppScopes.SuperAdmin]}
          >
            <RecentUsersSection />
          </ScopeBasedComponentAccess>
        </div>
        <div className="min-w-0">
          <ScopeBasedComponentAccess
            requiredScopes={[AppScopes.AdminDashboardFailedTasksRead, AppScopes.SuperAdmin]}
          >
            <FailedTasksSection />
          </ScopeBasedComponentAccess>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
