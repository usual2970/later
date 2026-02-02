import axios from 'axios';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

export const api = axios.create({
  baseURL: `${API_BASE_URL}/api/v1`,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Task types
export type TaskStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'dead_lettered';

export interface Task {
  id: string;
  name: string;
  payload: any;
  callback_url: string;
  status: TaskStatus;
  created_at: string;
  scheduled_at: string;
  started_at?: string;
  completed_at?: string;
  max_retries: number;
  retry_count: number;
  callback_attempts: number;
  priority: number;
  tags?: string[];
  error_message?: string;
  estimated_execution?: string;
}

export interface CreateTaskRequest {
  name: string;
  payload: any;
  callback_url: string;
  scheduled_for?: string;
  timeout_seconds?: number;
  max_retries?: number;
  priority?: number;
  tags?: string[];
}

export interface TaskListParams {
  status?: TaskStatus;
  priority?: number;
  tags?: string;
  date_from?: string;
  date_to?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}

export interface TaskListResponse {
  tasks: Task[];
  pagination: {
    page: number;
    limit: number;
    total: number;
    total_pages: number;
  };
}

export interface Stats {
  total: number;
  by_status: Record<TaskStatus, number>;
  last_24h: {
    submitted: number;
    completed: number;
    failed: number;
  };
  callback_success_rate: number;
}

// API functions
export const taskApi = {
  // Create a new task
  create: (data: CreateTaskRequest) => api.post<Task>('/tasks', data),

  // List tasks with filters
  list: (params: TaskListParams) => api.get<TaskListResponse>('/tasks', { params }),

  // Get task by ID
  get: (id: string) => api.get<Task>(`/tasks/${id}`),

  // Cancel a pending task
  cancel: (id: string) => api.delete(`/tasks/${id}`),

  // Retry a failed task
  retry: (id: string) => api.post<Task>(`/tasks/${id}/retry`),

  // Resurrect a dead_lettered task
  resurrect: (id: string) => api.post<Task>(`/tasks/${id}/resurrect`),

  // Get statistics
  stats: () => api.get<Stats>('/tasks/stats'),
};
