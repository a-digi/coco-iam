import type { ReactElement, ReactNode } from 'react';
import React, { Children, cloneElement, isValidElement, useState } from 'react';

interface FilterProps {
  children: ReactNode;
  onChange: (values: Record<string, string | number | null>) => void;
  className?: string;
}

function getInitialValues(children: ReactNode) {
  const initial: Record<string, string | number | null> = {};
  Children.forEach(children, (child, idx) => {
    if (isValidElement(child)) {
      const c = child as ReactElement<{ name?: string; value?: string | number }>;
      const key = c.props.name || `dropdown_${idx}`;
      initial[key] = c.props.value ?? null;
    }
  });
  return initial;
}

const Filter: React.FC<FilterProps> = ({ children, onChange, className }) => {
  const [values, setValues] = useState<Record<string, string | number | null>>(() => getInitialValues(children));

  React.useEffect(() => {
    onChange(values);
  }, [values, onChange]);

  const handleDropdownChange = (key: string) => (value: string | number) => {
    setValues(prev => {
      return { ...prev, [key]: value };
    });
  };

  const enhancedChildren = Children.map(children, (child, idx) => {
    if (!isValidElement(child)) return child;
    type DropdownOnChange = (option: { value: string | number }) => void;
    type InputOnChange = (value: string | number) => void;
    const c = child as ReactElement<{ name?: string; value?: string | number; className?: string; onChange?: DropdownOnChange | InputOnChange }>;
    const key = c.props.name || `dropdown_${idx}`;
    // Wenn es ein Dropdown ist, onChange mit Option-Objekt, sonst wie gehabt
    if (c.type && (c.type as { displayName?: string }).displayName === 'Dropdown') {
      return cloneElement(c, {
        onChange: (option: { value: string | number }) => handleDropdownChange(key)(option.value),
        name: key,
        value: values[key] === null ? undefined : values[key],
        className: `${c.props.className || ''} mx-1`,
      });
    }
    return cloneElement(c, {
      onChange: handleDropdownChange(key),
      name: key,
      value: values[key] === null ? undefined : values[key],
      className: `${c.props.className || ''} mx-1`,
    });
  });

  return (
    <div className={`flex flex-wrap gap-2 items-center bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 p-4 ${className || ''}`}>
      {enhancedChildren}
    </div>
  );
};

export default Filter;
