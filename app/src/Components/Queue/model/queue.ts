export interface Queue {
    id: string;
    name: string;
    description: string;
    isActive: boolean;
    createdAt: string;
}

export interface Consumer {
    id: string;
    label: string;
    registeredAt: string;
}

export interface QueueCounts {
    pending: number;
    inProgress: number;
    completed: number;
    failed: number;
    deadLettered: number;
}

export interface QueueWithCounts extends Queue {
    counts: QueueCounts;
    consumers: Consumer[];
}

// Raw shape returned by /admin/queue/queues.
export interface QueueWithCountsRaw {
    id: string;
    name: string;
    description: string;
    is_active: boolean;
    created_at: string;
    counts: {
        pending: number;
        in_progress: number;
        completed: number;
        failed: number;
        dead_lettered: number;
    };
    consumers: Array<{
        id: string;
        label: string;
        registered_at: string;
    }>;
}

export const toQueueWithCounts = (r: QueueWithCountsRaw): QueueWithCounts => ({
    id: r.id,
    name: r.name,
    description: r.description,
    isActive: r.is_active,
    createdAt: r.created_at,
    counts: {
        pending: r.counts.pending,
        inProgress: r.counts.in_progress,
        completed: r.counts.completed,
        failed: r.counts.failed,
        deadLettered: r.counts.dead_lettered,
    },
    consumers: r.consumers.map(c => ({
        id: c.id,
        label: c.label,
        registeredAt: c.registered_at,
    })),
});
