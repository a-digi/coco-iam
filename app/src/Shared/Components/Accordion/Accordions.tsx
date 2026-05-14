import React, { useState } from 'react';
import { AccordionItem } from './AccordionItem';
import { ScopeBasedComponentAccess } from '../Access/ScopeBasedComponentAccess';

export interface AccordionData {
    id: string;
    title: string;
    content: React.ReactNode;
    forceMount?: boolean;
    scopes?: string[];
}

export interface AccordionsProps {
    items: AccordionData[];
    initialExpandedId?: string | null;
    allowMultiple?: boolean;
    className?: string;
}

export const Accordions: React.FC<AccordionsProps> = ({
    items,
    initialExpandedId = null,
    allowMultiple = false,
    className = ""
}) => {
    const [expandedIds, setExpandedIds] = useState<Set<string>>(
        new Set(initialExpandedId ? [initialExpandedId] : [])
    );

    const toggleSection = (id: string) => {
        setExpandedIds(prev => {
            const newSet = new Set(prev);
            if (newSet.has(id)) {
                newSet.delete(id);
            } else {
                if (!allowMultiple) {
                    newSet.clear();
                }
                newSet.add(id);
            }
            return newSet;
        });
    };

    return (
        <div className={`w-full border border-gray-200 dark:border-surface-900 rounded-lg shadow-sm bg-white dark:bg-surface-800 ${className}`}>
            {items.map((item) => {
                const itemNode = (
                    <AccordionItem
                        key={item.id}
                        title={item.title}
                        isOpen={expandedIds.has(item.id)}
                        onToggle={() => toggleSection(item.id)}
                        variant="grouped"
                        forceMount={item.forceMount}
                    >
                        {item.content}
                    </AccordionItem>
                );

                if (item.scopes && item.scopes.length > 0) {
                    return (
                        <ScopeBasedComponentAccess key={item.id} requiredScopes={item.scopes}>
                            {itemNode}
                        </ScopeBasedComponentAccess>
                    );
                }

                return itemNode;
            })}
        </div>
    );
};

export default Accordions;
