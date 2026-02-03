import { useEffect, useRef, useState, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { taskKeys } from './useTasks';
import { useToast } from '../components/ui/use-toast';

interface WebSocketMessage {
  type: 'task_updated' | 'task_created' | 'task_deleted';
  data: {
    task_id: string;
    status: string;
    updated_at: string;
    [key: string]: unknown;
  };
}

interface UseWebSocketOptions {
  onTaskUpdated?: (data: WebSocketMessage['data']) => void;
  onTaskCreated?: (data: WebSocketMessage['data']) => void;
  onTaskDeleted?: (data: WebSocketMessage['data']) => void;
}

export function useWebSocket(url: string, options?: UseWebSocketOptions) {
  const [isConnected, setIsConnected] = useState(false);
  const queryClient = useQueryClient();
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | undefined>(undefined);
  const { toast } = useToast();
  const hasNotifiedConnection = useRef(false);

  const handleMessage = useCallback((event: MessageEvent) => {
    try {
      const message: WebSocketMessage = JSON.parse(event.data);

      console.log('WebSocket message:', message);

      // Invalidate relevant queries based on message type
      switch (message.type) {
        case 'task_updated':
          queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
          queryClient.invalidateQueries({ queryKey: taskKeys.details() });
          queryClient.invalidateQueries({ queryKey: taskKeys.stats() });

          // Call custom callback
          options?.onTaskUpdated?.(message.data);

          // Show toast notification
          const taskData = message.data;
          if (taskData.status === 'completed') {
            toast({
              title: 'Task Completed',
              description: `Task ${taskData.task_id.slice(0, 8)} completed successfully`,
            });
          } else if (taskData.status === 'failed') {
            toast({
              title: 'Task Failed',
              description: `Task ${taskData.task_id.slice(0, 8)} failed`,
              variant: 'destructive',
            });
          } else if (taskData.status === 'processing') {
            toast({
              title: 'Task Processing',
              description: `Task ${taskData.task_id.slice(0, 8)} is now processing`,
            });
          }
          break;

        case 'task_created':
          queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
          queryClient.invalidateQueries({ queryKey: taskKeys.stats() });

          options?.onTaskCreated?.(message.data);

          toast({
            title: 'Task Created',
            description: `New task ${message.data.task_id.slice(0, 8)} has been created`,
          });
          break;

        case 'task_deleted':
          queryClient.invalidateQueries({ queryKey: taskKeys.lists() });
          queryClient.invalidateQueries({ queryKey: taskKeys.details() });
          queryClient.invalidateQueries({ queryKey: taskKeys.stats() });

          options?.onTaskDeleted?.(message.data);

          toast({
            title: 'Task Deleted',
            description: `Task ${message.data.task_id.slice(0, 8)} has been deleted`,
          });
          break;
      }
    } catch (error) {
      console.error('Failed to parse WebSocket message:', error);
    }
  }, [queryClient, toast, options]);

  useEffect(() => {
    function connect() {
      try {
        const ws = new WebSocket(url);
        wsRef.current = ws;

        ws.onopen = () => {
          console.log('WebSocket connected');
          setIsConnected(true);

          // Only show toast on first connection
          if (!hasNotifiedConnection.current) {
            toast({
              title: 'Connected',
              description: 'Real-time updates enabled',
            });
            hasNotifiedConnection.current = true;
          }
        };

        ws.onmessage = handleMessage;

        ws.onclose = () => {
          console.log('WebSocket disconnected');
          setIsConnected(false);

          // Attempt to reconnect after 3 seconds
          reconnectTimeoutRef.current = window.setTimeout(() => {
            console.log('Attempting to reconnect...');
            connect();
          }, 3000);
        };

        ws.onerror = (error) => {
          console.error('WebSocket error:', error);
          toast({
            title: 'Connection Error',
            description: 'Failed to connect to real-time updates',
            variant: 'destructive',
          });
        };
      } catch (error) {
        console.error('Failed to create WebSocket connection:', error);
      }
    }

    connect();

    // Cleanup function
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [url, handleMessage, toast]);

  return { isConnected };
}
