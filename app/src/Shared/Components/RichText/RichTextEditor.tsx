import React, { useEffect, useRef, useState } from 'react';

// RichTextEditor is a minimal contentEditable-backed WYSIWYG surface.
// Features kept intentionally small: bold, paragraph, text colour,
// font size (rem-based presets), and per-block margin. Output is an
// HTML string stored as-is by the caller.
//
// NOTE on security: the editor emits HTML and consumers typically
// render it with dangerouslySetInnerHTML. Admins are the only authors
// today, but a compromised admin account could inject script. A
// sanitisation pass before render is the right next step if the
// threat model changes; for now the minimal scope is an explicit
// trade-off.
//
// NOTE on execCommand: the DOM API is flagged deprecated but remains
// universally supported across current browsers and is still the
// least-code path for a minimal editor. If the feature set grows
// meaningfully a proper library (Tiptap / Lexical / ProseMirror)
// should replace this component wholesale.

// RichTextDefaults persists the toolbar state so re-opening an
// already-edited template doesn't reset the colour swatch or the
// size / margin dropdowns to their built-in defaults. All fields
// optional.
export interface RichTextDefaults {
  foreground_color?: string;
  font_size?: string;
  block_margin?: string;
}

export interface RichTextEditorProps {
  value: string;
  onChange: (html: string) => void;
  label?: string;
  placeholder?: string;
  className?: string;
  // Minimum visible height for the editable surface, expressed as a
  // Tailwind class (defaults to "min-h-[8rem]").
  minHeightClass?: string;
  // Persistent toolbar state. `defaults` seeds the colour picker +
  // size / margin dropdowns; `onDefaultsChange` is fired whenever the
  // admin picks a new value so the parent can store it and pass the
  // same object back on the next mount.
  defaults?: RichTextDefaults;
  onDefaultsChange?: (next: RichTextDefaults) => void;
}

// Font-size presets, stored as rem so consumers can keep typography
// scales consistent across the app.
const FONT_SIZES: { label: string; value: string }[] = [
  { label: 'XS', value: '0.75rem' },
  { label: 'S', value: '0.875rem' },
  { label: 'M', value: '1rem' },
  { label: 'L', value: '1.25rem' },
  { label: 'XL', value: '1.5rem' },
  { label: '2XL', value: '2rem' },
];

// Block-level margin presets applied to the enclosing paragraph.
const MARGINS: { label: string; value: string }[] = [
  { label: 'None', value: '0' },
  { label: 'Small', value: '0.5rem' },
  { label: 'Medium', value: '1rem' },
  { label: 'Large', value: '2rem' },
];

