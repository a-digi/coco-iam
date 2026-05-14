export interface DashboardStats {
  total_admin_users: number;
  active_admin_users: number;
  total_org_users: number;
  active_org_users: number;
  org_users_with_app_access: number;
  total_organizations: number;
  total_workspaces: number;
  total_applications: number;
  total_groups: number;
  queue_pending: number;
  queue_failed: number;
}

export interface RegistrationPoint {
  label: string;
  count: number;
}

export interface OrgRegistrations {
  by_weekday: RegistrationPoint[];
  by_month: RegistrationPoint[];
  by_year: RegistrationPoint[];
}

export interface OrgUserCount {
  id: string;
  name: string;
  count: number;
}

export interface QueueStatusCount {
  status: string;
  count: number;
}

export interface QueueBreakdown {
  name: string;
  success: number;
  pending: number;
  failed: number;
  total: number;
}

export interface QueueResponse {
  by_status: QueueStatusCount[];
  top_queues: QueueBreakdown[];
}

export interface RecentUser {
  id: string;
  username: string;
  created_at: string;
}

export interface RecentTask {
  id: string;
  last_error: string;
  queue_name: string;
  attempts: number;
  created_at: string;
}
