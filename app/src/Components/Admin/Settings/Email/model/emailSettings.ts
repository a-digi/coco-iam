import type { EmailAccount } from '../../EmailAccounts/model/emailAccount';

export interface EventBinding {
    event: string;
    template: string;
    account: string;
}

export interface ActivationSettings {
    ttl_hours: number;
    resend_cooldown_seconds: number;
}

export interface EmailSettings {
    active_account: EmailAccount | null;
    events: EventBinding[];
    activation: ActivationSettings;
}

export interface ActivationPatch {
    ttl_hours?: number;
    resend_cooldown_seconds?: number;
}

export interface EmailSettingsPatch {
    events?: EventBinding[];
    activation?: ActivationPatch;
}

export interface MailEvent {
    key: string;
    label: string;
    description: string;
}

export const EMPTY_ACTIVATION: ActivationSettings = {
    ttl_hours: 24,
    resend_cooldown_seconds: 300,
};
