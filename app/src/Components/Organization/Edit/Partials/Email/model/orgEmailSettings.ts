import type { EmailAccount } from '../../../../../Admin/Settings/EmailAccounts/model/emailAccount';
import type { EventBinding } from '../../../../../Admin/Settings/Email/model/emailSettings';

// OrgActivationSettings mirrors entity.OrgActivationSettings — null
// means "not customized here, falls back to the global default";
// the backend has no explicit "clear override" mechanism (matches
// the global admin settings page's own limitation), so once set a
// field can only be overwritten with a new value, never unset.
export interface OrgActivationSettings {
    ttl_hours: number | null;
    resend_cooldown_seconds: number | null;
}

export interface OrgMailSettings {
    active_account: EmailAccount | null;
    events: EventBinding[];
    activation: OrgActivationSettings;
}

export interface OrgActivationPatch {
    ttl_hours?: number;
    resend_cooldown_seconds?: number;
}

export interface OrgMailSettingsPatch {
    events?: EventBinding[];
    activation?: OrgActivationPatch;
}

export const EMPTY_ORG_ACTIVATION: OrgActivationSettings = {
    ttl_hours: null,
    resend_cooldown_seconds: null,
};
