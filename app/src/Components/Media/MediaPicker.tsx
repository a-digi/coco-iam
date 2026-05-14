import React from 'react';
import { Modal } from '../../Shared/Components/Modal';
import { MediaBrowser } from './MediaBrowser';
import { type File } from './model/media';

interface Props {
    isOpen: boolean;
    onClose: () => void;
    applicationId: string;
    onSelect: (file: File) => void;
}

/**
 * MediaPicker is a modal wrapper around MediaBrowser used by editor
 * tabs that need to insert a file URL into user-authored content
 * (e.g. the Login Page HTML editor). Clicking a file closes the
 * modal and hands the file back via `onSelect`.
 */
export const MediaPicker: React.FC<Props> = ({ isOpen, onClose, applicationId, onSelect }) => (
    <Modal
        isOpen={isOpen}
        onClose={onClose}
        title="Pick a media file"
        maxWidth="4xl"
    >
        <MediaBrowser
            applicationId={applicationId}
            compact
            onFilePick={(f) => { onSelect(f); onClose(); }}
        />
    </Modal>
);

export default MediaPicker;
