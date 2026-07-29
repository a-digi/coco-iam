import React, { useEffect, useState } from 'react';
import { FormInput } from '../../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { formatGeoIPCountry, formatGeoIPOrg } from '../../geoipInfo';

interface IPSearchResult {
    ip: string;
    matched: boolean;
    country_code?: string;
    country?: string;
    asn?: number;
    as_org?: string;
}

interface IPSearchResponse {
    query: string;
    results: IPSearchResult[];
}

const DEBOUNCE_MS = 300;

// IPSearchTab is the only genuinely new piece of the Settings /
// Executable / IP search tab split — a debounced search box over
// GET admin/security/geoip/search. A complete IP gets a direct live
// GeoIP lookup; a partial address suggests IPs already seen in
// recorded attack/scan history, each also resolved live. See
// plan/geoip-enrichment/plan.md's "Extension: IP search" section.
export const IPSearchTab: React.FC = () => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [query, setQuery] = useState('');
    const [results, setResults] = useState<IPSearchResult[]>([]);
    const [searching, setSearching] = useState(false);
    const [searched, setSearched] = useState(false);

    useEffect(() => {
        const trimmed = query.trim();
        if (!trimmed) {
            setResults([]);
            setSearched(false);
            return;
        }

        const timeout = setTimeout(() => {
            void (async () => {
                setSearching(true);
                try {
                    const resp = await get<{ message: IPSearchResponse }>(
                        `admin/security/geoip/search?q=${encodeURIComponent(trimmed)}`
                    );
                    setResults(resp.message.results);
                } catch (err: unknown) {
                    errorMessage(err instanceof Error ? err.message : 'Failed to search.');
                } finally {
                    setSearching(false);
                    setSearched(true);
                }
            })();
        }, DEBOUNCE_MS);

        return () => clearTimeout(timeout);
    }, [query, get, errorMessage]);

    return (
        <div className="max-w-md space-y-4">
            <p className="text-sm text-gray-500 dark:text-gray-400">
                Look up any IP address against the current GeoIP data. A complete IP is looked up
                directly; a partial address suggests IPs already seen in recorded attack/scan history.
            </p>

            <FormInput
                id="geoip-ip-search"
                label="IP address"
                value={query}
                onChange={setQuery}
                placeholder="94.154.43.188 or 94.154."
            />

            {searching && <div className="text-sm text-gray-500 dark:text-gray-400">Searching…</div>}
            {!searching && searched && results.length === 0 && (
                <div className="text-sm text-gray-500 dark:text-gray-400">No matches.</div>
            )}

            {results.length > 0 && (
                <ul className="divide-y divide-gray-200 dark:divide-surface-800 border border-gray-200 dark:border-surface-800 rounded-lg overflow-hidden">
                    {results.map(r => (
                        <li key={r.ip} className="px-4 py-2 flex items-center justify-between gap-4 text-sm">
                            <span className="font-mono text-gray-900 dark:text-gray-100">{r.ip}</span>
                            <span className="text-gray-600 dark:text-gray-400 text-right">
                                {r.matched
                                    ? [formatGeoIPCountry(r), formatGeoIPOrg(r)].filter(Boolean).join(' · ')
                                    : 'No GeoIP data'}
                            </span>
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
};

export default IPSearchTab;
