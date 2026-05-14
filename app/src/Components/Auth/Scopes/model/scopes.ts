export interface Scope {
    id: string;
    description: string;
    scopes?: Scope[];
}

export type ScopesCategories = Scope[];
