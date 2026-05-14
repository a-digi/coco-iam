import React from 'react';
import { Modal } from '../../../../Shared/Components/Modal';
import { LOGIN_TEXT_PRESETS, type LoginTextPreset } from './LoginTextPresets';

interface Props {
    isOpen: boolean;
    onClose: () => void;
    onApply: (preset: LoginTextPreset) => void;
}

export const LoginTextPresetsModal: React.FC<Props> = ({ isOpen, onClose, onApply }) => (
    <Modal isOpen={isOpen} onClose={onClose} title="Use a suggested template" maxWidth="4xl">
        <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
            Pick a starting point. Applying a template replaces this column&rsquo;s title and content
            entries &mdash; you can still edit every line after it&rsquo;s dropped in.
        </p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {LOGIN_TEXT_PRESETS.map(preset => (
                <button
                    key={preset.id}
                    type="button"
                    onClick={() => onApply(preset)}
                    className="text-left rounded-lg border border-gray-200 dark:border-surface-700 p-4 hover:border-indigo-400 hover:shadow-md transition-all focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200"
                >
                    <div className="flex items-start justify-between gap-2 mb-1">
                        <span className="text-sm font-semibold text-gray-900 dark:text-gray-100">
                            {preset.label}
                        </span>
                        <span className="text-[0.65rem] font-semibold uppercase tracking-widest text-indigo-500">
                            Apply
                        </span>
                    </div>
                    <p className="text-xs text-gray-500 mb-3">{preset.description}</p>
                    {/* Mini preview — matches the side-panel typography rhythm
                        on a dark card so dark-on-dark presets read right. */}
                    <div className="rounded-md bg-gradient-to-br from-slate-800 to-slate-900 p-4 text-white">
                        <div
                            className="text-lg font-bold leading-tight mb-2 tracking-tight"
                            dangerouslySetInnerHTML={{ __html: preset.title }}
                        />
                        <div className="space-y-1.5 text-[0.75rem] leading-relaxed opacity-90">
                            {preset.contents.map((c, i) => (
                                <div key={i} dangerouslySetInnerHTML={{ __html: c }} />
                            ))}
                        </div>
                    </div>
                </button>
            ))}
        </div>
    </Modal>
);

export default LoginTextPresetsModal;
