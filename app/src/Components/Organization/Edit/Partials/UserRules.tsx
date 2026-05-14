import React, { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { AppScopes } from '../../../../config/security/scopes';
import { UserRulesForm } from '../../../UserRules/UserRulesForm';
import {
    EMPTY_RULE_SET,
    type RuleSet,
} from '../../../UserRules/model/userRules';
import { OrganizationResource } from '../../model/organization';

interface Props {
    organizationId: string;
}

/**
 * UserRules partial — loads the org's rule set and shows the shared
 * editor. Lives in the Org Edit tabs next to Details / Workspaces.
 */
export const UserRules: React.FC<Props> = ({ organizationId }) => {
    const { get, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [rules, setRules] = useState<RuleSet>(EMPTY_RULE_SET);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);

    const fetchRules = useCallback(async () => {
        setLoading(true);
        try {
            const response = await get<{ message?: RuleSet }>(
                `organizations/{${OrganizationResource}}/{id:${organizationId}}/user-rules`,
            );
            if (response?.message) setRules(response.message);
        } catch (err: unknown) {
            let msg = 'Failed to load organization user rules';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage, organizationId]);

    useEffect(() => {
        void fetchRules();
    }, [fetchRules]);

    const handleSave = useCallback(async (next: RuleSet) => {
        setSaving(true);
        try {
            const response = await patch<{ message?: RuleSet }>(
                `organizations/{${OrganizationResource}}/{id:${organizationId}}/user-rules`,
                next,
            );
            if (response?.message) setRules(response.message);
            successMessage('User rules saved.');
        } catch (err: unknown) {
            let msg = 'Failed to save organization user rules';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSaving(false);
        }
    }, [patch, successMessage, errorMessage, organizationId]);

    if (loading) {
        return <div className="text-sm text-gray-500 py-2">Loading user rules…</div>;
    }

    return (
        <div className="space-y-6 mt-4">
            <div>
                <p className="text-sm text-gray-500">
                    Rules applied to users of this organization — on creation
                    (username + email) and on every password change / activation.
                </p>
            </div>

            <UserRulesForm
                initial={rules}
                onSave={handleSave}
                loading={saving}
                writeScopes={[AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin]}
            />
        </div>
    );
};

export default UserRules;
