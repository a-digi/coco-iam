// TypeScript mirror of api/src/userrules/model.go. Keep field names in
// sync — they cross the wire as JSON.

export interface PasswordRules {
    min_length: number;
    max_length: number;
    require_upper: boolean;
    require_lower: boolean;
    require_digit: boolean;
    require_special: boolean;
    disallow_username: boolean;
    disallow_email: boolean;
    expiry_days: number;
    notify_days: number[];
}

export interface UsernameRules {
    min_length: number;
    max_length: number;
    regex: string;
    reserved: string[];
}

export interface EmailRules {
    allowed_domains: string[];
    blocked_domains: string[];
}

export interface RuleSet {
    password: PasswordRules;
    username: UsernameRules;
    email: EmailRules;
}

export const EMPTY_RULE_SET: RuleSet = {
    password: {
        min_length: 8,
        max_length: 128,
        require_upper: false,
        require_lower: false,
        require_digit: false,
        require_special: false,
        disallow_username: true,
        disallow_email: true,
        expiry_days: 0,
        notify_days: [],
    },
    username: {
        min_length: 3,
        max_length: 64,
        regex: '^[a-zA-Z0-9_.\\-]+$',
        reserved: ['root', 'admin', 'system'],
    },
    email: {
        allowed_domains: [],
        blocked_domains: [],
    },
};
