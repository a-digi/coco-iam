export const parseJwt = (token: string) => {
    try {
        const base64Url = token.split('.')[1];
        if (!base64Url) return null;
        const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
        const jsonPayload = decodeURIComponent(
            window
                .atob(base64)
                .split('')
                .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
                .join('')
        );

        const result = JSON.parse(jsonPayload) as Record<string, unknown>;
        if (result && typeof result.scope === 'string' && !Array.isArray(result.scopes)) {
            result.scopes = result.scope.split(' ').filter(Boolean);
        }
        return result;
    } catch (error) {
        console.error("JWT Parsing Error:", error);
        return null;
    }
};