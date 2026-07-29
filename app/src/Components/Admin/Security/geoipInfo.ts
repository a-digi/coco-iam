// Shared parsing/formatting for the geoip_info JSON snapshot carried
// on attack and port-scan episodes — see
// plan/geoip-enrichment/plan.md. Frozen at episode-creation time on
// the backend, so this is purely a display concern here: parse
// whatever was stored, never re-derive it from a live lookup.

export interface GeoIPInfo {
    country_code?: string;
    country?: string;
    asn?: number;
    as_org?: string;
}

// parseGeoIPInfo tolerates absent/malformed input — geoip_info is
// only ever populated when geoip resolved something for a public IP
// at episode creation, so a missing/unparseable value is an expected,
// not exceptional, case (loopback IP, geoip disabled, or the lookup
// simply found nothing for that address).
export const parseGeoIPInfo = (raw?: string): GeoIPInfo | null => {
    if (!raw) return null;
    try {
        return JSON.parse(raw) as GeoIPInfo;
    } catch {
        return null;
    }
};

// Compact "DE · Deutsche Telekom AG" style summary for table columns —
// falls back to whatever fields are actually present, and to an
// em-dash when nothing parses.
export const formatGeoIPSummary = (raw?: string): string => {
    const info = parseGeoIPInfo(raw);
    if (!info) return '—';
    const parts = [info.country_code || info.country, info.as_org].filter(Boolean);
    return parts.length ? parts.join(' · ') : '—';
};

// Longer-form strings for detail-page Fields.
export const formatGeoIPCountry = (info: GeoIPInfo): string | null => {
    if (info.country && info.country_code) return `${info.country} (${info.country_code})`;
    return info.country || info.country_code || null;
};

export const formatGeoIPOrg = (info: GeoIPInfo): string | null => {
    if (info.as_org && info.asn) return `${info.as_org} (AS${info.asn})`;
    if (info.as_org) return info.as_org;
    if (info.asn) return `AS${info.asn}`;
    return null;
};
