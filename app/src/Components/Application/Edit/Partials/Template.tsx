import React from 'react';
import { LoginSettings } from './LoginSettings';

interface Props {
    applicationId: string;
    workspaceId: string;
}

/**
 * Template owns the per-application login page configuration. Admins
 * pick one of three layouts and edit typed settings (background,
 * logo, titles, brand, text block, recovery/registration switches).
 * Custom HTML is no longer supported — see
 * plan/application-login-templates/plan.md.
 */
export const Template: React.FC<Props> = ({ applicationId, workspaceId }) => {
    return (
        <div className="mt-2">
            <LoginSettings applicationId={applicationId} workspaceId={workspaceId} />
        </div>
    );
};

export default Template;
