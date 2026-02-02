import { Badge } from './ui/badge';
import { TaskStatus } from '../lib/api';

interface StatusBadgeProps {
  status: TaskStatus;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const variants: Record<TaskStatus, 'default' | 'success' | 'warning' | 'destructive' | 'info'> = {
    pending: 'warning',
    processing: 'info',
    completed: 'success',
    failed: 'destructive',
    dead_lettered: 'default',
  };

  const labels: Record<TaskStatus, string> = {
    pending: 'Pending',
    processing: 'Processing',
    completed: 'Completed',
    failed: 'Failed',
    dead_lettered: 'Dead Lettered',
  };

  return (
    <Badge variant={variants[status]}>
      {labels[status]}
    </Badge>
  );
}
