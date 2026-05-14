import React, { useCallback, useEffect, useState, type SyntheticEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../../Shared/Components/Font/Title';
import { Submit, Cancel } from '../../../../Shared/Components/Button';
import { Switch } from '../../../../Shared/Components/Switch';
import { FormInput, FormTextarea } from '../../../../Shared/Components/Form';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import {
    type EmailTemplate,
    EmailTemplateSchema,
    TEMPLATE_NAME_PATTERN,
} from './model/emailTemplate';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

interface StarterToken {
    name: string;
    description: string;
    example?: string;
}

interface Starter {
    name: string;
    display_name: string;
    description: string;
    subject: string;
    text_body: string;
    html_body: string;
    tokens: StarterToken[];
}

export interface EmailTemplateFormProps {
    mode: 'create' | 'edit';
}

export const EmailTemplateForm: React.FC<EmailTemplateFormProps> = ({ mode }) => {
    useBreadcrumbItems([
        { label: 'Admin' },
        { label: 'Settings' },
        { label: 'Email Templates', href: '/admin/settings/email-templates' },
        { label: mode === 'create' ? 'Create' : 'Edit' },
    ]);
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const { get, post, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [subject, setSubject] = useState('');
    // Single content area; the isHtml flag chooses which backend column the
    // content lands in at save time (text_body vs html_body).
    const [content, setContent] = useState('');
    const [isHtml, setIsHtml] = useState(true);
    const [isActive, setIsActive] = useState(true);
    const [fetching, setFetching] = useState(mode === 'edit');
    const [submitting, setSubmitting] = useState(false);

    const [starters, setStarters] = useState<Starter[]>([]);
    const [selectedStarter, setSelectedStarter] = useState<string>('');

    // Load starter catalog once, only in create mode.
    useEffect(() => {
        if (mode !== 'create') return;
        let cancelled = false;
        (async () => {
            try {
                const response = await get<{ message?: Starter[] }>('admin/mail/template-starters');
                const list = response?.message ?? [];
                if (!cancelled && Array.isArray(list)) setStarters(list);
            } catch {
                // Starters are optional — form still works without them.
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [mode, get]);

    const activeStarter = starters.find(s => s.name === selectedStarter);

    const applyStarter = useCallback((starterName: string) => {
        setSelectedStarter(starterName);
        if (!starterName) return;
        const starter = starters.find(s => s.name === starterName);
        if (!starter) return;
        if (!name.trim()) setName(starter.name);
        if (!description.trim()) setDescription(starter.display_name);
        setSubject(starter.subject);
        // Prefer the HTML body when the starter ships both — it's the richer
        // version. The admin can uncheck the HTML flag to switch to plain text.
        if (starter.html_body) {
            setContent(starter.html_body);
            setIsHtml(true);
        } else {
            setContent(starter.text_body);
            setIsHtml(false);
        }
    }, [starters, name, description]);

    useEffect(() => {
        if (mode !== 'edit' || !id) return;
        let cancelled = false;
        (async () => {
            setFetching(true);
            try {
                const response = await get<{ message?: unknown }>(`admin/mail/templates/{id:${id}}`);
                const raw = response?.message;
                if (!raw) {
                    errorMessage('Template not found');
                    return;
                }
                const mapped = mapObjects(EmailTemplateSchema, [raw]) as unknown as EmailTemplate[];
                const t = mapped[0];
                if (cancelled || !t) return;
                setName(t.name);
                setDescription(t.description);
                setSubject(t.subject);
                // If both columns exist we show HTML (it's the richer view);
                // the admin can toggle the flag to switch to plain text.
                if (t.htmlBody) {
                    setContent(t.htmlBody);
                    setIsHtml(true);
                } else {
                    setContent(t.textBody);
                    setIsHtml(false);
                }
                setIsActive(t.isActive);
            } catch (err: unknown) {
                let msg = 'Failed to load template';
                if (err instanceof Error) msg = err.message || msg;
                errorMessage(msg);
            } finally {
                if (!cancelled) setFetching(false);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [mode, id, get, errorMessage]);

    const handleSubmit = useCallback(async (e: SyntheticEvent) => {
        e.preventDefault();
        if (mode === 'create') {
            const trimmed = name.trim();
            if (!trimmed) {
                errorMessage('Name is required');
                return;
            }
            if (!TEMPLATE_NAME_PATTERN.test(trimmed)) {
                errorMessage('Name must start with a letter and contain only lowercase letters, digits, underscore or hyphen.');
                return;
            }
        }
        if (!content.trim()) {
            errorMessage('Content cannot be empty.');
            return;
        }

        // Route content into exactly one body column; the other is cleared so
        // backend readers don't accidentally send the stale version from the
        // previous save (when the admin flipped the checkbox).
        const textBody = isHtml ? '' : content;
        const htmlBody = isHtml ? content : '';

        setSubmitting(true);
        try {
            if (mode === 'create') {
                await post('admin/mail/templates', {
                    name: name.trim(),
                    description,
                    subject,
                    text_body: textBody,
                    html_body: htmlBody,
                    is_active: isActive,
                });
                successMessage(`Template ${name} created.`);
            } else if (id) {
                await patch(`admin/mail/templates/{id:${id}}`, {
                    description,
                    subject,
                    text_body: textBody,
                    html_body: htmlBody,
                    is_active: isActive,
                });
                successMessage(`Template ${name} updated.`);
            }
            navigate('/admin/settings/email-templates');
        } catch (err: unknown) {
            let msg = mode === 'create' ? 'Failed to create template' : 'Failed to update template';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [mode, id, name, description, subject, content, isHtml, isActive,
        post, patch, navigate, successMessage, errorMessage]);

    if (fetching) {
        return <div className="text-sm text-gray-500 py-4">Loading template...</div>;
    }

    return (
        <div className="max-w-3xl">
            <Title>{mode === 'create' ? 'Create email template' : `Edit template: ${name}`}</Title>

            <form onSubmit={handleSubmit} className="mt-6 space-y-5">
                {mode === 'create' && starters.length > 0 && (
                    <div className="p-4 border border-indigo-100 dark:border-indigo-900 bg-indigo-50/60 dark:bg-indigo-900/20 rounded-lg">
                        <label className="block text-sm font-medium text-indigo-900 dark:text-indigo-200 mb-1">
                            Start from template
                        </label>
                        <select
                            value={selectedStarter}
                            onChange={e => applyStarter(e.target.value)}
                            className="w-full px-3 py-2 border border-indigo-200 dark:border-indigo-800 rounded-lg bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                        >
                            <option value="">— Blank template —</option>
                            {starters.map(s => (
                                <option key={s.name} value={s.name}>{s.display_name}</option>
                            ))}
                        </select>
                        {activeStarter && (
                            <div className="mt-3 text-xs text-indigo-900 dark:text-indigo-200">
                                <div>{activeStarter.description}</div>
                                {activeStarter.tokens.length > 0 && (
                                    <div className="mt-2">
                                        <div className="font-medium">Available tokens:</div>
                                        <ul className="mt-1 space-y-0.5 list-disc list-inside">
                                            {activeStarter.tokens.map(tok => (
                                                <li key={tok.name}>
                                                    <code className="font-mono bg-white/60 dark:bg-surface-900/60 px-1 rounded">{`{{ .${tok.name} }}`}</code>
                                                    {' — '}{tok.description}
                                                </li>
                                            ))}
                                        </ul>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                )}

                <FormInput
                    id="name"
                    label="Name"
                    value={name}
                    onChange={setName}
                    disabled={mode === 'edit'}
                    required
                    placeholder="e.g. welcome, password-reset"
                    description="Lowercase letters, digits, underscore, hyphen; must start with a letter. Cannot be renamed after creation."
                    inputClassName="font-mono text-sm"
                />

                <FormInput
                    id="description"
                    label="Description"
                    value={description}
                    onChange={setDescription}
                />

                <FormInput
                    id="subject"
                    label="Subject"
                    value={subject}
                    onChange={setSubject}
                />

                <div>
                    <div className="flex items-center justify-between mb-1">
                        <label htmlFor="content" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                            Content {isHtml
                                ? <span className="text-xs text-indigo-600 dark:text-indigo-400 font-normal">(HTML)</span>
                                : <span className="text-xs text-gray-500 font-normal">(plain text)</span>
                            }
                        </label>
                        <Switch
                            checked={isHtml}
                            onChange={setIsHtml}
                            label={isHtml ? 'HTML' : 'Text'}
                        />
                    </div>
                    <FormTextarea
                        id="content"
                        value={content}
                        onChange={setContent}
                        rows={16}
                        textareaClassName="font-mono text-sm"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                        Use <code>{'{{ .Name }}'}</code> syntax for template variables. The mail task's <code>Data</code> map is the template scope.
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    <Switch checked={isActive} onChange={setIsActive} />
                    <span className="text-sm text-gray-700 dark:text-gray-300">Active</span>
                </div>

                <div className="flex items-center justify-end gap-3 pt-2">
                    <Cancel to="/admin/settings/email-templates" />
                    <Submit
                        loading={submitting}
                        loadingText="Saving..."
                        label={mode === 'create' ? 'Create template' : 'Save changes'}
                    />
                </div>
            </form>
        </div>
    );
};

export default EmailTemplateForm;
