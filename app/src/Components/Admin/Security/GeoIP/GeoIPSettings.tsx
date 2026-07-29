import React from 'react';
import { Tabs, type TabData } from '../../../../Shared/Components/Tabs';
import { SettingsTab } from './tabs/SettingsTab';
import { ExecutableTab } from './tabs/ExecutableTab';
import { IPSearchTab } from './tabs/IPSearchTab';

const TABS: TabData[] = [
    { id: 'executable', title: 'Executable', content: <ExecutableTab /> },
    { id: 'settings', title: 'Settings', content: <SettingsTab /> },
    { id: 'ip-search', title: 'IP search', content: <IPSearchTab /> },
];

// GeoIPSettings is the admin UI for configuring MaxMind GeoLite2
// credentials, controlling the geoip-updater process, and searching
// IPs against the current GeoIP data. Deliberately lives inside
// Components/Admin/Security/GeoIP (not e.g. Settings/) to mirror the
// backend's "everything within api/src/security, no shared code"
// placement for this feature — see plan/geoip-enrichment/plan.md.
//
// A thin shell over three tabs (Executable / Settings / IP search),
// each owning its own data and requests — switching tabs unmounts the
// inactive ones (Tabs only renders the active item's content), so e.g.
// the Executable tab's 5s status poll stops while another tab is
// selected and simply restarts on return, same as its original
// on-mount behavior before this split.
export const GeoIPSettings: React.FC = () => {
    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-2">GeoIP enrichment</h2>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                    Enriches attack and port-scan episodes with country/ISP data from MaxMind GeoLite2. A
                    separate geoip-updater process pulls fresh data on a loop and rebuilds a standalone
                    database — no historical IP data is retained.
                </p>
            </div>

            <Tabs items={TABS} initialActiveId="executable" />
        </div>
    );
};

export default GeoIPSettings;
