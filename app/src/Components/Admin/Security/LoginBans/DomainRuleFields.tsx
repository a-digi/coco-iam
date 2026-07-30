import React from 'react';
import { FormInput } from '../../../../Shared/Components/Form/Input/FormInput';

interface DomainRuleFieldsProps {
    title: string;
    idPrefix: string;
    enabled: boolean;
    threshold: string;
    windowSeconds: string;
    banSeconds: string;
    onEnabledChange: (value: boolean) => void;
    onThresholdChange: (value: string) => void;
    onWindowSecondsChange: (value: string) => void;
    onBanSecondsChange: (value: string) => void;
}

// DomainRuleFields renders one domain's (admin or application) failed-login
// ban rule — an enabled toggle plus the three numbers that define it. Purely
// props-in/callbacks-out; LoginBanRulesSettings owns all state and the save
// call, since the two domains are always read/written together as one
// object. See plan/login-ban-rules/plan.md.
export const DomainRuleFields: React.FC<DomainRuleFieldsProps> = ({
    title,
    idPrefix,
    enabled,
    threshold,
    windowSeconds,
    banSeconds,
    onEnabledChange,
    onThresholdChange,
    onWindowSecondsChange,
    onBanSecondsChange,
}) => {
    return (
        <div className="space-y-4">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100">{title}</h3>

            <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                <input
                    type="checkbox"
                    className="accent-indigo-600"
                    checked={enabled}
                    onChange={e => onEnabledChange(e.target.checked)}
                />
                Enabled
            </label>

            <FormInput
                id={`${idPrefix}-threshold`}
                label="Failed attempts"
                type="number"
                value={threshold}
                onChange={onThresholdChange}
                disabled={!enabled}
                min={1}
                description="Number of failed logins from one IP that triggers a ban."
            />

            <FormInput
                id={`${idPrefix}-window-seconds`}
                label="Time window (seconds)"
                type="number"
                value={windowSeconds}
                onChange={onWindowSecondsChange}
                disabled={!enabled}
                min={1}
                description="Failed attempts are only counted within this many seconds of each other."
            />

            <FormInput
                id={`${idPrefix}-ban-seconds`}
                label="Ban duration (seconds)"
                type="number"
                value={banSeconds}
                onChange={onBanSecondsChange}
                disabled={!enabled}
                min={1}
                description="How long the IP stays banned once triggered."
            />
        </div>
    );
};

export default DomainRuleFields;
