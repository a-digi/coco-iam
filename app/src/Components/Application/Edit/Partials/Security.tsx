import React, { useMemo } from 'react';
import { Tabs, type TabData } from '../../../../Shared/Components/Tabs';
import { Keys } from './Keys';

interface Props {
    applicationId: string;
}

/**
 * Security groups application-level security features in a tab row.
 * Signing keys is the only entry today; IP allowlists, session
 * policies, or OAuth client secrets would slot in next without
 * touching the shell. API credentials live under their own top-level
 * side-menu entry because they can carry purposes beyond security.
 */
export const Security: React.FC<Props> = ({ applicationId }) => {
    const items = useMemo<TabData[]>(
        () => [
            {
                id: 'signing-keys',
                title: 'Signing keys',
                content: <Keys applicationId={applicationId} />,
            },
        ],
        [applicationId],
    );

    return (
        <div className="mt-2">
            <Tabs items={items} initialActiveId="signing-keys" />
        </div>
    );
};

export default Security;
