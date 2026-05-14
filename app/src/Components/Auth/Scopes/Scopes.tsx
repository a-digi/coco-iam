import React, { useContext, useEffect, useState } from 'react';
import { HttpClientContext } from '../../../api/http/HttpClientContext';
import { SubmitSmall } from '../../../Shared/Components/Button/SubmitSmall';
import { DefaultBadge } from '../../../Shared/Components/Badge/DefaultBadge';
import Dropdown, { type DropdownOption } from '../../../Shared/Components/Dropdown/Dropdown';
import { FormInput } from '../../../Shared/Components/Form';
import { type Scope } from './model/scopes';
import { type ApiCollectionResponse } from '../../../config/data/response/response';

interface ScopesProps {
    selectedValues?: string[];
    onChange?: (value: string) => void;
    className?: string;
}

export const Scopes: React.FC<ScopesProps> = ({ selectedValues, onChange, className }) => {
    const httpClient = useContext(HttpClientContext);
    const [options, setOptions] = useState<{ id: string; label: string; description: string }[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);
    const [searchQuery, setSearchQuery] = useState('');
    const [selectedCategory, setSelectedCategory] = useState<string>('all');

    useEffect(() => {
        const fetchScopes = async () => {
            if (!httpClient) return;

            try {
                setLoading(true);
                const response = await httpClient.get<ApiCollectionResponse<Scope>>('admin/acl/scopes');
                const flattenedOptions: typeof options = [];

                const processScope = (scope: Scope, parentLabel: string) => {
                    if (scope.id) {
                        flattenedOptions.push({
                            id: scope.id,
                            label: parentLabel || scope.id.split(':')[0] || 'GENERAL',
                            description: scope.description,
                        });
                    }
                    if (scope.scopes && scope.scopes.length > 0) {
                        scope.scopes.forEach(child => {
                            processScope(child, scope.id.toUpperCase());
                        });
                    }
                };

                const rootScopes = response?.message || [];
                rootScopes.forEach((scope) => {
                    processScope(scope, scope.id.toUpperCase());
                });

                setOptions(flattenedOptions);
            } catch (err) {
                setError('Failed to load scopes');
                console.error(err);
            } finally {
                setLoading(false);
            }
        };

        fetchScopes();
    }, [httpClient]);

    if (loading) {
        return <div className={`text-sm text-gray-500 py-2 ${className || ''}`}>Loading scopes...</div>;
    }

    if (error) {
        return <div className={`text-sm text-red-500 py-2 ${className || ''}`}>{error}</div>;
    }

    const availableOptions = options.filter(opt => {
        if (selectedValues && selectedValues.includes(opt.id)) {
            return false; // Already selected
        }

        if (searchQuery) {
            const query = searchQuery.toLowerCase();
            return opt.id.toLowerCase().includes(query) ||
                opt.label.toLowerCase().includes(query) ||
                opt.description.toLowerCase().includes(query);
        }

        return true;
    });

    const categoryOptions: DropdownOption[] = [
        { label: 'All Categories', value: 'all' },
        ...Array.from(new Set(options.map(o => o.label))).map(cat => ({
            label: cat,
            value: cat,
        }))
    ];

    const finalOptions = availableOptions.filter(opt => {
        if (selectedCategory !== 'all' && opt.label !== selectedCategory) {
            return false;
        }
        return true;
    });

    return (
        <div className={`w-full ${className || ''}`}>
            <div className="mb-4 flex flex-col sm:flex-row gap-4">
                <div className="relative flex-grow">
                    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                        <svg className="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                        </svg>
                    </div>
                    <FormInput
                        id="scopeSearch"
                        value={searchQuery}
                        onChange={setSearchQuery}
                        placeholder="Search scopes by ID, category, or description..."
                        inputClassName="pl-10"
                    />
                </div>
                <div className="w-full sm:w-64">
                    <Dropdown
                        options={categoryOptions}
                        value={selectedCategory}
                        onChange={(opt) => setSelectedCategory(String(opt.value))}
                        className="w-full"
                    />
                </div>
            </div>

            <div className="border border-gray-200 dark:border-surface-900 rounded-md overflow-hidden bg-white dark:bg-surface-800">
                <div className="max-h-64 overflow-y-auto">
                    <table className="min-w-full divide-y divide-gray-200 dark:divide-surface-900">
                        <thead className="bg-gray-50 dark:bg-surface-900 sticky top-0">
                            <tr>
                                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Scope ID</th>
                                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Category</th>
                                <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Description</th>
                                <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Action</th>
                            </tr>
                        </thead>
                        <tbody className="bg-white dark:bg-surface-800 divide-y divide-gray-200 dark:divide-surface-900">
                            {finalOptions.length === 0 ? (
                                <tr>
                                    <td colSpan={4} className="px-6 py-4 text-center text-sm text-gray-500">
                                        No scopes found matching your criteria.
                                    </td>
                                </tr>
                            ) : (
                                finalOptions.map((opt) => (
                                    <tr key={opt.id} className="hover:bg-gray-50 dark:hover:bg-surface-500 transition-colors">
                                        <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">
                                            {opt.id}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                            <DefaultBadge label={opt.label} />
                                        </td>
                                        <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                                            {opt.description}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                                            <SubmitSmall
                                                onClick={() => onChange && onChange(opt.id)}
                                            >
                                                Add
                                            </SubmitSmall>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
};

export default Scopes;
