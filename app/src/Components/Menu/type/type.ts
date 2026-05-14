export type MenuItem = {
  name: string;
  href?: string;
  icon?: string;
  accessScopes?: string[];
  children?: MenuItem[];
  // When true, the entry is hidden from the sidebar until at least
  // one organization exists. Used for entries whose core workflow
  // has an organization as a prerequisite (e.g. Workspaces must
  // belong to an organization).
  hideWhenNoOrganizations?: boolean;
};
