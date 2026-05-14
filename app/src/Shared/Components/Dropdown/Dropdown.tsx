import React, { useState, useRef } from 'react';

export interface DropdownOption {
  label: string;
  value: string | number;
}

interface DropdownProps {
  name?: string;
  options: DropdownOption[];
  value?: string | number | null;
  onChange: (option: DropdownOption) => void;
  placeholder?: string;
  className?: string;
}

const Dropdown: React.FC<DropdownProps> = ({ options, value, onChange, placeholder = 'Select...', className }) => {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Der Wert wird direkt aus dem value-Prop abgeleitet, kein interner State nötig
  const selected = options.find(opt => String(opt.value) === String(value));

  return (
    <div className={`relative inline-block w-56 ${className || ''}`} ref={ref}>
      <button
        type="button"
        className="w-full bg-white dark:bg-surface-900 border border-gray-300 dark:border-gray-700 rounded px-4 py-2 text-left shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 flex justify-between items-center hover:border-blue-400 dark:hover:border-blue-500 transition"
        onClick={() => setOpen(o => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className={selected ? 'text-gray-900 dark:text-gray-100' : 'text-gray-400 dark:text-gray-500'}>
          {selected ? selected.label : placeholder}
        </span>
        <svg className={`w-4 h-4 ml-2 transition-transform ${open ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && (
        <ul
          className="absolute z-10 mt-1 w-full bg-white dark:bg-surface-900 border border-gray-200 dark:border-gray-700 rounded shadow-lg max-h-60 overflow-auto animate-fade-in"
          tabIndex={-1}
          role="listbox"
        >
          {options.length === 0 && (
            <li className="px-4 py-2 text-gray-400 dark:text-gray-500">No options</li>
          )}
          {options.map(opt => (
            <li
              key={opt.value}
              className={`px-4 py-2 cursor-pointer hover:bg-blue-50 dark:hover:bg-blue-900/40 ${String(value) === String(opt.value) ? 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-200 font-semibold' : 'text-gray-900 dark:text-gray-100'}`}
              onClick={() => {
                onChange(opt);
                setOpen(false);
              }}
              role="option"
              aria-selected={String(value) === String(opt.value)}
            >
              {opt.label}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

Dropdown.displayName = 'Dropdown';

export default Dropdown;
