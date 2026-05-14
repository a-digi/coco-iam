import { useCallback } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { mapObjects, type Schema } from '../mapper/mapper';
import type { ApiCollectionResponse } from '../response/response';
import { buildFilterQueryString, type ResourceFilter } from './filters';

export interface FetchCollectionOptions {
    filters?: ResourceFilter[];
}

export function useResourceRepository() {
    const { get } = useHttpClient();

    const fetchCollection = useCallback(async <T>(
        endpoint: string,
        schema: Record<string, unknown>,
        options?: FetchCollectionOptions
    ): Promise<T[]> => {
        let url = endpoint;
        const queryString = buildFilterQueryString(options?.filters);

        if (queryString) {
            url += url.includes('?') ? `&${queryString}` : `?${queryString}`;
        }

        const data = await get(url);
        const response = data as ApiCollectionResponse<Record<string, unknown>>;
        const rawItems = Array.isArray(response?.message) ? response.message : [];
        return mapObjects(schema as unknown as Schema, rawItems) as unknown as T[];
    }, [get]);

    return {
        fetchCollection,
    };
}
