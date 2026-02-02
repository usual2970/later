import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { StatusBadge } from './StatusBadge';
import { Copy, Check } from 'lucide-react';
import { useState } from 'react';
import type { Task } from '../lib/api';

interface TaskDetailProps {
  task: Task | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function TaskDetail({ task, open, onOpenChange }: TaskDetailProps) {
  const [copied, setCopied] = useState(false);

  if (!task) return null;

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center justify-between">
            <span>{task.name}</span>
            <StatusBadge status={task.status} />
          </DialogTitle>
          <DialogDescription>
            Task ID: <code className="text-xs">{task.id}</code>
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Basic Info */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-medium text-muted-foreground">Priority</label>
              <p className="text-lg">{task.priority}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-muted-foreground">Retries</label>
              <p className="text-lg">{task.retry_count} / {task.max_retries}</p>
            </div>
          </div>

          {/* Callback URL */}
          <div>
            <label className="text-sm font-medium text-muted-foreground">Callback URL</label>
            <div className="flex items-center gap-2 mt-1">
              <code className="flex-1 text-xs bg-muted p-2 rounded break-all">
                {task.callback_url}
              </code>
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleCopy(task.callback_url)}
              >
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              </Button>
            </div>
          </div>

          {/* Timestamps */}
          <div className="space-y-2">
            <h3 className="text-sm font-medium">Timestamps</h3>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-muted-foreground">Created:</span>{' '}
                {formatDate(task.created_at)}
              </div>
              <div>
                <span className="text-muted-foreground">Scheduled:</span>{' '}
                {formatDate(task.scheduled_at)}
              </div>
              <div>
                <span className="text-muted-foreground">Started:</span>{' '}
                {formatDate(task.started_at)}
              </div>
              <div>
                <span className="text-muted-foreground">Completed:</span>{' '}
                {formatDate(task.completed_at)}
              </div>
            </div>
          </div>

          {/* Payload */}
          <div>
            <label className="text-sm font-medium text-muted-foreground">Payload</label>
            <pre className="mt-1 text-xs bg-muted p-4 rounded overflow-x-auto">
              {JSON.stringify(task.payload, null, 2)}
            </pre>
          </div>

          {/* Tags */}
          {task.tags && task.tags.length > 0 && (
            <div>
              <label className="text-sm font-medium text-muted-foreground">Tags</label>
              <div className="flex gap-2 mt-2 flex-wrap">
                {task.tags.map((tag) => (
                  <Badge key={tag} variant="secondary">{tag}</Badge>
                ))}
              </div>
            </div>
          )}

          {/* Error Message */}
          {task.error_message && (
            <div>
              <label className="text-sm font-medium text-destructive">Error Message</label>
              <p className="mt-1 text-sm bg-destructive/10 text-destructive p-3 rounded">
                {task.error_message}
              </p>
            </div>
          )}

          {/* Callback Attempts */}
          <div>
            <label className="text-sm font-medium text-muted-foreground">Callback Attempts</label>
            <p className="text-lg">{task.callback_attempts}</p>
          </div>
        </div>

        <DialogFooter>
          <Button onClick={() => onOpenChange(false)}>Close</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
