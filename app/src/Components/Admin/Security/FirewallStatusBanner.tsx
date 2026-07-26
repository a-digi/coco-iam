import React, { useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import Alert from '../../../Shared/Components/Alert/Alert';

interface SecurityStatus {
    os: string;
    firewall_backend: string;
    firewall_available: boolean;
    firewall_detail?: string;
}

// FirewallStatusBanner is fetched once by SecurityPage and shown above
// whichever tab is active — see plan/ip-abuse-protection/plan.md
// sections 13-14: don't let the page silently imply full network-level
// blocking when only application-layer rate limiting (429s) is
// actually protecting this host.
export const FirewallStatusBanner: React.FC = () => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();
    const [status, setStatus] = useState<SecurityStatus | null>(null);

    useEffect(() => {
        (async () => {
            try {
                const resp = await get<{ message: SecurityStatus }>('admin/security/status');
                setStatus(resp.message);
            } catch (err: unknown) {
                errorMessage(err instanceof Error ? err.message : 'Failed to load firewall status.');
            }
        })();
    }, [get, errorMessage]);

    if (!status) {
        return null;
    }

    if (status.firewall_available) {
        return (
            <Alert variant="success" title="Firewall enforcement active" className="mb-4">
                Banned IPs are blocked at the OS level via <strong>{status.firewall_backend}</strong> ({status.os}),
                in addition to application-layer rate limiting.
            </Alert>
        );
    }

    return (
        <Alert variant="warning" title="Firewall enforcement unavailable" className="mb-4">
            Only application-layer rate limiting is active on this host ({status.os}) — banned IPs are still
            rejected here, but not blocked at the network level.{status.firewall_detail ? ` ${status.firewall_detail}.` : ''}
        </Alert>
    );
};

export default FirewallStatusBanner;
