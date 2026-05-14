import React, { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { AppScopes } from '../../../../config/security/scopes';
import { UserRulesForm } from '../../../UserRules/UserRulesForm';
import {
    EMPTY_RULE_SET,
    type RuleSet,
} from '../../../UserRules/model/userRules';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export const UserRulesManager: React.FC = () => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Settings' }, { label: 'User Rules' }]);
    const { get, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [rules, setRules] = useState<RuleSet>(EMPTY_RULE_SET);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);

    const fetchRules = useCallback(async () => {
        setLoading(true);
        try {
            const response = await get<{ message?: RuleSet }>('admin/settings/user-rules');
            if (response?.message) setRules(response.message);
        } catch (err: unknown) {
            let msg = 'Failed to load user rules';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void fetchRules();
    }, [fetchRules]);

    const handleSave = useCallback(async (next: RuleSet) => {
        setSaving(true);
        try {
            const response = await patch<{ message?: RuleSet }>('admin/settings/user-rules', next);
            if (response?.message) setRules(response.message);
            successMessage('User rules saved.');
        } catch (err: unknown) {
            let msg = 'Failed to save user rules';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSaving(false);
        }
    }, [patch, successMessage, errorMessage]);

    if (loading) {
        return <div className="text-sm text-gray-500 py-2">Loading user rules…</div>;
    }

    return (
        <div className="space-y-6">
            <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Admin user rules</h3>
                <p className="text-sm text-gray-500">
                    Validation rules applied to every admin user — on creation (username + email)
                    and on every password change / activation.
                </p>
            </div>

            <UserRulesForm
                initial={rules}
                onSave={handleSave}
                loading={saving}
                writeScopes={[AppScopes.AdminSettingsUserRulesWrite, AppScopes.AdminSettingsUserRules, AppScopes.SuperAdmin]}
            />
        </div>
    );
};

export default UserRulesManager;
