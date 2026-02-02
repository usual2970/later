import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { useStats } from '../hooks/useTasks';

export function TaskStats() {
  const { data: stats, isLoading } = useStats();

  if (isLoading) {
    return <div>Loading stats...</div>;
  }

  if (!stats) {
    return null;
  }

  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Total Tasks</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">{stats.total}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Completed (24h)</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">{stats.last_24h.completed}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Failed (24h)</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold text-destructive">{stats.last_24h.failed}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Success Rate</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">
            {(stats.callback_success_rate * 100).toFixed(1)}%
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
