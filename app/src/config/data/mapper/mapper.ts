export interface Schema {
    [key: string]: string;
}

export function mapObjects<T extends object>(schema: Schema, arr: T[]): Record<string, unknown>[] {
    return arr.map(obj => {
        const mapped: Record<string, unknown> = {};
        for (const key in schema) {
            if (Object.prototype.hasOwnProperty.call(obj, schema[key])) {
                mapped[key] = obj[schema[key] as keyof T];
            }
        }

        return mapped;
    });
}
