export interface ApiCollectionResponse<T = unknown> {
    message?: T[] | null;
    success?: boolean;
}