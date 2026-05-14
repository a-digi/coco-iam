import React, { useState, type SyntheticEvent } from 'react';
import { Submit, Cancel } from '../../Shared/Components/Button';
import { FormInput } from '../../Shared/Components/Form';
import { Switch } from '../../Shared/Components/Switch';
import ScopeBasedComponentAccess from '../../Shared/Components/Access/ScopeBasedComponentAccess';
import type { RuleSet } from './model/userRules';

const PRESET_NOTIFY_DAYS = [1, 7, 14, 30];

interface NotifyDaysPickerProps {
    days: number[];
    onChange: (days: number[]) => void;
}

const NotifyDaysPicker: React.FC<NotifyDaysPickerProps> = ({ days, onChange }) => {
    const [customInput, setCustomInput] = useState('');

    const toggle = (d: number) => {
        const next = days.includes(d) ? days.filter(x => x !== d) : [...days, d];
        onChange(next.sort((a, b) => a - b));
    };

    const addCustom = () => {
        const v = parseInt(customInput, 10);
        if (!isNaN(v) && v > 0 && !days.includes(v)) {
            onChange([...days, v].sort((a, b) => a - b));
        }
        setCustomInput('');
    };

    const remove = (d: number) => onChange(days.filter(x => x !== d));
    const customDays = days.filter(d => !PRESET_NOTIFY_DAYS.includes(d));

    return (
        <div className="space-y-3">
            <div className="flex flex-wrap gap-4">
                {PRESET_NOTIFY_DAYS.map(d => (
                    <label key={d} className="flex items-center gap-2 cursor-pointer select-none">
                        <input
                            type="checkbox"
                            checked={days.includes(d)}
                            onChange={() => toggle(d)}
                            className="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                        />
                        <span className="text-sm text-gray-700 dark:text-gray-300">
                            {d} {d === 1 ? 'day' : 'days'} before
                        </span>
                    </label>
                ))}
            </div>
            {customDays.length > 0 && (
                <div className="flex flex-wrap gap-2">
                    {customDays.map(d => (
                        <span key={d} className="inline-flex items-center gap-1 px-2 py-0.5 text-xs rounded-full bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-800">
                            {d} days before
                            <button type="button" onClick={() => remove(d)} className="ml-1 hover:text-red-500">×</button>
                        </span>
                    ))}
                </div>
            )}
            <div className="flex items-center gap-2">
                <input
                    type="number"
                    min={1}
                    value={customInput}
                    onChange={e => setCustomInput(e.target.value)}
                    onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addCustom(); } }}
                    placeholder="Custom days…"
                    className="w-32 px-3 py-1.5 text-sm rounded-md border border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <button
                    type="button"
                    onClick={addCustom}
                    className="px-3 py-1.5 text-sm rounded-md border border-gray-300 dark:border-surface-600 hover:bg-gray-50 dark:hover:bg-surface-700 text-gray-700 dark:text-gray-300"
                >
                    Add
                </button>
            </div>
        </div>
    );
};

export interface UserRulesFormProps {
    initial: RuleSet;
    onSave: (next: RuleSet) => Promise<void>;
    loading?: boolean;
    /** Scopes gating the Save button. Passed through to
     *  ScopeBasedComponentAccess — must include at least one. */
    writeScopes: string[];
    /** Where the Cancel link navigates. Optional. */
    cancelTo?: string;
}

type ListField = 'reserved' | 'allowed_domains' | 'blocked_domains';

// Split a comma- or newline-separated string into a clean list.
const parseList = (raw: string): string[] =>
    raw
        .split(/[,\n]/)
        .map(s => s.trim())
        .filter(s => s.length > 0);

const joinList = (list: string[]): string => list.join(', ');

