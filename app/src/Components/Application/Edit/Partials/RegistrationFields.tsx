import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput, FormSelect, FormTextarea } from '../../../../Shared/Components/Form';
import { Switch } from '../../../../Shared/Components/Switch';
import { ApplicationResource } from '../../model/application';
import { WorkspaceResource, WorkspaceSchema, type Workspace } from '../../../Workspace/model/workspace';
import { mapObjects } from '../../../../config/data/mapper/mapper';

// ---------- Types that mirror the admin wire shape ----------

type FieldSource = 'profile' | 'custom';
type DataType = 'text' | 'long_text' | 'number' | 'date' | 'email' | 'url' | 'select';

interface RegField {
    id: string;
    order_index: number;
    source: FieldSource;
    profile_field_id?: string | null;
    required_override?: boolean | null;
    name?: string;
    label?: string;
    description?: string;
    data_type?: DataType | string;
    is_required: boolean;
    min_value?: number | null;
    max_value?: number | null;
    options_json?: string;
    regex?: string;
}

interface RegStep {
    id: string;
    title: string;
    description?: string;
    order_index: number;
    fields: RegField[];
}

interface RegistrationDesignResponse {
    message?: { steps: RegStep[] };
}

interface ProfileFieldLite {
    id: string;
    name: string;
    label: string;
    data_type: string;
}

interface ProfileFieldsResponse {
    message?: ProfileFieldLite[];
}

interface ApplicationDetailResponse {
    message?: { registration_type?: string; allow_registration?: boolean };
}

interface Props {
    applicationId: string;
    workspaceId: string;
}

const DATA_TYPES: Array<{ label: string; value: DataType }> = [
    { label: 'Text', value: 'text' },
    { label: 'Long text', value: 'long_text' },
    { label: 'Number', value: 'number' },
    { label: 'Date', value: 'date' },
    { label: 'Email', value: 'email' },
    { label: 'URL', value: 'url' },
    { label: 'Select', value: 'select' },
];

// newID returns a stable client-generated id so drag-to-reorder +
// saves don't look like a delete+add in the audit trail. Follows
// the same convention as the login-template content blocks.
const newID = (prefix: string): string => {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
        return `${prefix}-${crypto.randomUUID()}`;
    }
    return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
};

const blankStep = (orderIndex: number): RegStep => ({
    id: newID('step'),
    title: `Step ${orderIndex + 1}`,
    description: '',
    order_index: orderIndex,
    fields: [],
});

const blankField = (stepOrder: number): RegField => ({
    id: newID('field'),
    order_index: stepOrder,
    source: 'custom',
    name: '',
    label: '',
    description: '',
    data_type: 'text',
    is_required: false,
});

// validateSteps produces per-field error messages matching the
// backend's invariants so the admin sees the problem inline before
// hitting the network. Must stay in lockstep with the server-side
// `validateAndBuild` in admin/replace_handler.go — if one gets
// stricter without the other, the UI will start showing opaque
// backend errors again.
//
// Key rules enforced here (mirrors backend):
//   - custom fields need name, label, data_type
//   - profile fields need a profile_field_id
//   - no duplicate field names across the whole design
//   - no duplicate profile_field_id references across the design
const validateSteps = (steps: RegStep[]): Record<string, string> => {
    const errors: Record<string, string> = {};
    const seenNames = new Map<string, string>();  // name → first field id that used it
    const seenProfile = new Map<string, string>(); // profile_field_id → first field id

    for (const step of steps) {
        for (const field of step.fields) {
            if (field.source === 'custom') {
                const missing: string[] = [];
                if (!field.name || field.name.trim() === '') missing.push('name');
                if (!field.label || field.label.trim() === '') missing.push('label');
                if (!field.data_type || String(field.data_type).trim() === '') missing.push('data type');
                if (missing.length > 0) {
                    errors[field.id] = `Missing: ${missing.join(', ')}.`;
                    continue;
                }
                const prev = seenNames.get(field.name!);
                if (prev && prev !== field.id) {
                    errors[field.id] = `Duplicate field name "${field.name}".`;
                    if (!errors[prev]) errors[prev] = `Duplicate field name "${field.name}".`;
                    continue;
                }
                seenNames.set(field.name!, field.id);
            } else {
                if (!field.profile_field_id) {
                    errors[field.id] = 'Pick a profile field.';
                    continue;
                }
                const prev = seenProfile.get(field.profile_field_id);
                if (prev && prev !== field.id) {
                    errors[field.id] = 'This profile field is already used by another entry.';
                    if (!errors[prev]) errors[prev] = 'This profile field is already used by another entry.';
                    continue;
                }
                seenProfile.set(field.profile_field_id, field.id);
            }
        }
    }
    return errors;
};

