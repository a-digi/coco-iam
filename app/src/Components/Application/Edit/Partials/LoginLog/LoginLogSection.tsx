import React, { useState } from 'react';
import { AttemptsList } from './AttemptsList';
import { ArchivesList } from './ArchivesList';

interface LoginLogSectionProps {
    applicationId: string;
}

type View = 'live' | 'archives' | { archiveId: string };

// LoginLogSection is the SideMenu item's content for one application's
// end-user login-attempt audit log. Unlike the admin login log (which
// gets real routes, mirroring Attacks/Archives — see
// plan/login-audit-log/plan.md Step 11), per-application sections in
// EditApplication.tsx are SideMenu items, not routed pages, so
// switching between the live view, the archives list, and one
// archived generation is local component state here, not
// react-router navigation. See plan/login-audit-log/plan.md Step 12.
export const LoginLogSection: React.FC<LoginLogSectionProps> = ({ applicationId }) => {
    const [view, setView] = useState<View>('live');

    if (view === 'archives') {
        return (
            <div>
                <div className="mb-4">
                    <button
                        type="button"
                        onClick={() => setView('live')}
                        className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                    >
                        ← Back to login log
                    </button>
                </div>
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4">Login log archives</h2>
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                    Rotated-out generations of this application&apos;s login-attempt database, kept indefinitely and still browsable.
                </p>
                <ArchivesList applicationId={applicationId} onSelectArchive={id => setView({ archiveId: id })} />
            </div>
        );
    }

    if (typeof view === 'object') {
        return (
            <div>
                <div className="mb-4">
                    <button
                        type="button"
                        onClick={() => setView('archives')}
                        className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                    >
                        ← Back to archives
                    </button>
                </div>
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4">Archived login attempts</h2>
                <AttemptsList applicationId={applicationId} archiveId={view.archiveId} />
            </div>
        );
    }

    return (
        <div>
            <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100">Login log</h2>
                <button
                    type="button"
                    onClick={() => setView('archives')}
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    View archives →
                </button>
            </div>
            <AttemptsList applicationId={applicationId} />
        </div>
    );
};

export default LoginLogSection;