export const RichTextEditor: React.FC<RichTextEditorProps> = ({
  value,
  onChange,
  label,
  placeholder,
  className = '',
  minHeightClass = 'min-h-[8rem]',
  defaults,
  onDefaultsChange,
}) => {
  const ref = useRef<HTMLDivElement>(null);

  // mode toggles between the WYSIWYG contentEditable surface and a
  // raw <textarea> showing the HTML source. Per-instance state — not
  // persisted server-side, since it's a viewing preference the admin
  // can flip at any time.
  const [mode, setMode] = useState<'wysiwyg' | 'html'>('wysiwyg');

  // pushDefault emits a merged defaults object so the parent can
  // store it verbatim. No-op when the caller didn't wire the prop.
  const pushDefault = (patch: Partial<RichTextDefaults>) => {
    if (!onDefaultsChange) return;
    onDefaultsChange({ ...(defaults ?? {}), ...patch });
  };

  // Sync external value into the DOM only when the editor isn't the
  // one driving the update (focused → user is typing). Resetting
  // innerHTML on every keystroke would stomp the caret. Skipped
  // entirely in HTML mode where the textarea drives the value.
  useEffect(() => {
    if (mode !== 'wysiwyg') return;
    const el = ref.current;
    if (!el) return;
    if (document.activeElement === el) return;
    if (el.innerHTML !== value) {
      el.innerHTML = value ?? '';
    }
  }, [value, mode]);

  // Announce the new HTML string to the parent. Keeps re-renders
  // bounded by calling only on input/blur events.
  const emit = () => {
    if (ref.current) onChange(ref.current.innerHTML);
  };

  // exec wraps document.execCommand with the styleWithCSS toggle
  // enabled — without it, colour commands insert <font> tags which
  // are harder to round-trip across browsers.
  const exec = (cmd: string, arg?: string) => {
    try {
      document.execCommand('styleWithCSS', false, 'true');
      document.execCommand(cmd, false, arg);
    } catch {
      // execCommand throws on unknown commands in some browsers —
      // swallow so the editor never breaks the page.
    }
    emit();
    ref.current?.focus();
  };

  // wrapSelection wraps the current range in a <span> carrying the
  // supplied inline style. Collapsed selections do nothing — size /
  // colour pickers should not affect the document when nothing is
  // highlighted.
  const wrapSelection = (style: Partial<CSSStyleDeclaration>) => {
    const selection = window.getSelection();
    if (!selection || selection.rangeCount === 0) return;
    const range = selection.getRangeAt(0);
    if (range.collapsed) return;
    if (!ref.current || !ref.current.contains(range.commonAncestorContainer)) return;

    const span = document.createElement('span');
    Object.assign(span.style, style);
    span.appendChild(range.extractContents());
    range.insertNode(span);

    // Re-select the newly wrapped span so the admin can chain
    // further commands without re-highlighting.
    selection.removeAllRanges();
    const next = document.createRange();
    next.selectNodeContents(span);
    selection.addRange(next);
    emit();
  };

  // applyBlockMargin walks up from the current selection to the
  // closest block-level element inside the editor and sets its
  // margin. If no block parent exists (selection is in the root
  // text node), a <p> is wrapped first.
  const applyBlockMargin = (margin: string) => {
    const selection = window.getSelection();
    if (!selection || selection.rangeCount === 0) return;
    const editor = ref.current;
    if (!editor) return;
    let node: Node | null = selection.getRangeAt(0).commonAncestorContainer;
    while (node && node !== editor) {
      if (
        node instanceof HTMLElement &&
        ['P', 'DIV', 'H1', 'H2', 'H3', 'H4', 'BLOCKQUOTE'].includes(node.tagName)
      ) {
        node.style.margin = margin;
        emit();
        return;
      }
      node = node.parentNode;
    }
    document.execCommand('formatBlock', false, 'p');
    applyBlockMargin(margin);
  };

  // onPaste strips non-text content so the clipboard can't inject
  // arbitrary HTML / script. Bold/colour/etc. still work via the
  // toolbar once the text is in.
  const handlePaste = (e: React.ClipboardEvent<HTMLDivElement>) => {
    e.preventDefault();
    const text = e.clipboardData.getData('text/plain');
    document.execCommand('insertText', false, text);
  };

  return (
    <div className={className}>
      {label && (
        <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
          {label}
        </label>
      )}
      <div className="rounded-lg border border-gray-200 dark:border-surface-700 overflow-hidden bg-white dark:bg-surface-900">
        <div className="flex flex-wrap items-center gap-1 px-2 py-1.5 bg-gray-50 dark:bg-surface-800 border-b border-gray-200 dark:border-surface-700 text-sm">
          {/* Formatting controls — only meaningful while editing the
              rendered view. Disabled in HTML mode so the admin can
              see at a glance that typing edits raw source. */}
          <ToolbarButton
            title="Bold (Ctrl+B)"
            onClick={() => exec('bold')}
            disabled={mode === 'html'}
          >
            <span className="font-bold">B</span>
          </ToolbarButton>
          <ToolbarButton
            title="Paragraph"
            onClick={() => exec('formatBlock', 'p')}
            disabled={mode === 'html'}
          >
            <span className="text-base">¶</span>
          </ToolbarButton>

          <div className="h-5 w-px bg-gray-200 dark:bg-surface-600 mx-1" aria-hidden />

          <label
            className={`inline-flex items-center gap-1 px-2 h-7 rounded text-gray-700 dark:text-gray-200 ${
              mode === 'html'
                ? 'opacity-40 cursor-not-allowed'
                : 'cursor-pointer hover:bg-gray-100 dark:hover:bg-surface-700'
            }`}
            title="Text color"
          >
            <span className="text-xs">A</span>
            <input
              type="color"
              disabled={mode === 'html'}
              className="h-4 w-5 rounded border-0 bg-transparent cursor-pointer disabled:cursor-not-allowed"
              // `value` + onChange keeps the swatch locked to the
              // persisted default across re-mounts. Fallback to a
              // neutral dark grey so the input has a visible swatch
              // before the admin ever touches it.
              value={defaults?.foreground_color ?? '#111827'}
              onChange={e => {
                exec('foreColor', e.target.value);
                pushDefault({ foreground_color: e.target.value });
              }}
            />
          </label>

          <SelectTool
            title="Font size"
            placeholderLabel="Size"
            value={defaults?.font_size ?? ''}
            options={FONT_SIZES}
            disabled={mode === 'html'}
            onPick={v => {
              wrapSelection({ fontSize: v });
              pushDefault({ font_size: v });
            }}
          />

          <SelectTool
            title="Block margin"
            placeholderLabel="Margin"
            value={defaults?.block_margin ?? ''}
            options={MARGINS}
            disabled={mode === 'html'}
            onPick={v => {
              applyBlockMargin(v);
              pushDefault({ block_margin: v });
            }}
          />

          {/* Right-aligned HTML toggle. Always enabled — it's how the
              admin switches back to the WYSIWYG view. */}
          <div className="ml-auto">
            <ToolbarButton
              title={mode === 'html' ? 'Back to visual editor' : 'Edit HTML source'}
              onClick={() => setMode(mode === 'html' ? 'wysiwyg' : 'html')}
              active={mode === 'html'}
            >
              <span className="font-mono text-xs">{'</>'}</span>
            </ToolbarButton>
          </div>
        </div>

        {mode === 'wysiwyg' ? (
          <div
            ref={ref}
            contentEditable
            suppressContentEditableWarning
            onInput={emit}
            onBlur={emit}
            onPaste={handlePaste}
            className={`${minHeightClass} px-3 py-2 text-[0.9375rem] text-gray-900 dark:text-gray-100 outline-none empty:before:text-gray-400 empty:before:content-[attr(data-placeholder)]`}
            style={{ wordBreak: 'break-word' }}
            data-placeholder={placeholder ?? ''}
          />
        ) : (
          // HTML source view — raw textarea bound to the same value.
          // The admin types / pastes any HTML they want; switching
          // back to WYSIWYG rehydrates the contentEditable from this
          // exact string.
          <textarea
            value={value}
            onChange={e => onChange(e.target.value)}
            placeholder="<p>Your HTML…</p>"
            spellCheck={false}
            className={`${minHeightClass} w-full px-3 py-2 font-mono text-[0.8125rem] text-gray-900 dark:text-gray-100 bg-white dark:bg-surface-900 outline-none resize-y`}
          />
        )}
      </div>
    </div>
  );
};

