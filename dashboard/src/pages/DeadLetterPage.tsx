import { useTasks, useResurrectTask } from '../hooks/useTasks';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { StatusBadge } from '../components/StatusBadge';
import { AlertTriangle, RefreshCw } from 'lucide-react';

export function DeadLetterPage() {
  const { data, isLoading } = useTasks({
    status: 'dead_lettered',
    page: 1,
    limit: 50,
    sort_by: 'created_at',
    sort_order: 'desc',
  });
  const resurrectTask = useResurrectTask();

  const handleResurrect = (id: string, name: string) => {
    if (window.confirm(`Resurrect task "${name}"? This will reset it to pending status.`)) {
      resurrectTask.mutate(id);
    }
  };

  return (
    <div className="space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-4xl font-bold tracking-tight flex items-center gap-2">
          <AlertTriangle className="h-10 w-10 text-destructive" />
          Dead Letter Queue
        </h1>
        <p className="text-muted-foreground mt-2">
          Tasks that have exhausted all retry attempts and require manual intervention
        </p>
      </div>

      {/* Info Card */}
      <Card className="border-destructive bg-destructive/5">
        <CardHeader>
          <CardTitle className="text-destructive">What is the Dead Letter Queue?</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <p>
            Tasks in this queue have failed after exhausting their maximum retry attempts.
            This typically indicates persistent issues such as:
          </p>
          <ul className="list-disc list-inside space-y-1 text-muted-foreground ml-4">
            <li>Callback endpoint is down or unreachable</li>
            <li>Callback endpoint returning errors (4xx/5xx)</li>
            <li>Network connectivity issues</li>
            <li>Invalid or rejected payload data</li>
          </ul>
          <p className="pt-2">
            You can review the error details and choose to resurrect tasks for retry
            or investigate the underlying issue.
          </p>
        </CardContent>
      </Card>

      {/* Dead Lettered Tasks */}
      {isLoading ? (
        <div className="flex items-center justify-center p-8">
          <div className="text-lg">Loading dead-lettered tasks...</div>
        </div>
      ) : !data || data.tasks.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <AlertTriangle className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No Dead-Lettered Tasks</h3>
            <p className="text-muted-foreground text-center">
              Great! No tasks have exhausted their retry attempts.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-2xl font-bold">
              {data.pagination.total} Dead-Lettered Task{data.pagination.total !== 1 ? 's' : ''}
            </h2>
          </div>

          <div className="grid gap-4">
            {data.tasks.map((task) => (
              <Card key={task.id} className="border-destructive/50">
                <CardHeader>
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <CardTitle className="text-lg">{task.name}</CardTitle>
                        <StatusBadge status={task.status} />
                      </div>
                      <CardDescription className="font-mono text-xs">
                        {task.id}
                      </CardDescription>
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleResurrect(task.id, task.name)}
                      disabled={resurrectTask.isPending}
                      className="gap-2"
                    >
                      <RefreshCw className="h-4 w-4" />
                      Resurrect
                    </Button>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  {/* Error Message */}
                  {task.error_message && (
                    <div className="bg-destructive/10 text-destructive p-3 rounded text-sm">
                      <strong>Error:</strong> {task.error_message}
                    </div>
                  )}

                  {/* Task Details Grid */}
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                    <div>
                      <label className="text-muted-foreground">Priority</label>
                      <p className="font-semibold">{task.priority}</p>
                    </div>
                    <div>
                      <label className="text-muted-foreground">Retries</label>
                      <p className="font-semibold">
                        {task.retry_count} / {task.max_retries}
                      </p>
                    </div>
                    <div>
                      <label className="text-muted-foreground">Callback Attempts</label>
                      <p className="font-semibold">{task.callback_attempts}</p>
                    </div>
                    <div>
                      <label className="text-muted-foreground">Created</label>
                      <p className="font-semibold">
                        {new Date(task.created_at).toLocaleDateString()}
                      </p>
                    </div>
                  </div>

                  {/* Callback URL */}
                  <div>
                    <label className="text-sm font-medium text-muted-foreground">Callback URL</label>
                    <code className="block text-xs bg-muted p-2 rounded mt-1 break-all">
                      {task.callback_url}
                    </code>
                  </div>

                  {/* Tags */}
                  {task.tags && task.tags.length > 0 && (
                    <div className="flex gap-2 flex-wrap">
                      {task.tags.map((tag) => (
                        <Badge key={tag} variant="secondary">{tag}</Badge>
                      ))}
                    </div>
                  )}

                  {/* Payload Toggle */}
                  <details className="text-sm">
                    <summary className="cursor-pointer text-muted-foreground hover:text-foreground">
                      View Payload
                    </summary>
                    <pre className="mt-2 text-xs bg-muted p-3 rounded overflow-x-auto">
                      {JSON.stringify(task.payload, null, 2)}
                    </pre>
                  </details>
                </CardContent>
              </Card>
            ))}
          </div>

          {/* Pagination Info */}
          {data.pagination.total_pages > 1 && (
            <div className="text-center text-sm text-muted-foreground">
              Showing {data.tasks.length} of {data.pagination.total} dead-lettered tasks
            </div>
          )}
        </div>
      )}
    </div>
  );
}
