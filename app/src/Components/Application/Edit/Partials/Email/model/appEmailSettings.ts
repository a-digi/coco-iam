import type { EmailAccount } from '../../../../../Admin/Settings/EmailAccounts/model/emailAccount';
import type { EventBinding } from '../../../../../Admin/Settings/Email/model/emailSettings';

// AppActivationSettings mirrors entity.AppActivationSettings — null
// means "not customized here, falls back to the organization's
// override, then the global default"; the backend has no explicit
// "clear override" mechanism (matches the org and global settings
// pages' own limitation), so once set a field can only be overwritten
// with a new value, never unset.
export interface AppActivationSettings {
    ttl_hours: number | null;
    resend_cooldown_seconds: number | null;
}

export interface AppMailSettings {
    active_account: EmailAccount | null;
    events: EventBinding[];
    activation: AppActivationSettings;
}

export interface AppActivationPatch {
    ttl_hours?: number;
    resend_cooldown_seconds?: number;
}

export interface AppMailSettingsPatch {
    events?: EventBinding[];
    activation?: AppActivationPatch;
}

export const EMPTY_APP_ACTIVATION: AppActivationSettings = {
    ttl_hours: null,
    resend_cooldown_seconds: null,
};
