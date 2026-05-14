import React, { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../api/http/useHttpClient';
import { useSnackBar } from '../../Shared/Components/SnackBar/SnackBarContext';
import { API_BASE_URL } from '../../api/client';
import { findAuthToken } from '../Auth/Guard/model/auth';
import { ApplicationResource } from '../Application/model/application';
import { useApplicationSlugs } from '../Application/hooks/useApplicationSlugs';
import {
    fileUrl,
    formatBytes,
    isImageMime,
    type File,
    type Folder,
    type Listing,
    type SlugTrio,
} from './model/media';

interface Props {
    applicationId: string;
    /** When set, clicking a file calls this instead of copying its URL.
     *  Used by the MediaPicker modal. */
    onFilePick?: (file: File) => void;
    /** Compact = hide action labels, show icons only. For the picker modal. */
    compact?: boolean;
}

interface Crumb {
    id: string | null;
    label: string;
}

/**
 * MediaBrowser renders the folder tree + file grid for an
 * application's media library. Self-contained (own fetch/upload/
 * delete) so it can drop into a tab or a modal without ceremony.
 */
export const MediaBrowser: React.FC<Props> = ({ applicationId, onFilePick, compact }) => {
    const { get, post, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [currentFolderId, setCurrentFolderId] = useState<string | null>(null);
    const [crumbs, setCrumbs] = useState<Crumb[]>([{ id: null, label: 'Root' }]);
    const [listing, setListing] = useState<Listing>({ folders: [], files: [] });
    const [loading, setLoading] = useState(true);
    const [uploading, setUploading] = useState(false);
    const [creatingFolder, setCreatingFolder] = useState(false);
    const [newFolderSlug, setNewFolderSlug] = useState('');

    const mediaBase = `applications/{${ApplicationResource}}/{id:${applicationId}}/media`;
    const appSlugs = useApplicationSlugs(applicationId);
    const slugTrio: SlugTrio | undefined = appSlugs
        ? { org: appSlugs.organization_id, ws: appSlugs.workspace_id, app: appSlugs.client_id }
        : undefined;

    const refresh = useCallback(async (folderId: string | null) => {
        setLoading(true);
        try {
            const suffix = folderId ? `?parent_id=${encodeURIComponent(folderId)}` : '';
            const resp = await get<Listing>(`${mediaBase}${suffix}`) as { message?: Listing };
            setListing(resp?.message ?? { folders: [], files: [] });
        } catch (err: unknown) {
            let msg = 'Failed to load media';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage, mediaBase]);

    useEffect(() => { void refresh(currentFolderId); }, [refresh, currentFolderId]);

    const enterFolder = (f: Folder) => {
        setCurrentFolderId(f.id);
        setCrumbs(prev => [...prev, { id: f.id, label: f.slug }]);
    };

    const jumpToCrumb = (index: number) => {
        const next = crumbs.slice(0, index + 1);
        setCrumbs(next);
        setCurrentFolderId(next[next.length - 1].id);
    };

    const handleCreateFolder = async () => {
        const slug = newFolderSlug.trim();
        if (!slug) return;
        try {
            await post(`${mediaBase}/folders`, {
                parent_id: currentFolderId ?? null,
                slug,
            });
            setNewFolderSlug('');
            setCreatingFolder(false);
            successMessage('Folder created.');
            void refresh(currentFolderId);
        } catch (err: unknown) {
            let msg = 'Failed to create folder';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        }
    };

    const handleDeleteFolder = async (f: Folder) => {
        if (!window.confirm(`Delete folder "${f.slug}" and everything inside?`)) return;
        try {
            await del(`${mediaBase}/folders/{id:${f.id}}?recursive=true`);
            successMessage('Folder deleted.');
            void refresh(currentFolderId);
        } catch (err: unknown) {
            let msg = 'Failed to delete folder';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        }
    };

    const handleDeleteFile = async (f: File) => {
        if (!window.confirm(`Delete "${f.filename}"?`)) return;
        try {
            await del(`${mediaBase}/files/{id:${f.id}}`);
            successMessage('File deleted.');
            void refresh(currentFolderId);
        } catch (err: unknown) {
            let msg = 'Failed to delete file';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        }
    };

    // Multipart uploads bypass the typed http client.
    const handleUpload = async (file: globalThis.File) => {
        setUploading(true);
        try {
            const form = new FormData();
            if (currentFolderId) form.append('folder_id', currentFolderId);
            form.append('file', file);
            const token = findAuthToken();
            const res = await window.fetch(`${API_BASE_URL}${mediaBase}/files`, {
                method: 'POST',
                headers: token ? { Authorization: `Bearer ${token.access_token}` } : undefined,
                body: form,
            });
            if (!res.ok) {
                const txt = await res.text();
                throw new Error(txt || `Upload failed (${res.status})`);
            }
            successMessage('File uploaded.');
            void refresh(currentFolderId);
        } catch (err: unknown) {
            let msg = 'Upload failed';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setUploading(false);
        }
    };

    const copyUrl = async (f: File) => {
        try {
            await navigator.clipboard.writeText(fileUrl(f, slugTrio));
            successMessage('URL copied.');
        } catch {
            errorMessage('Could not copy URL.');
        }
    };

    return (
        <div className="space-y-4">
            {/* Breadcrumb + actions */}
            <div className="flex items-center justify-between flex-wrap gap-2">
                <nav className="text-sm flex items-center gap-1 text-gray-600 dark:text-gray-400 flex-wrap">
                    {crumbs.map((c, i) => (
                        <React.Fragment key={`${c.id ?? 'root'}-${i}`}>
                            {i > 0 && <span className="text-gray-400">/</span>}
                            <button
                                type="button"
                                className={`hover:text-indigo-600 dark:hover:text-indigo-400 ${i === crumbs.length - 1 ? 'font-semibold text-gray-900 dark:text-gray-100' : ''}`}
                                onClick={() => jumpToCrumb(i)}
                            >
                                {c.label}
                            </button>
                        </React.Fragment>
                    ))}
                </nav>
                <div className="flex gap-2">
                    <NewFolderButton
                        open={creatingFolder}
                        setOpen={setCreatingFolder}
                        slug={newFolderSlug}
                        setSlug={setNewFolderSlug}
                        onSave={handleCreateFolder}
                        compact={compact}
                    />
                    <UploadButton uploading={uploading} onPick={handleUpload} compact={compact} />
                </div>
            </div>

            {loading ? (
                <div className="text-sm text-gray-500 py-4">Loading…</div>
            ) : (
                <>
                    {listing.folders.length === 0 && listing.files.length === 0 ? (
                        <div className="py-10 text-center text-sm text-gray-400 italic border-2 border-dashed border-gray-200 dark:border-surface-700 rounded-lg">
                            This folder is empty. Upload a file or create a sub-folder.
                        </div>
                    ) : (
                        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
                            {listing.folders.map(f => (
                                <FolderCard
                                    key={f.id}
                                    folder={f}
                                    onOpen={() => enterFolder(f)}
                                    onDelete={() => handleDeleteFolder(f)}
                                    compact={compact}
                                />
                            ))}
                            {listing.files.map(f => (
                                <FileCard
                                    key={f.id}
                                    file={f}
                                    slugs={slugTrio}
                                    onPick={onFilePick}
                                    onCopy={() => copyUrl(f)}
                                    onDelete={() => handleDeleteFile(f)}
                                    compact={compact}
                                />
                            ))}
                        </div>
                    )}
                </>
            )}
        </div>
    );
};

const FolderCard: React.FC<{
    folder: Folder;
    onOpen: () => void;
    onDelete: () => void;
    compact?: boolean;
}> = ({ folder, onOpen, onDelete, compact }) => (
    <div className="group relative border border-gray-200 dark:border-surface-700 rounded-lg bg-white dark:bg-surface-900 overflow-hidden">
        <button
            type="button"
            onClick={onOpen}
            className="w-full flex flex-col items-center gap-1 p-3 hover:bg-gray-50 dark:hover:bg-surface-800"
        >
            <svg className="w-10 h-10 text-amber-400" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M10 4H4a2 2 0 00-2 2v12a2 2 0 002 2h16a2 2 0 002-2V8a2 2 0 00-2-2h-8l-2-2z" />
            </svg>
            <div className="text-xs text-gray-900 dark:text-gray-100 truncate w-full text-center">{folder.slug}</div>
        </button>
        {!compact && (
            <button
                type="button"
                onClick={onDelete}
                className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity bg-white/90 dark:bg-surface-900/90 text-red-600 hover:text-red-700 rounded p-1"
                aria-label="Delete folder"
                title="Delete"
            >
                <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M1 7h22M9 7V4a1 1 0 011-1h4a1 1 0 011 1v3" />
                </svg>
            </button>
        )}
    </div>
);

const FileCard: React.FC<{
    file: File;
    slugs?: SlugTrio;
    onPick?: (f: File) => void;
    onCopy: () => void;
    onDelete: () => void;
    compact?: boolean;
}> = ({ file, slugs, onPick, onCopy, onDelete, compact }) => {
    const image = isImageMime(file.mime_type);
    const body = (
        <>
            {image ? (
                <img
                    src={fileUrl(file, slugs)}
                    alt={file.filename}
                    className="h-24 w-full object-cover rounded-t-md bg-gray-100 dark:bg-surface-800"
                />
            ) : (
                <div className="h-24 w-full flex items-center justify-center bg-gray-50 dark:bg-surface-800 rounded-t-md">
                    <svg className="w-10 h-10 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                    </svg>
                </div>
            )}
            <div className="p-2 text-xs">
                <div className="truncate text-gray-900 dark:text-gray-100" title={file.filename}>{file.filename}</div>
                <div className="text-[10px] text-gray-500 uppercase">{formatBytes(file.size_bytes)} · {file.mime_type.split('/')[1]}</div>
            </div>
        </>
    );
    return (
        <div className="group relative border border-gray-200 dark:border-surface-700 rounded-lg bg-white dark:bg-surface-900 overflow-hidden">
            {onPick ? (
                <button type="button" onClick={() => onPick(file)} className="block w-full text-left hover:bg-gray-50 dark:hover:bg-surface-800">
                    {body}
                </button>
            ) : (
                <div>{body}</div>
            )}
            {!compact && !onPick && (
                <div className="absolute top-1 right-1 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                        type="button"
                        onClick={onCopy}
                        className="bg-white/90 dark:bg-surface-900/90 text-indigo-600 hover:text-indigo-700 rounded p-1"
                        title="Copy URL"
                        aria-label="Copy URL"
                    >
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                        </svg>
                    </button>
                    <button
                        type="button"
                        onClick={onDelete}
                        className="bg-white/90 dark:bg-surface-900/90 text-red-600 hover:text-red-700 rounded p-1"
                        title="Delete"
                        aria-label="Delete"
                    >
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M1 7h22M9 7V4a1 1 0 011-1h4a1 1 0 011 1v3" />
                        </svg>
                    </button>
                </div>
            )}
        </div>
    );
};

const NewFolderButton: React.FC<{
    open: boolean;
    setOpen: (b: boolean) => void;
    slug: string;
    setSlug: (s: string) => void;
    onSave: () => void;
    compact?: boolean;
}> = ({ open, setOpen, slug, setSlug, onSave, compact }) => {
    if (open) {
        return (
            <div className="flex items-center gap-1">
                <input
                    type="text"
                    autoFocus
                    value={slug}
                    onChange={e => setSlug(e.target.value.toLowerCase())}
                    onKeyDown={e => { if (e.key === 'Enter') onSave(); if (e.key === 'Escape') setOpen(false); }}
                    placeholder="folder-name"
                    className="px-2 py-1 text-sm border border-gray-300 dark:border-surface-700 rounded bg-white dark:bg-surface-900"
                />
                <button type="button" onClick={onSave} className="px-2 py-1 text-xs rounded bg-indigo-600 text-white hover:bg-indigo-500">Create</button>
                <button type="button" onClick={() => setOpen(false)} className="px-2 py-1 text-xs rounded border border-gray-300 dark:border-surface-700">Cancel</button>
            </div>
        );
    }
    return (
        <button
            type="button"
            onClick={() => setOpen(true)}
            className="inline-flex items-center gap-1 px-3 py-1.5 text-sm rounded-md border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800"
        >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
            </svg>
            {!compact && 'New folder'}
        </button>
    );
};

const UploadButton: React.FC<{ uploading: boolean; onPick: (f: globalThis.File) => void; compact?: boolean }> = ({ uploading, onPick, compact }) => {
    const ref = React.useRef<HTMLInputElement | null>(null);
    return (
        <>
            <input
                ref={ref}
                type="file"
                accept="image/png,image/jpeg,image/webp,image/gif,text/css,font/woff,font/woff2,font/ttf,font/otf,application/pdf"
                className="hidden"
                onChange={e => {
                    const f = e.target.files?.[0];
                    if (f) onPick(f);
                    e.currentTarget.value = '';
                }}
            />
            <button
                type="button"
                disabled={uploading}
                onClick={() => ref.current?.click()}
                className="inline-flex items-center gap-1 px-3 py-1.5 text-sm rounded-md bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-50"
            >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0l-4 4m4-4v12" />
                </svg>
                {uploading ? 'Uploading…' : (compact ? '' : 'Upload')}
            </button>
        </>
    );
};

export default MediaBrowser;