export const RegistrationFields: React.FC<Props> = ({ applicationId, workspaceId }) => {
    const { get, put, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [steps, setSteps] = useState<RegStep[]>([]);
    const [registrationType, setRegistrationType] = useState<string>('legacy');
    const [profileFields, setProfileFields] = useState<ProfileFieldLite[]>([]);
    // Map of field id → error message. Populated by the client-side
    // validator so the UI can highlight the offending row instead of
    // forcing the admin to chase a field id from the backend error.
    const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

    // --- Fetch -----------------------------------------------------

    const load = useCallback(async () => {
        setLoading(true);
        try {
            // 1. App row — pulls the current registration_type so
            //    admins see what's saved. allow_registration lives
            //    on the Edit form in the separate Details tab.
            const appResp = await get<ApplicationDetailResponse>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}`,
            );
            const appBody = appResp?.message;
            if (appBody?.registration_type) {
                setRegistrationType(appBody.registration_type);
            }

            // 2. Workspace → organization id. Needed to fetch the
            //    organisation's profile fields (can't guess orgID
            //    from the app row without a join; easier to pull
            //    the workspace directly).
            const wsResp = await get<{ message: unknown }>(
                `workspaces/{${WorkspaceResource}}/{id:${workspaceId}}`,
            );
            const wsRaw = wsResp?.message;
            if (wsRaw) {
                const mapped = mapObjects(WorkspaceSchema, [wsRaw] as Record<string, unknown>[]) as unknown as Workspace[];
                const orgID = mapped[0]?.organizationId;
                if (orgID) {
                    try {
                        const pfResp = await get<ProfileFieldsResponse>(
                            `organizations/{res:organizations}/{id:${orgID}}/profile-fields`,
                        );
                        setProfileFields(pfResp?.message ?? []);
                    } catch {
                        // Non-fatal — profile linking just won't be
                        // available until the admin retries.
                        setProfileFields([]);
                    }
                }
            }

            // 3. The saved registration design.
            const designResp = await get<RegistrationDesignResponse>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/registration-fields`,
            );
            // Go marshals nil slices as JSON null, so defensively
            // coerce any null `fields` on a step to an empty array
            // before it reaches useState — the StepCard's
            // `step.fields.length` access would otherwise throw.
            const loadedSteps = (designResp?.message?.steps ?? []).map(s => ({
                ...s,
                fields: s.fields ?? [],
            }));
            setSteps(loadedSteps.length > 0 ? loadedSteps : [blankStep(0)]);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load registration design');
        } finally {
            setLoading(false);
        }
    }, [get, applicationId, workspaceId, errorMessage]);

    useEffect(() => {
        void load();
    }, [load]);

    // --- Mutations -------------------------------------------------

    const patchStep = (stepID: string, patch: Partial<RegStep>) =>
        setSteps(prev => prev.map(s => (s.id === stepID ? { ...s, ...patch } : s)));

    const patchField = (stepID: string, fieldID: string, patch: Partial<RegField>) => {
        setSteps(prev =>
            prev.map(s =>
                s.id === stepID
                    ? { ...s, fields: s.fields.map(f => (f.id === fieldID ? { ...f, ...patch } : f)) }
                    : s,
            ),
        );
        // Clear any validation error on this field now that the admin
        // is editing it again. Re-validation happens on save; we don't
        // want to nag as they type.
        if (fieldErrors[fieldID]) {
            setFieldErrors(prev => {
                const next = { ...prev };
                delete next[fieldID];
                return next;
            });
        }
    };

    const addStep = () => setSteps(prev => [...prev, blankStep(prev.length)]);

    const removeStep = (stepID: string) =>
        setSteps(prev => prev.filter(s => s.id !== stepID).map((s, i) => ({ ...s, order_index: i })));

    const moveStep = (stepID: string, delta: number) =>
        setSteps(prev => {
            const idx = prev.findIndex(s => s.id === stepID);
            const target = idx + delta;
            if (idx < 0 || target < 0 || target >= prev.length) return prev;
            const next = [...prev];
            [next[idx], next[target]] = [next[target], next[idx]];
            return next.map((s, i) => ({ ...s, order_index: i }));
        });

    const addField = (stepID: string) =>
        setSteps(prev =>
            prev.map(s =>
                s.id === stepID
                    ? { ...s, fields: [...s.fields, blankField(s.fields.length)] }
                    : s,
            ),
        );

    const removeField = (stepID: string, fieldID: string) =>
        setSteps(prev =>
            prev.map(s =>
                s.id === stepID
                    ? {
                        ...s,
                        fields: s.fields.filter(f => f.id !== fieldID).map((f, i) => ({ ...f, order_index: i })),
                      }
                    : s,
            ),
        );

    const moveField = (stepID: string, fieldID: string, delta: number) =>
        setSteps(prev =>
            prev.map(s => {
                if (s.id !== stepID) return s;
                const idx = s.fields.findIndex(f => f.id === fieldID);
                const target = idx + delta;
                if (idx < 0 || target < 0 || target >= s.fields.length) return s;
                const next = [...s.fields];
                [next[idx], next[target]] = [next[target], next[idx]];
                return { ...s, fields: next.map((f, i) => ({ ...f, order_index: i })) };
            }),
        );

    // --- Save ------------------------------------------------------

    const save = async () => {
        // Pre-flight validation mirrors the backend's rules so the
        // admin sees inline highlights rather than an opaque "field
        // <uuid>: ..." response after a round-trip.
        const errors = validateSteps(steps);
        setFieldErrors(errors);
        if (Object.keys(errors).length > 0) {
            errorMessage('Fix the highlighted fields before saving.');
            return;
        }
        setSaving(true);
        try {
            // registration_type is persisted on the applications
            // row through the existing application-patch endpoint.
            // PATCH (not PUT) matches how the Edit tab already
            // writes — the generic resource handler expects a
            // partial body. Non-fatal on failure: the schema save
            // is the more important half.
            try {
                await patch(
                    `applications/{${ApplicationResource}}/{id:${applicationId}}`,
                    { registration_type: registrationType },
                );
            } catch {
                errorMessage('Could not update registration type — schema still saved.');
            }

            // Re-number before sending so whatever the UI did with
            // insertions leaves clean consecutive indexes on the
            // server side.
            const payload = {
                steps: steps.map((s, i) => ({
                    ...s,
                    order_index: i,
                    fields: s.fields.map((f, j) => ({ ...f, order_index: j })),
                })),
            };
            await put(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/registration-fields`,
                payload,
            );
            successMessage('Registration schema saved.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to save registration schema');
        } finally {
            setSaving(false);
        }
    };

    const profileByID = useMemo(() => {
        const idx = new Map<string, ProfileFieldLite>();
        profileFields.forEach(p => idx.set(p.id, p));
        return idx;
    }, [profileFields]);

    if (loading) return <div className="text-sm text-gray-500 p-4">Loading registration design…</div>;

    return (
        <div className="space-y-6 mt-2">
            <div className="rounded-lg border border-indigo-200 dark:border-indigo-900/40 bg-indigo-50 dark:bg-indigo-900/20 px-4 py-3 text-sm text-indigo-800 dark:text-indigo-200">
                <div className="font-semibold mb-1">Registration schema</div>
                Design the form external apps render at
                <code className="mx-1 text-[0.8rem] font-mono">/a/&lt;org&gt;/&lt;ws&gt;/&lt;app&gt;/registration-fields</code>.
                When <strong>allow registration</strong> is off, the endpoint returns 404 — you can still design the schema in advance.
            </div>

            <FormSelect
                id="registration_type"
                label="Registration type"
                value={registrationType}
                onChange={setRegistrationType}
                options={[
                    { label: 'Legacy (username + password)', value: 'legacy' },
                    { label: 'OAuth (coming soon)', value: 'oauth' },
                ]}
            />

            {steps.map((step, stepIdx) => (
                <StepCard
                    key={step.id}
                    step={step}
                    stepIdx={stepIdx}
                    totalSteps={steps.length}
                    profileFields={profileFields}
                    profileByID={profileByID}
                    fieldErrors={fieldErrors}
                    onPatchStep={patch => patchStep(step.id, patch)}
                    onPatchField={(fieldID, patch) => patchField(step.id, fieldID, patch)}
                    onAddField={() => addField(step.id)}
                    onRemoveField={fieldID => removeField(step.id, fieldID)}
                    onMoveField={(fieldID, delta) => moveField(step.id, fieldID, delta)}
                    onMoveStep={delta => moveStep(step.id, delta)}
                    onRemoveStep={() => removeStep(step.id)}
                />
            ))}

            <div className="flex items-center justify-between">
                <button
                    type="button"
                    onClick={addStep}
                    className="text-sm font-medium px-3 py-2 rounded-md border border-indigo-200 text-indigo-600 hover:bg-indigo-50"
                >
                    + Add step
                </button>
                <Submit type="button" onClick={() => void save()} loading={saving} label="Save schema" />
            </div>
        </div>
    );
};

// ---------- FieldRow ----------

interface FieldRowProps {
    field: RegField;
    isFirst: boolean;
    isLast: boolean;
    profileFields: ProfileFieldLite[];
    profileByID: Map<string, ProfileFieldLite>;
    // Client-side validation message for this row. Undefined = no
    // problem; a string = show it inline and apply the error border
    // so the admin spots the offender without reading IDs.
    error?: string;
    onChange: (patch: Partial<RegField>) => void;
    onRemove: () => void;
    onMoveUp: () => void;
    onMoveDown: () => void;
}

// ---------- StepCard ----------

interface StepCardProps {
    step: RegStep;
    stepIdx: number;
    totalSteps: number;
    profileFields: ProfileFieldLite[];
    profileByID: Map<string, ProfileFieldLite>;
    fieldErrors: Record<string, string>;
    onPatchStep: (patch: Partial<RegStep>) => void;
    onPatchField: (fieldID: string, patch: Partial<RegField>) => void;
    onAddField: () => void;
    onRemoveField: (fieldID: string) => void;
    onMoveField: (fieldID: string, delta: number) => void;
    onMoveStep: (delta: number) => void;
    onRemoveStep: () => void;
}

// hasStepError returns true when any of the step's fields has a
// validation error. Used by the accordion to force-expand a step
// that's hiding an offender — admins shouldn't be able to save
// with errors tucked inside a collapsed step.
const hasStepError = (step: RegStep, errors: Record<string, string>): boolean => {
    for (const f of step.fields) {
        if (errors[f.id]) return true;
    }
    return false;
};

const StepCard: React.FC<StepCardProps> = ({
    step, stepIdx, totalSteps,
    profileFields, profileByID, fieldErrors,
    onPatchStep, onPatchField, onAddField, onRemoveField, onMoveField,
    onMoveStep, onRemoveStep,
}) => {
    // Accordion: a step with zero fields defaults to expanded (the
    // admin just added it and needs to configure it); a populated
    // step defaults to collapsed so the overall list stays compact.
    // Any field-level error in this step force-expands regardless.
    const [collapsed, setCollapsed] = useState(() => step.fields.length > 0);
    const stepHasError = hasStepError(step, fieldErrors);
    const expanded = !collapsed || stepHasError;

    const stepSummary = (() => {
        const parts: string[] = [];
        if (step.title && step.title.trim() !== '') parts.push(step.title);
        parts.push(step.fields.length === 1 ? '1 field' : `${step.fields.length} fields`);
        return parts.join(' • ');
    })();

    return (
        <div className="rounded-lg border border-gray-200 dark:border-surface-700">
            {/* Header — always visible. Left half (button) toggles
                the accordion; the buttons on the right live outside
                the toggle area so they don't also flip the state. */}
            <div className="flex items-stretch">
                <button
                    type="button"
                    onClick={() => setCollapsed(c => !c)}
                    className="flex items-center gap-3 flex-1 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-surface-800/50 rounded-l-lg"
                    aria-expanded={expanded}
                >
                    <span
                        className={`inline-block transform transition-transform text-gray-500 ${expanded ? 'rotate-90' : ''}`}
                        aria-hidden="true"
                    >
                        ▶
                    </span>
                    <span className="text-[0.7rem] font-semibold uppercase tracking-widest text-gray-500">
                        Step {stepIdx + 1}
                    </span>
                    <span className="text-sm font-medium text-gray-800 dark:text-gray-200 truncate">
                        {stepSummary}
                    </span>
                    {stepHasError && (
                        <span className="ml-1 text-[0.65rem] font-semibold uppercase tracking-widest text-red-600">
                            • needs attention
                        </span>
                    )}
                </button>
                <div className="flex items-center gap-1 pr-3">
                    <button
                        type="button"
                        onClick={() => onMoveStep(-1)}
                        disabled={stepIdx === 0}
                        className="px-2 py-1 text-xs rounded border border-gray-200 dark:border-surface-700 disabled:opacity-40"
                        title="Move step up"
                    >
                        ↑
                    </button>
                    <button
                        type="button"
                        onClick={() => onMoveStep(1)}
                        disabled={stepIdx === totalSteps - 1}
                        className="px-2 py-1 text-xs rounded border border-gray-200 dark:border-surface-700 disabled:opacity-40"
                        title="Move step down"
                    >
                        ↓
                    </button>
                    <button
                        type="button"
                        onClick={onRemoveStep}
                        disabled={totalSteps === 1}
                        className="px-2 py-1 text-xs rounded border border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/50 dark:text-red-300 dark:hover:bg-red-900/30 disabled:opacity-40"
                        title="Remove step"
                    >
                        Remove step
                    </button>
                </div>
            </div>

            {/* Body — only renders when expanded (or when a field
                error forces expansion). */}
            {expanded && (
                <div className="px-4 pb-4 pt-1 space-y-4 border-t border-gray-200 dark:border-surface-700">
                    <FormInput
                        id={`step-title-${step.id}`}
                        label="Step title"
                        value={step.title}
                        onChange={v => onPatchStep({ title: v })}
                    />
                    <FormTextarea
                        id={`step-desc-${step.id}`}
                        label="Step description"
                        value={step.description ?? ''}
                        onChange={v => onPatchStep({ description: v })}
                        placeholder="Optional — short hint shown below the step title."
                    />

                    <div className="space-y-3">
                        {step.fields.length === 0 && (
                            <p className="text-xs text-gray-500 italic">
                                No fields in this step yet. Use <strong>Add field</strong> below.
                            </p>
                        )}
                        {step.fields.map((field, fIdx) => (
                            <FieldRow
                                key={field.id}
                                field={field}
                                isFirst={fIdx === 0}
                                isLast={fIdx === step.fields.length - 1}
                                profileFields={profileFields}
                                profileByID={profileByID}
                                error={fieldErrors[field.id]}
                                onChange={patch => onPatchField(field.id, patch)}
                                onRemove={() => onRemoveField(field.id)}
                                onMoveUp={() => onMoveField(field.id, -1)}
                                onMoveDown={() => onMoveField(field.id, 1)}
                            />
                        ))}
                        <button
                            type="button"
                            onClick={onAddField}
                            className="text-xs font-medium px-3 py-1.5 rounded-md border border-indigo-200 text-indigo-600 hover:bg-indigo-50"
                        >
                            + Add field
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
};

// ---------- FieldRow ----------

// isFreshField returns true when the field has no authored content
// yet (custom with empty name+label, or profile with no selection).
// The accordion uses this to decide the initial expand state —
// a freshly-added field auto-opens so the admin can fill it in;
// rows loaded from the server start collapsed.
const isFreshField = (f: RegField): boolean => {
    if (f.source === 'profile') {
        return !f.profile_field_id;
    }
    return (!f.name || f.name === '') && (!f.label || f.label === '');
};

// summaryTitle builds the short text shown on a collapsed row so
// admins can identify a field without expanding. Falls back to a
// placeholder when nothing is authored yet.
const summaryTitle = (f: RegField, profileByID: Map<string, ProfileFieldLite>): string => {
    if (f.source === 'profile') {
        const linked = f.profile_field_id ? profileByID.get(f.profile_field_id) : undefined;
        if (linked) return `${linked.label} (${linked.name})`;
        return '(profile field — not selected)';
    }
    if (f.label && f.name) return `${f.label} (${f.name})`;
    if (f.label) return f.label;
    if (f.name) return f.name;
    return '(new custom field)';
};

const FieldRow: React.FC<FieldRowProps> = ({
    field, isFirst, isLast, profileFields, profileByID, error,
    onChange, onRemove, onMoveUp, onMoveDown,
}) => {
    const linked = field.profile_field_id ? profileByID.get(field.profile_field_id) : undefined;
    const rowBorder = error
        ? 'border-red-400 dark:border-red-500'
        : 'border-gray-200 dark:border-surface-700';

    // Accordion state: start expanded when the field is freshly
    // added (admin needs to fill it in right away), collapsed when
    // it came from the server with content. A validation error
    // force-expands regardless so the admin can fix it.
    const [collapsed, setCollapsed] = useState(() => !isFreshField(field));
    const expanded = !collapsed || Boolean(error);

    return (
        <div className={`rounded-md border bg-gray-50/50 dark:bg-surface-900/30 ${rowBorder}`}>
            {/* Header row — always visible. Left half is clickable
                to toggle the accordion; the buttons on the right
                stopPropagation so clicking them doesn't also flip
                the expand state. */}
            <div className="flex items-stretch">
                <button
                    type="button"
                    onClick={() => setCollapsed(c => !c)}
                    className="flex items-center gap-2 flex-1 px-3 py-2 text-left hover:bg-gray-100/50 dark:hover:bg-surface-800/30 rounded-l-md"
                    aria-expanded={expanded}
                >
                    <span
                        className={`inline-block transform transition-transform text-gray-500 ${expanded ? 'rotate-90' : ''}`}
                        aria-hidden="true"
                    >
                        ▶
                    </span>
                    <span className="text-[0.65rem] font-semibold uppercase tracking-widest text-gray-500">
                        {field.source === 'profile' ? 'Profile' : 'Custom'}
                    </span>
                    <span className="text-sm text-gray-800 dark:text-gray-200 truncate">
                        {summaryTitle(field, profileByID)}
                    </span>
                    {error && (
                        <span className="ml-1 text-[0.65rem] font-semibold uppercase tracking-widest text-red-600">
                            • needs attention
                        </span>
                    )}
                </button>
                <div className="flex items-center gap-1 pr-2">
                    <button
                        type="button"
                        onClick={onMoveUp}
                        disabled={isFirst}
                        className="px-2 py-1 text-xs rounded border border-gray-200 dark:border-surface-700 disabled:opacity-40"
                        title="Move up"
                    >
                        ↑
                    </button>
                    <button
                        type="button"
                        onClick={onMoveDown}
                        disabled={isLast}
                        className="px-2 py-1 text-xs rounded border border-gray-200 dark:border-surface-700 disabled:opacity-40"
                        title="Move down"
                    >
                        ↓
                    </button>
                    <button
                        type="button"
                        onClick={onRemove}
                        className="px-2 py-1 text-xs rounded border border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/50 dark:text-red-300 dark:hover:bg-red-900/30"
                        title="Remove field"
                    >
                        ✕
                    </button>
                </div>
            </div>

            {/* Collapsed rows end here — the body only renders when
                expanded (or when there's a validation error to
                surface). */}
            {expanded && (
                <div className="px-3 pb-3 pt-1 space-y-3 border-t border-gray-200 dark:border-surface-700">
                    {error && (
                        <div className="rounded-md border border-red-200 dark:border-red-900/40 bg-red-50 dark:bg-red-900/20 px-3 py-2 text-xs text-red-700 dark:text-red-200">
                            {error}
                        </div>
                    )}

                    <FormSelect
                id={`field-source-${field.id}`}
                label="Source"
                value={field.source}
                onChange={v => {
                    const src = v as FieldSource;
                    // Switch: reset source-specific columns so the
                    // validator doesn't trip on stale data.
                    if (src === 'profile') {
                        onChange({
                            source: 'profile',
                            name: '',
                            label: '',
                            data_type: '',
                            min_value: null,
                            max_value: null,
                            regex: '',
                            options_json: '[]',
                        });
                    } else {
                        onChange({
                            source: 'custom',
                            profile_field_id: null,
                            required_override: null,
                        });
                    }
                }}
                options={[
                    { label: 'Link to profile field (avoid duplication)', value: 'profile' },
                    { label: 'Custom field (one-off)', value: 'custom' },
                ]}
            />

            {field.source === 'profile' && (
                <>
                    <FormSelect
                        id={`field-profile-${field.id}`}
                        label="Profile field"
                        value={field.profile_field_id ?? ''}
                        onChange={v => onChange({ profile_field_id: v })}
                        options={[
                            { label: '— select one —', value: '' },
                            ...profileFields.map(p => ({ label: `${p.label} (${p.name})`, value: p.id })),
                        ]}
                    />
                    {linked && (
                        <div className="text-xs text-gray-500 pl-1">
                            Inherits: <span className="font-mono">{linked.name}</span> ({linked.data_type})
                        </div>
                    )}
                    <Switch
                        id={`field-required-override-${field.id}`}
                        checked={field.required_override ?? false}
                        onChange={v => onChange({ required_override: v })}
                        label="Require on registration (override profile setting)"
                    />
                </>
            )}

            {field.source === 'custom' && (
                <div className="space-y-3">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                        <FormInput
                            id={`field-name-${field.id}`}
                            label="Name (machine-friendly)"
                            value={field.name ?? ''}
                            onChange={v => onChange({ name: v })}
                            placeholder="e.g. promo_code"
                        />
                        <FormInput
                            id={`field-label-${field.id}`}
                            label="Label (shown to user)"
                            value={field.label ?? ''}
                            onChange={v => onChange({ label: v })}
                            placeholder="e.g. Promo code"
                        />
                    </div>
                    <FormSelect
                        id={`field-type-${field.id}`}
                        label="Data type"
                        value={field.data_type ?? 'text'}
                        onChange={v => onChange({ data_type: v as DataType })}
                        options={DATA_TYPES}
                    />
                    <FormTextarea
                        id={`field-desc-${field.id}`}
                        label="Description (optional)"
                        value={field.description ?? ''}
                        onChange={v => onChange({ description: v })}
                    />
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                        <FormInput
                            id={`field-regex-${field.id}`}
                            label="Regex (optional)"
                            value={field.regex ?? ''}
                            onChange={v => onChange({ regex: v })}
                            placeholder="^[A-Z0-9-]+$"
                        />
                    </div>
                    <Switch
                        id={`field-required-${field.id}`}
                        checked={field.is_required}
                        onChange={v => onChange({ is_required: v })}
                        label="Required"
                    />
                </div>
            )}
                </div>
            )}
        </div>
    );
};

export default RegistrationFields;
