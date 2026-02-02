import { TaskStats } from './components/TaskStats';
import { TaskList } from './components/TaskList';

function App() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto py-8 px-4">
        <div className="mb-8">
          <h1 className="text-4xl font-bold tracking-tight">Task Dashboard</h1>
          <p className="text-muted-foreground mt-2">
            Monitor and manage your asynchronous tasks
          </p>
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
