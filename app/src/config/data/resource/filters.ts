export type FilterOperator = 'like' | 'exact' | 'date-gte' | 'date-lte' | 'gte' | 'lte';

export interface ResourceFilter {
    field: string;
    operator: FilterOperator;
    value: string | number | boolean;
}

export function buildFilterQueryString(filters?: ResourceFilter[]): string {

    if (!filters || filters.length === 0) {
        return '';
    }

    const params = new URLSearchParams();
    filters.forEach(filter => {
        const key = `filter[@${filter.operator}:${filter.field}]`;
        params.append(key, String(filter.value));
    });

    return params.toString();
}
