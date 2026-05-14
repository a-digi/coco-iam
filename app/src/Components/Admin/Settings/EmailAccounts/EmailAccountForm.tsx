import React, { useCallback, useEffect, useState, type SyntheticEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../../Shared/Components/Font/Title';
import { Submit, SubmitSmall, Cancel } from '../../../../Shared/Components/Button';
import { Switch } from '../../../../Shared/Components/Switch';
import { FormInput } from '../../../../Shared/Components/Form';
import { type EmailAccount, ACCOUNT_NAME_PATTERN } from './model/emailAccount';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export interface EmailAccountFormProps {
    mode: 'create' | 'edit';
}

export const EmailAccountForm: React.FC<EmailAccountFormProps> = ({ mode }) => {
    useBreadcrumbItems([
        { label: 'Admin' },
        { label: 'Settings' },
        { label: 'Email Accounts', href: '/admin/settings/email-accounts' },
        { label: mode === 'create' ? 'Create' : 'Edit' },
    ]);
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const { get, post, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [name, setName] = useState('');
    const [host, setHost] = useState('');
    const [port, setPort] = useState(587);
    const [username, setUsername] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [fromName, setFromName] = useState('');
    const [fromEmail, setFromEmail] = useState('');
    const [useTLS, setUseTLS] = useState(false);
    const [isActive, setIsActive] = useState(false);
    const [fetching, setFetching] = useState(mode === 'edit');
    const [submitting, setSubmitting] = useState(false);
    const [testing, setTesting] = useState(false);
    const [testTo, setTestTo] = useState('');

    useEffect(() => {
        if (mode !== 'edit' || !id) return;
        let cancelled = false;
        (async () => {
            setFetching(true);
            try {
                const response = await get<{ message?: EmailAccount }>(`admin/mail/accounts/{id:${id}}`);
                const a = response?.message;
                if (!a) {
                    errorMessage('Account not found');
                    return;
                }
                if (cancelled) return;
                setName(a.name);
                setHost(a.host);
                setPort(a.port);
                setUsername(a.username);
                setFromName(a.from_name);
                setFromEmail(a.from_email);
                setUseTLS(a.use_tls);
                setIsActive(a.is_active);
            } catch (err: unknown) {
                let msg = 'Failed to load account';
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
            if (!ACCOUNT_NAME_PATTERN.test(trimmed)) {
                errorMessage('Name must start with a letter and contain only lowercase letters, digits, underscore or hyphen.');
                return;
            }
        }
        if (!host.trim()) {
            errorMessage('Host is required');
            return;
        }
        if (port < 1 || port > 65535) {
            errorMessage('Port must be between 1 and 65535.');
            return;
        }
        if (mode === 'create' && !newPassword && username.trim()) {
            errorMessage('Password is required when a username is set.');
            return;
        }

        setSubmitting(true);
        try {
            if (mode === 'create') {
                await post('admin/mail/accounts', {
                    name: name.trim(),
                    host,
                    port,
                    username,
                    password: newPassword,
                    from_name: fromName,
                    from_email: fromEmail,
                    use_tls: useTLS,
                    is_active: isActive,
                });
                successMessage(`Account ${name} created.`);
            } else if (id) {
                const body: Record<string, unknown> = {
                    host,
                    port,
                    username,
                    from_name: fromName,
                    from_email: fromEmail,
                    use_tls: useTLS,
                };
                if (newPassword !== '') body.password = newPassword;
                await patch(`admin/mail/accounts/{id:${id}}`, body);
                successMessage(`Account ${name} updated.`);
            }
            navigate('/admin/settings/email-accounts');
        } catch (err: unknown) {
            let msg = mode === 'create' ? 'Failed to create account' : 'Failed to update account';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [mode, id, name, host, port, username, newPassword, fromName, fromEmail, useTLS, isActive,
        post, patch, navigate, successMessage, errorMessage]);

    const handleTest = useCallback(async () => {
        if (mode !== 'edit' || !id) {
            errorMessage('Save the account first, then use the test button.');
            return;
        }
        const to = testTo.trim();
        if (!to) {
            errorMessage('Enter a recipient email for the test.');
            return;
        }
        setTesting(true);
        try {
            await post(`admin/mail/accounts/{id:${id}}/test`, { to, name: 'admin' });
            successMessage(`Test email sent to ${to} via ${name}.`);
        } catch (err: unknown) {
            let msg = 'SMTP test failed';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setTesting(false);
        }
    }, [mode, id, testTo, name, post, successMessage, errorMessage]);

    if (fetching) {
        return <div className="text-sm text-gray-500 py-4">Loading account…</div>;
    }

    return (
        <div className="max-w-3xl">
            <Title>{mode === 'create' ? 'Create email account' : `Edit account: ${name}`}</Title>

            <form onSubmit={handleSubmit} className="mt-6 space-y-5">
                <FormInput
                    id="name"
                    label="Name"
                    value={name}
                    onChange={setName}
                    disabled={mode === 'edit'}
                    required
                    placeholder="e.g. dev-mailpit, production-postmark"
                    description="Lowercase letters, digits, underscore, hyphen; must start with a letter. Cannot be renamed after creation."
                    inputClassName="font-mono text-sm"
                />

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <FormInput
                        id="host"
                        label="Host"
                        value={host}
                        onChange={setHost}
                        required
                        placeholder="smtp.example.com"
                    />
                    <FormInput
                        id="port"
                        type="number"
                        label="Port"
                        value={port}
                        onChange={v => setPort(parseInt(v, 10) || 0)}
                        min={1}
                        max={65535}
                    />
                    <FormInput
                        id="username"
                        label="Username"
                        value={username}
                        onChange={setUsername}
                        placeholder="(optional)"
                    />
                    <FormInput
                        id="newPassword"
                        type="password"
                        label="Password"
                        value={newPassword}
                        onChange={setNewPassword}
                        placeholder={mode === 'edit' ? 'Leave blank to keep the stored password' : ''}
                        autoComplete="new-password"
                    />
                    <FormInput
                        id="fromName"
                        label="From name"
                        value={fromName}
                        onChange={setFromName}
                        placeholder="coco-iam"
                    />
                    <FormInput
                        id="fromEmail"
                        type="email"
                        label="From email"
                        value={fromEmail}
                        onChange={setFromEmail}
                        placeholder="noreply@example.com"
                    />
                </div>

                <div className="flex items-center gap-6">
                    <Switch checked={useTLS} onChange={setUseTLS} label="Use TLS" />
                    {mode === 'create' && (
                        <Switch checked={isActive} onChange={setIsActive} label="Activate immediately" />
                    )}
                </div>

                {mode === 'edit' && (
                    <div className="p-4 border border-gray-200 dark:border-surface-800 rounded-lg bg-gray-50 dark:bg-surface-900/40">
                        <div className="flex items-center justify-between gap-3 flex-wrap">
                            <div>
                                <div className="text-sm font-medium text-gray-800 dark:text-gray-200">Send a test email</div>
                                <div className="text-xs text-gray-500">
                                    Uses this account regardless of which one is active. Bypasses the queue and reports
                                    the SMTP outcome directly.
                                </div>
                            </div>
                            <div className="flex items-center gap-2">
                                <input
                                    type="email"
                                    value={testTo}
                                    onChange={e => setTestTo(e.target.value)}
                                    placeholder="recipient@example.com"
                                    className="px-3 py-2 border border-gray-300 dark:border-surface-700 rounded-lg bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                                />
                                <SubmitSmall type="button" onClick={handleTest} disabled={testing}>
                                    {testing ? 'Sending…' : 'Send test'}
                                </SubmitSmall>
                            </div>
                        </div>
                    </div>
                )}

                <div className="flex items-center justify-end gap-3 pt-2">
                    <Cancel to="/admin/settings/email-accounts" />
                    <Submit
                        loading={submitting}
                        loadingText="Saving…"
                        label={mode === 'create' ? 'Create account' : 'Save changes'}
                    />
                </div>
            </form>
        </div>
    );
};

export default EmailAccountForm;
