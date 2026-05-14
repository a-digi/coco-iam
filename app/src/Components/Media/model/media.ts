// TS mirror of coco-server/server/media/model.go. Fields are renamed
// to match the generic "owner" naming the backend now uses.

import { API_ORIGIN } from '../../../api/client';

export interface Folder {
    id: string;
    owner_id: string;
    parent_id: string | null;
    slug: string;
    created_at?: string;
}

export interface File {
    id: string;
    owner_id: string;
    folder_id: string | null;
    filename: string;
    mime_type: string;
    size_bytes: number;
    // `<ownerID>/<folder>/.../<filename>` — appended to the
    // public media prefix to get the file-server URL. Populated by
    // the backend on every list + get.
    public_path: string;
    created_at?: string;
}

export interface Listing {
    folders: Folder[];
    files: File[];
}

// SlugTrio is the admin-chosen identifier set used to address an
// application in public URLs. When supplied, fileUrl rewrites the
// leading owner-UUID segment of `public_path` to the three-segment
// slug form, keeping UUIDs out of URLs admins copy or share.
export interface SlugTrio {
    org: string;
    ws: string;
    app: string;
}

// Public URL of an uploaded media file. The file-server route
// (`/p/media/**`, outside /api/v1/) serves bytes for any path under
// it — images are hot-linkable with no auth / CORS / versioned-API
// headaches. When `slugs` is supplied, the leading owner-UUID segment
// is rewritten to the (org, ws, app) slug trio; the backend's slug
// dispatcher resolves the trio back to the owning application's UUID.
export const fileUrl = (file: File, slugs?: SlugTrio): string => {
    if (slugs && slugs.org && slugs.ws && slugs.app) {
        const rest = file.public_path.split('/').slice(1).join('/');
        const prefix = `${encodeURIComponent(slugs.org)}/${encodeURIComponent(slugs.ws)}/${encodeURIComponent(slugs.app)}`;
        return `${API_ORIGIN}/p/media/${prefix}${rest ? '/' + rest : ''}`;
    }
    return `${API_ORIGIN}/p/media/${file.public_path}`;
};

// Classify a file by its stored mime for UI decisions (show an
// image thumbnail vs a file-type icon, etc.).
export function isImageMime(mime: string): boolean {
    return mime.startsWith('image/');
}

// Format bytes for human display.
export function formatBytes(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
