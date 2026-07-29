export interface DomainRuleSettings {
    enabled: boolean;
    threshold: number;
    window_seconds: number;
    ban_seconds: number;
}

export interface LoginBanRulesResponse {
    admin: DomainRuleSettings;
    application: DomainRuleSettings;
}
