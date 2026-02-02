import { TaskStats } from './components/TaskStats';
import { TaskList } from './components/TaskList';
import { CreateTaskForm } from './components/CreateTaskForm';
import { WebSocketStatus } from './components/WebSocketStatus';

function App() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto py-8 px-4">
        <div className="mb-8 flex items-center justify-between">
          <div>
            <h1 className="text-4xl font-bold tracking-tight">Task Dashboard</h1>
            <p className="text-muted-foreground mt-2">
              Monitor and manage your asynchronous tasks
            </p>
          </div>
          <div className="flex items-center gap-4">
            <WebSocketStatus />
            <CreateTaskForm />
          </div>
        </div>

        <div className="space-y-8">
          <TaskStats />
          <TaskList />
        </div>
      </div>
    </div>
  );
}

export default App;
