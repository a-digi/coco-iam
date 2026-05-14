import React, { useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { PageHead, PageHeadBack } from '../../../Shared/Components/PageHead';
import { OrganizationResource, type Organization, OrganizationSchema } from '../model/organization';
import { mapObjects } from '../../../config/data/mapper/mapper';

interface OrganizationPageHeadProps {
    /** UUID of the organization. Used as the single source of truth for
     *  data fetching unless a pre-resolved `organization` is supplied. */
    organizationId: string;
    /** Optional: pass a pre-fetched organization (e.g. from a parent
     *  that already needed the data for a form). Skips the internal
     *  fetch when provided. */
    organization?: Organization | null;
    /** Back-link target. Defaults to the organizations index. Pass
     *  `null` to suppress the back link entirely. */
    backTo?: string | null;
    backLabel?: string;
    /** Optional right-aligned slot rendered inside the PageHead. */
    actions?: React.ReactNode;
}

/**
 * OrganizationPageHead is the shared header shown on every page that
 * scopes to a single organization — edit page, nested dashboards
 * (users, groups, workspaces, …). Uses the shared <PageHead> for
 * styling so all organization-scoped pages have a consistent look and
 * feel. When `organization` is supplied by the parent the internal
 * fetch is skipped to avoid a redundant round-trip.
 */
export const OrganizationPageHead: React.FC<OrganizationPageHeadProps> = ({
    organizationId,
    organization: organizationProp,
    backTo = '/organizations',
    backLabel = 'Back to organizations',
    actions,
}) => {
    const [organization, setOrganization] = useState<Organization | null>(organizationProp ?? null);
    const [loading, setLoading] = useState<boolean>(organizationProp === undefined);

    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    useEffect(() => {
        // Parent supplied the org — mirror it into local state and skip the fetch.
        if (organizationProp !== undefined) {
            setOrganization(organizationProp);
            setLoading(false);
            return;
        }
        if (!organizationId) return;
        let cancelled = false;
        setLoading(true);
        (async () => {
            try {
                const response = await get<{ message: unknown }>(
                    `organizations/{${OrganizationResource}}/{id:${organizationId}}`,
                );
                const raw = response?.message || response;
                if (raw && !cancelled) {
                    const mapped = mapObjects(OrganizationSchema, [raw]) as unknown as Organization[];
                    setOrganization(mapped[0] ?? null);
                }
            } catch (err: unknown) {
                let errorMsg = 'Failed to load organization';
                if (err instanceof Error) errorMsg = err.message || errorMsg;
                errorMessage(errorMsg);
            } finally {
                if (!cancelled) setLoading(false);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [organizationId, organizationProp, get, errorMessage]);

    if (loading || !organization) return null;

    return (
        <>
            {backTo && <PageHeadBack to={backTo} label={backLabel} />}
            <PageHead
                kicker="Organization"
                title={organization.title}
                description={organization.description}
                actions={actions}
            />
        </>
    );
};

export default OrganizationPageHead;
