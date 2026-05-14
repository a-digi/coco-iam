import React from 'react';
import { MediaBrowser } from '../../../Media/MediaBrowser';

interface Props {
    applicationId: string;
}

/**
 * Media tab on Application Edit. Just the browser — the MediaBrowser
 * owns state, fetching, folder ops and uploads. Anything that needs
 * to pick a file from elsewhere in the admin UI uses `MediaPicker`
 * (the modal wrapper).
 */
export const Media: React.FC<Props> = ({ applicationId }) => {
    return (
        <div className="mt-4">
            <div className="mb-4">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Media library</h3>
                <p className="text-sm text-gray-500">
                    Upload images, CSS, fonts, and PDFs. Organise them in folders. Every file gets
                    a public URL you can paste into any admin-authored content — the login page,
                    future branding, anywhere.
                </p>
            </div>
            <MediaBrowser applicationId={applicationId} />
        </div>
    );
};

export default Media;
