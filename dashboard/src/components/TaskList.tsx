import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from './ui/table';
import { Button } from './ui/button';
import { StatusBadge } from './StatusBadge';
import { useTasks, useCancelTask, useRetryTask, useResurrectTask } from '../hooks/useTasks';
import { useState } from 'react';
import { TaskDetail } from './TaskDetail';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog';
import type { TaskListParams, TaskStatus, Task } from '../lib/api';

export function TaskList() {
  const [params, setParams] = useState<TaskListParams>({
    page: 1,
    limit: 20,
    sort_by: 'created_at',
    sort_order: 'desc',
  });
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [taskToDelete, setTaskToDelete] = useState<Task | null>(null);

  const { data, isLoading } = useTasks(params);
  const cancelTask = useCancelTask();
  const retryTask = useRetryTask();
  const resurrectTask = useResurrectTask();

  const handleDeleteClick = (task: Task) => {
    setTaskToDelete(task);
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = () => {
    if (taskToDelete) {
      cancelTask.mutate(taskToDelete.id);
      setDeleteDialogOpen(false);
      setTaskToDelete(null);
    }
  };

  const handleRetry = (id: string) => {
    retryTask.mutate(id);
  };

  const handleResurrect = (id: string) => {
    resurrectTask.mutate(id);
  };

  const handleStatusFilter = (status?: TaskStatus) => {
    setParams({ ...params, status, page: 1 });
  };

  const handleTaskClick = (task: Task) => {
    setSelectedTask(task);
    setDetailOpen(true);
  };

  if (isLoading) {
    return <div className="flex items-center justify-center p-8">Loading tasks...</div>;
  }

  return (
    <div className="space-y-4">
      {/* Status Filters */}
      <div className="flex gap-2 flex-wrap">
        <Button
          variant={!params.status ? 'default' : 'outline'}
          size="sm"
          onClick={() => handleStatusFilter(undefined)}
        >
          All
        </Button>
        <Button
          variant={params.status === 'pending' ? 'default' : 'outline'}
          size="sm"
          onClick={() => handleStatusFilter('pending')}
        >
          Pending
        </Button>
        <Button
          variant={params.status === 'processing' ? 'default' : 'outline'}
          size="sm"
          onClick={() => handleStatusFilter('processing')}
        >
          Processing
        </Button>
        <Button
          variant={params.status === 'completed' ? 'default' : 'outline'}
          size="sm"
          onClick={() => handleStatusFilter('completed')}
        >
          Completed
        </Button>
        <Button
          variant={params.status === 'failed' ? 'default' : 'outline'}
          size="sm"
          onClick={() => handleStatusFilter('failed')}
        >
          Failed
        </Button>
        <Button
          variant={params.status === 'dead_lettered' ? 'default' : 'outline'}
          size="sm"
          onClick={() => handleStatusFilter('dead_lettered')}
        >
          Dead Lettered
        </Button>
      </div>

      {/* Tasks Table */}
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[100px]">ID</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Priority</TableHead>
              <TableHead>Scheduled</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Retries</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data?.tasks.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="text-center py-8 text-muted-foreground">
                  No tasks found
                </TableCell>
              </TableRow>
            ) : (
              data?.tasks.map((task) => (
                <TableRow key={task.id} className="cursor-pointer hover:bg-muted/50">
                  <TableCell className="font-mono text-xs">
                    {task.id.slice(0, 8)}
                  </TableCell>
                  <TableCell className="font-medium" onClick={() => handleTaskClick(task)}>
                    {task.name}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={task.status} />
                  </TableCell>
                  <TableCell>{task.priority}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(task.scheduled_at).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(task.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    {task.retry_count} / {task.max_retries}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      {(task.status === 'pending' || task.status === 'failed') && (
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => handleDeleteClick(task)}
                          disabled={cancelTask.isPending}
                        >
                          Delete
                        </Button>
                      )}
                      {task.status === 'failed' && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleRetry(task.id)}
                          disabled={retryTask.isPending}
                        >
                          Retry
                        </Button>
                      )}
                      {task.status === 'dead_lettered' && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleResurrect(task.id)}
                          disabled={resurrectTask.isPending}
                        >
                          Resurrect
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {data && data.pagination.total_pages > 1 && (
        <div className="flex items-center justify-between px-2">
          <div className="text-sm text-muted-foreground">
            Showing {((params.page! - 1) * params.limit!) + 1} to{' '}
            {Math.min(params.page! * params.limit!, data.pagination.total)} of{' '}
            {data.pagination.total} tasks
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setParams({ ...params, page: params.page! - 1 })}
              disabled={params.page! <= 1}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setParams({ ...params, page: params.page! + 1 })}
              disabled={params.page! >= data.pagination.total_pages}
            >
              Next
            </Button>
          </div>
        </div>
      )}

      {/* Task Detail Dialog */}
      <TaskDetail
        task={selectedTask}
        open={detailOpen}
        onOpenChange={setDetailOpen}
      />

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Task</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete task "{taskToDelete?.name}"? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setDeleteDialogOpen(false);
                setTaskToDelete(null);
              }}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteConfirm}
              disabled={cancelTask.isPending}
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
