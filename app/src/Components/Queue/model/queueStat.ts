export interface QueueStat {
    name: string;
    pending: number;
    inProgress: number;
    completed: number;
    failed: number;
    deadLettered: number;
}

// The backend `/admin/queue/queues` endpoint returns the raw shape below.
export interface QueueStatRaw {
    name: string;
    pending: number;
    in_progress: number;
    completed: number;
    failed: number;
    dead_lettered: number;
}

export const toQueueStat = (r: QueueStatRaw): QueueStat => ({
    name: r.name,
    pending: r.pending,
    inProgress: r.in_progress,
    completed: r.completed,
    failed: r.failed,
    deadLettered: r.dead_lettered,
});