export const UserRulesForm: React.FC<UserRulesFormProps> = ({
    initial,
    onSave,
    loading = false,
    writeScopes,
    cancelTo,
}) => {
    const [form, setForm] = useState<RuleSet>(initial);
    // Raw strings for list-style inputs so typing commas feels natural.
    const [listRaw, setListRaw] = useState<Record<ListField, string>>({
        reserved: joinList(initial.username.reserved),
        allowed_domains: joinList(initial.email.allowed_domains),
        blocked_domains: joinList(initial.email.blocked_domains),
    });

    const patchPassword = (partial: Partial<RuleSet['password']>) =>
        setForm(prev => ({ ...prev, password: { ...prev.password, ...partial } }));
    const patchUsername = (partial: Partial<RuleSet['username']>) =>
        setForm(prev => ({ ...prev, username: { ...prev.username, ...partial } }));

    const handleSubmit = async (e: SyntheticEvent) => {
        e.preventDefault();
        // Finalise list fields from their raw-text counterparts before saving.
        const next: RuleSet = {
            ...form,
            username: { ...form.username, reserved: parseList(listRaw.reserved) },
            email: {
                allowed_domains: parseList(listRaw.allowed_domains),
                blocked_domains: parseList(listRaw.blocked_domains),
            },
        };
        await onSave(next);
    };

    return (
        <form onSubmit={handleSubmit} className="space-y-8">
            {/* Password section */}
            <section className="space-y-4">
                <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Password</h3>
                    <p className="text-sm text-gray-500">Shape requirements for every new password.</p>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <FormInput
                        id="pw_min_length"
                        type="number"
                        label="Minimum length"
                        value={String(form.password.min_length)}
                        onChange={v => patchPassword({ min_length: Math.max(0, Number(v) || 0) })}
                        min={0}
                    />
                    <FormInput
                        id="pw_max_length"
                        type="number"
                        label="Maximum length (0 = no limit)"
                        value={String(form.password.max_length)}
                        onChange={v => patchPassword({ max_length: Math.max(0, Number(v) || 0) })}
                        min={0}
                    />
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-700">
                    <Switch id="pw_require_upper"   checked={form.password.require_upper}   onChange={v => patchPassword({ require_upper: v })}   label="Require uppercase" />
                    <Switch id="pw_require_lower"   checked={form.password.require_lower}   onChange={v => patchPassword({ require_lower: v })}   label="Require lowercase" />
                    <Switch id="pw_require_digit"   checked={form.password.require_digit}   onChange={v => patchPassword({ require_digit: v })}   label="Require digit" />
                    <Switch id="pw_require_special" checked={form.password.require_special} onChange={v => patchPassword({ require_special: v })} label="Require special char" />
                    <Switch id="pw_disallow_username" checked={form.password.disallow_username} onChange={v => patchPassword({ disallow_username: v })} label="Disallow username in password" />
                    <Switch id="pw_disallow_email"    checked={form.password.disallow_email}    onChange={v => patchPassword({ disallow_email: v })}    label="Disallow email in password" />
                </div>
            </section>

            {/* Password expiry section */}
            <section className="space-y-4">
                <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Password expiry</h3>
                    <p className="text-sm text-gray-500">Force users to change their password after a set period.</p>
                </div>
                <div className="p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-700 space-y-4">
                    <Switch
                        id="pw_expiry_enabled"
                        checked={form.password.expiry_days > 0}
                        onChange={v => patchPassword({ expiry_days: v ? 90 : 0 })}
                        label="Enable password expiry"
                    />
                    {form.password.expiry_days > 0 && (
                        <FormInput
                            id="pw_expiry_days"
                            type="number"
                            label="Expire after (days)"
                            value={String(form.password.expiry_days)}
                            onChange={v => patchPassword({ expiry_days: Math.max(1, Number(v) || 1) })}
                            min={1}
                        />
                    )}
                </div>
            </section>

            {/* Expiry email notification section */}
            <section className="space-y-4">
                <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Expiry email notifications</h3>
                    <p className="text-sm text-gray-500">Send users an email reminder before their password expires. Only active when password expiry is enabled.</p>
                </div>
                <div className="p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-700 space-y-4">
                    <Switch
                        id="pw_notify_enabled"
                        checked={(form.password.notify_days ?? []).length > 0}
                        onChange={v => patchPassword({ notify_days: v ? [1, 7, 14] : [] })}
                        label="Enable expiry notifications"
                    />
                    {(form.password.notify_days ?? []).length > 0 && (
                        <NotifyDaysPicker
                            days={form.password.notify_days ?? []}
                            onChange={days => patchPassword({ notify_days: days })}
                        />
                    )}
                </div>
            </section>

            {/* Username section */}
            <section className="space-y-4">
                <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Username</h3>
                    <p className="text-sm text-gray-500">Accepted format for new usernames.</p>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <FormInput
                        id="un_min_length"
                        type="number"
                        label="Minimum length"
                        value={String(form.username.min_length)}
                        onChange={v => patchUsername({ min_length: Math.max(0, Number(v) || 0) })}
                        min={0}
                    />
                    <FormInput
                        id="un_max_length"
                        type="number"
                        label="Maximum length"
                        value={String(form.username.max_length)}
                        onChange={v => patchUsername({ max_length: Math.max(0, Number(v) || 0) })}
                        min={0}
                    />
                </div>
                <FormInput
                    id="un_regex"
                    label="Regex (Go / ECMAScript compatible)"
                    value={form.username.regex}
                    onChange={v => patchUsername({ regex: v })}
                    description={`Default: ^[a-zA-Z0-9_.\\-]+$`}
                />
                <FormInput
                    id="un_reserved"
                    label="Reserved usernames (comma-separated)"
                    value={listRaw.reserved}
                    onChange={v => setListRaw(prev => ({ ...prev, reserved: v }))}
                    description="Users are not allowed to pick these names."
                />
            </section>

            {/* Email section */}
            <section className="space-y-4">
                <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Email</h3>
                    <p className="text-sm text-gray-500">Domain allow / block lists. Blocks always win over allows.</p>
                </div>
                <FormInput
                    id="em_allowed"
                    label="Allowed domains (comma-separated; empty = any)"
                    value={listRaw.allowed_domains}
                    onChange={v => setListRaw(prev => ({ ...prev, allowed_domains: v }))}
                    placeholder="example.com, acme.co"
                />
                <FormInput
                    id="em_blocked"
                    label="Blocked domains (comma-separated)"
                    value={listRaw.blocked_domains}
                    onChange={v => setListRaw(prev => ({ ...prev, blocked_domains: v }))}
                    placeholder="mailinator.com"
                />
            </section>

            <div className="flex items-center gap-4 pt-2">
                <ScopeBasedComponentAccess requiredScopes={writeScopes}>
                    <Submit loading={loading} label="Save rules" />
                </ScopeBasedComponentAccess>
                {cancelTo && <Cancel to={cancelTo} />}
            </div>
        </form>
    );
};

export default UserRulesForm;
