import {API_BASE_URL, buildHeaders, handleResponse} from './client.ts';

export async function get(path: string, init?: RequestInit) {
    const headers = await buildHeaders(init?.headers);
    const res = await fetch(`${API_BASE_URL}${path}`, {
        method: 'GET',
        headers,
        ...init,
    });

    return handleResponse(res);
}