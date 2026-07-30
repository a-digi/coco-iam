import React from 'react';
import { Modal } from './Modal';
import { parseGeoIPInfo, formatGeoIPCountry, formatGeoIPCity, formatGeoIPOrg } from '../../../Components/Admin/Security/geoipInfo';
import { formatDate } from '../../../config/data/date/date';

// Structural shape both AdminLoginAttempt and ApplicationLoginAttempt
// satisfy — this modal doesn't care which domain the attempt came
// from, only these fields. Kept intentionally narrow (props-in,
// no API calls) so it's shareable between the admin login-log
// dashboard and the per-application AttemptsList without either
// depending on the other's types.
export interface LoginAttemptDetail {
    username: string;
    success: boolean;
    failure_reason?: string;
    ip: string;
    user_agent?: string;
    created_at: string;
    geoip_info?: string;
}

interface LoginAttemptDetailModalProps {
    attempt: LoginAttemptDetail | null;
    onClose: () => void;
}

const Field: React.FC<{ label: string; value: React.ReactNode }> = ({ label, value }) => (
    <div>
        <div className="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-1">{label}</div>
        <div className="text-sm text-gray-900 dark:text-gray-100 break-words">{value}</div>
    </div>
);

// LoginAttemptDetailModal shows everything the table row doesn't —
// user agent and the full GeoIP breakdown (Country/City/ISP) — for
// one login attempt. See plan/login-log-geoip/plan.md for the
// backend geoip_info field this reads.
export const LoginAttemptDetailModal: React.FC<LoginAttemptDetailModalProps> = ({ attempt, onClose }) => {
    const geo = attempt ? parseGeoIPInfo(attempt.geoip_info) : null;
    const geoCountry = geo ? formatGeoIPCountry(geo) : null;
    const geoCity = geo ? formatGeoIPCity(geo) : null;
    const geoOrg = geo ? formatGeoIPOrg(geo) : null;
    const hasGeoData = Boolean(geoCountry || geoCity || geoOrg);

    return (
        <Modal isOpen={attempt !== null} onClose={onClose} title="Login attempt details" maxWidth="lg">
            {attempt && (
                <div className="space-y-5">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <Field label="Username" value={attempt.username} />
                        <Field
                            label="Result"
                            value={
                                <span className={attempt.success ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}>
                                    {attempt.success ? 'Success' : 'Failed'}
                                </span>
                            }
                        />
                        {attempt.failure_reason && <Field label="Reason" value={attempt.failure_reason} />}
                        <Field label="IP address" value={attempt.ip} />
                        <Field label="When" value={formatDate(attempt.created_at)} />
                        <Field label="User agent" value={attempt.user_agent || '—'} />
                    </div>

                    <div>
                        <div className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-3">GeoIP info</div>
                        {hasGeoData ? (
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {geoCountry && <Field label="Country" value={geoCountry} />}
                                {geoCity && <Field label="City" value={geoCity} />}
                                {geoOrg && <Field label="ISP / ASN" value={geoOrg} />}
                            </div>
                        ) : (
                            <p className="text-sm text-gray-500 dark:text-gray-400">
                                No GeoIP data available for this attempt.
                            </p>
                        )}
                    </div>
                </div>
            )}
        </Modal>
    );
};

export default LoginAttemptDetailModal;
