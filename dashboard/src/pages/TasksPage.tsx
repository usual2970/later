import { TaskList } from '../components/TaskList';

export function TasksPage() {
  return (
    <div className="space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-4xl font-bold tracking-tight">Tasks</h1>
        <p className="text-muted-foreground mt-2">
          View and manage all tasks with advanced filtering and pagination
        </p>
      </div>

      {/* Task List with Filters */}
      <TaskList />
    </div>
  );
}
