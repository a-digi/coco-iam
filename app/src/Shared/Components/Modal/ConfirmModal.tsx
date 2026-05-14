import React from 'react';
import { Modal } from './Modal';
import { Submit, Cancel } from '../Button';

export interface ConfirmModalProps {
    isOpen?: boolean;
    onClose?: () => void;
    onCancel?: () => void;
    onConfirm: () => void | Promise<void>;
    title?: string;
    message?: React.ReactNode;
    confirmLabel?: string;
    cancelLabel?: string;
    isLoading?: boolean;
    variant?: 'primary' | 'danger';
}

export const ConfirmModal: React.FC<ConfirmModalProps> = ({
    isOpen = true,
    onClose,
    onCancel,
    onConfirm,
    title = 'Confirm Action',
    message = 'Are you sure you want to proceed?',
    confirmLabel = 'Confirm',
    cancelLabel = 'Cancel',
    isLoading = false,
    variant = 'danger'
}) => {
    const close = onClose ?? onCancel ?? (() => {});

    // Style the confirm button dynamically if it's a danger modal (e.g. Delete) vs primary
    const submitClassName = variant === 'danger'
        ? 'bg-red-600 hover:bg-red-700 focus:ring-red-500 text-white'
        : 'bg-indigo-600 hover:bg-indigo-700 focus:ring-indigo-500 text-white';

    const footer = (
        <>
            <Submit
                type="button"
                onClick={onConfirm}
                loading={isLoading}
                label={confirmLabel}
                className={submitClassName}
            />
            <Cancel
                onClick={close}
                disabled={isLoading}
                label={cancelLabel}
            />
        </>
    );

    return (
        <Modal
            isOpen={isOpen}
            onClose={close}
            title={title}
            maxWidth="sm"
            footer={footer}
            closeOnBackdropClick={!isLoading}
        >
            <div className="text-gray-600 dark:text-gray-400 text-base">
                {message}
            </div>
        </Modal>
    );
};

export default ConfirmModal;
