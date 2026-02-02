import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { taskApi, Task, CreateTaskRequest, TaskListParams } from '../lib/api';

export const taskKeys = {
  all: ['tasks'] as const,
  lists: () => [...taskKeys.all, 'list'] as const,
  list: (params: TaskListParams) => [...taskKeys.lists(), params] as const,
  details: () => [...taskKeys.all, 'detail'] as const,
  detail: (id: string) => [...taskKeys.details(), id] as const,
  stats: () => ['stats'] as const,
};

// Hook to fetch tasks with optional filters
export function useTasks(params: TaskListParams = {}) {
  return useQuery({
    queryKey: taskKeys.list(params),
    queryFn: async () => {
      const { data } = await taskApi.list(params);
      return data;
    },
  });
}

// Hook to fetch a single task by ID
export function useTask(id: string) {
  return useQuery({
    queryKey: taskKeys.detail(id),
    queryFn: async () => {
      const { data } = await taskApi.get(id);
      return data;
    },
    enabled: !!id,
  });
}

// Hook to create a new task
export function useCreateTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateTaskRequest) => taskApi.create(data),
    onSuccess: () => {
      // Invalidate tasks list queries
      queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
    },
  });
}

// Hook to cancel a task
export function useCancelTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => taskApi.cancel(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
      queryClient.invalidateQueries({ queryKey: taskKeys.details() });
    },
  });
}

// Hook to retry a failed task
export function useRetryTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => taskApi.retry(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
      queryClient.invalidateQueries({ queryKey: taskKeys.details() });
    },
  });
}

// Hook to resurrect a dead_lettered task
export function useResurrectTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => taskApi.resurrect(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
      queryClient.invalidateQueries({ queryKey: taskKeys.details() });
    },
  });
}

// Hook to fetch statistics
export function useStats() {
  return useQuery({
    queryKey: taskKeys.stats(),
    queryFn: async () => {
      const { data } = await taskApi.stats();
      return data;
    },
    refetchInterval: 10000, // Refetch every 10 seconds
  });
}
