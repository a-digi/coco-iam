// TS mirror of api/src/applications/loginpage/model.go.
// Custom HTML templates were removed — admins pick one of three fixed
// layouts and edit typed settings.

import { publicAssetURL } from '../../../api/client';

export type TemplateKind = 'centered_1col' | 'split_login_left' | 'split_login_right';

// Per-column override, pre-resolved by the backend. Background
// fields empty → inherit wrapper. Side-panel text is a single title
// plus an ordered list of HTML content strings.
export interface PublicColumnConfig {
  column_index: number;
  background_color?: string;
  background_url?: string;
  background_gradient?: string;
  text_color?: string;
  text_block_title?: string;
  text_contents?: string[];
}

export interface PublicLoginConfig {
  workspace_id: string;
  application_id: string;
  workspace_name: string;
  application_name: string;
  configured: boolean;

  template_kind: TemplateKind;
  background_color: string;
  background_url?: string;
  // Pre-composed CSS linear-gradient — empty when no gradient set.
  background_gradient?: string;
  logo_url?: string;
  show_logo: boolean;
  page_title: string;
  brand_text: string;
  allow_recovery: boolean;
  allow_registration: boolean;

  // Per-column overrides for split templates. Indices are 0 = left,
  // 1 = right. Empty / missing = inherit the wrapper background for
  // visual styling; text fields are column-scoped only.
  columns?: PublicColumnConfig[];
}

// Public URL of an uploaded asset. Used by the admin editor preview.
export const assetUrl = publicAssetURL;
