import { useParams, Link } from 'react-router-dom';
import { useTask } from '../hooks/useTasks';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { StatusBadge } from '../components/StatusBadge';
import { ArrowLeft, Copy, Check } from 'lucide-react';
import { useState } from 'react';
import { useCancelTask, useRetryTask, useResurrectTask } from '../hooks/useTasks';

export function TaskDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: task, isLoading, error } = useTask(id!);
  const [copied, setCopied] = useState(false);
  const cancelTask = useCancelTask();
  const retryTask = useRetryTask();
  const resurrectTask = useResurrectTask();

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleCancel = () => {
    if (window.confirm('Are you sure you want to cancel this task?')) {
      cancelTask.mutate(id!);
    }
  };

  const handleRetry = () => {
    retryTask.mutate(id!);
  };

  const handleResurrect = () => {
    resurrectTask.mutate(id!);
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString();
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-lg">Loading task details...</div>
      </div>
    );
  }

  if (error || !task) {
    return (
      <div className="space-y-4">
        <Link to="/tasks">
          <Button variant="ghost" size="sm">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Tasks
          </Button>
        </Link>
        <div className="text-destructive">Error loading task: {error?.message || 'Task not found'}</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header with Back Button */}
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <Link to="/tasks">
            <Button variant="ghost" size="sm">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to Tasks
            </Button>
          </Link>
          <div className="flex items-center gap-4 mt-2">
            <h1 className="text-4xl font-bold tracking-tight">{task.name}</h1>
            <StatusBadge status={task.status} />
          </div>
          <p className="text-muted-foreground text-sm font-mono">{task.id}</p>
        </div>

        {/* Action Buttons */}
        <div className="flex gap-2">
          {task.status === 'pending' && (
            <Button variant="destructive" onClick={handleCancel} disabled={cancelTask.isPending}>
              Cancel Task
            </Button>
          )}
          {task.status === 'failed' && (
            <Button variant="outline" onClick={handleRetry} disabled={retryTask.isPending}>
              Retry Task
            </Button>
          )}
          {task.status === 'dead_lettered' && (
            <Button variant="outline" onClick={handleResurrect} disabled={resurrectTask.isPending}>
              Resurrect Task
            </Button>
          )}
        </div>
      </div>

      {/* Task Details Grid */}
      <div className="grid gap-6 md:grid-cols-2">
        {/* Basic Info Card */}
        <Card>
          <CardHeader>
            <CardTitle>Basic Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <label className="text-sm font-medium text-muted-foreground">Task ID</label>
              <div className="flex items-center gap-2 mt-1">
                <code className="flex-1 text-xs bg-muted p-2 rounded">{task.id}</code>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleCopy(task.id)}
                >
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>

            <div>
              <label className="text-sm font-medium text-muted-foreground">Status</label>
              <div className="mt-1">
                <StatusBadge status={task.status} />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium text-muted-foreground">Priority</label>
                <p className="text-lg font-semibold">{task.priority}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">Retries</label>
                <p className="text-lg font-semibold">
                  {task.retry_count} / {task.max_retries}
                </p>
              </div>
            </div>

            <div>
              <label className="text-sm font-medium text-muted-foreground">Callback Attempts</label>
              <p className="text-lg font-semibold">{task.callback_attempts}</p>
            </div>
          </CardContent>
        </Card>

        {/* Timestamps Card */}
        <Card>
          <CardHeader>
            <CardTitle>Timestamps</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div>
              <label className="text-sm font-medium text-muted-foreground">Created At</label>
              <p className="text-sm">{formatDate(task.created_at)}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-muted-foreground">Scheduled For</label>
              <p className="text-sm">{formatDate(task.scheduled_at)}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-muted-foreground">Started At</label>
              <p className="text-sm">{formatDate(task.started_at)}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-muted-foreground">Completed At</label>
              <p className="text-sm">{formatDate(task.completed_at)}</p>
            </div>

            {/* Duration calculation */}
            {task.started_at && task.completed_at && (
              <div className="pt-2 border-t">
                <label className="text-sm font-medium text-muted-foreground">Execution Duration</label>
                <p className="text-sm font-semibold">
                  {new Date(task.completed_at).getTime() - new Date(task.started_at).getTime()}ms
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Callback URL Card */}
      <Card>
        <CardHeader>
          <CardTitle>Callback Configuration</CardTitle>
        </CardHeader>
        <CardContent>
          <div>
            <label className="text-sm font-medium text-muted-foreground">Callback URL</label>
            <div className="flex items-center gap-2 mt-1">
              <code className="flex-1 text-xs bg-muted p-3 rounded break-all">
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
        </CardContent>
      </Card>

      {/* Payload Card */}
      <Card>
        <CardHeader>
          <CardTitle>Task Payload</CardTitle>
          <CardDescription>Original task payload data</CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="text-xs bg-muted p-4 rounded overflow-x-auto">
            {JSON.stringify(task.payload, null, 2)}
          </pre>
        </CardContent>
      </Card>

      {/* Tags Card */}
      {task.tags && task.tags.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Tags</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex gap-2 flex-wrap">
              {task.tags.map((tag) => (
                <Badge key={tag} variant="secondary">{tag}</Badge>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Error Message Card */}
      {task.error_message && (
        <Card className="border-destructive">
          <CardHeader>
            <CardTitle className="text-destructive">Error Information</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="bg-destructive/10 text-destructive p-4 rounded">
              <p className="text-sm">{task.error_message}</p>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