// ToolbarButton is a square icon-style button for the toolbar row.
// `active` highlights the button (used by the HTML toggle to signal
// the current mode); `disabled` greys it out and blocks clicks.
const ToolbarButton: React.FC<{
  title: string;
  onClick: () => void;
  children: React.ReactNode;
  active?: boolean;
  disabled?: boolean;
}> = ({ title, onClick, children, active, disabled }) => (
  <button
    type="button"
    title={title}
    disabled={disabled}
    onMouseDown={e => e.preventDefault()}
    onClick={onClick}
    className={`inline-flex items-center justify-center h-7 w-7 rounded text-gray-700 dark:text-gray-200 ${
      disabled
        ? 'opacity-40 cursor-not-allowed'
        : active
          ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-200'
          : 'hover:bg-gray-100 dark:hover:bg-surface-700'
    }`}
  >
    {children}
  </button>
);

// SelectTool is a dropdown styled to match the icon buttons. Keeps
// the last picked value visible so admins see the current default
// without having to open the list. The placeholder option (disabled
// string key `""`) is shown only when `value` is empty.
const SelectTool: React.FC<{
  title: string;
  placeholderLabel: string;
  value: string;
  options: { label: string; value: string }[];
  onPick: (value: string) => void;
  disabled?: boolean;
}> = ({ title, placeholderLabel, value, options, onPick, disabled }) => (
  <select
    title={title}
    aria-label={title}
    disabled={disabled}
    onMouseDown={e => e.stopPropagation()}
    value={value}
    onChange={e => {
      const v = e.target.value;
      if (v) onPick(v);
    }}
    className={`h-7 rounded bg-transparent text-xs text-gray-700 dark:text-gray-200 px-2 focus:outline-none ${
      disabled
        ? 'opacity-40 cursor-not-allowed'
        : 'hover:bg-gray-100 dark:hover:bg-surface-700'
    }`}
  >
    <option value="" disabled>
      {placeholderLabel}
    </option>
    {options.map(o => (
      <option key={o.value} value={o.value}>
        {o.label}
      </option>
    ))}
  </select>
);

export default RichTextEditor;
