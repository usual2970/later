import { BrowserRouter, Routes, Route, Link, useLocation } from 'react-router-dom';
import { Toaster } from './components/ui/toaster';
import { DashboardPage } from './pages/DashboardPage';
import { TasksPage } from './pages/TasksPage';
import { TaskDetailPage } from './pages/TaskDetailPage';
import { DeadLetterPage } from './pages/DeadLetterPage';
import { CreateTaskForm } from './components/CreateTaskForm';

function Navigation() {
  const location = useLocation();
  const isActive = (path: string) => location.pathname === path;

  return (
    <nav className="border-b">
      <div className="container mx-auto px-4">
        <div className="flex items-center justify-between h-16">
          <div className="flex items-center gap-6">
            <Link to="/" className="text-xl font-bold">
              Later
            </Link>
            <div className="flex gap-4">
              <Link
                to="/"
                className={`text-sm font-medium transition-colors hover:text-primary ${
                  isActive('/') ? 'text-foreground' : 'text-muted-foreground'
                }`}
              >
                Dashboard
              </Link>
              <Link
                to="/tasks"
                className={`text-sm font-medium transition-colors hover:text-primary ${
                  isActive('/tasks') || isActive('/tasks/') ? 'text-foreground' : 'text-muted-foreground'
                }`}
              >
                Tasks
              </Link>
              <Link
                to="/dead-letter"
                className={`text-sm font-medium transition-colors hover:text-primary ${
                  isActive('/dead-letter') ? 'text-foreground' : 'text-muted-foreground'
                }`}
              >
                Dead Letter
              </Link>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <CreateTaskForm />
          </div>
        </div>
      </div>
    </nav>
  );
}

function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen bg-background">
        <Navigation />
        <main className="container mx-auto py-8 px-4">
          <Routes>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/tasks" element={<TasksPage />} />
            <Route path="/tasks/:id" element={<TaskDetailPage />} />
            <Route path="/dead-letter" element={<DeadLetterPage />} />
          </Routes>
        </main>
      </div>
      <Toaster />
    </BrowserRouter>
  );
}

export default App;
