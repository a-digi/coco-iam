import type { Schema } from '../../../config/data/mapper/mapper.ts';

export type QueueTaskStatus = 'pending' | 'in_progress' | 'completed' | 'failed' | 'dead_lettered';

export interface QueueTask {
    id: string;
    queueName: string;
    status: QueueTaskStatus;
    attempts: number;
    maxAttempts: number;
    lastError: string;
    nextAttemptAt: string;
    createdAt: string;
    updatedAt: string;
    completedAt: string;
}

export const QueueTaskSchema: Schema = {
    id: 'id',
    queueName: 'queue_name',
    status: 'status',
    attempts: 'attempts',
    maxAttempts: 'max_attempts',
    lastError: 'last_error',
    nextAttemptAt: 'next_attempt_at',
    createdAt: 'created_at',
    updatedAt: 'updated_at',
    completedAt: 'completed_at',
};

export const QueueTaskResource = 'res:queue_tasks';
