import { useWebSocket } from '../hooks/useWebSocket';

export function WebSocketStatus() {
  // WebSocket URL - same as API URL but with ws:// or wss:// protocol
  const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';
  const wsUrl = apiBaseUrl.replace('http://', 'ws://').replace('https://', 'wss://') + '/api/v1/tasks/stream';

  const { isConnected } = useWebSocket(wsUrl);

  return (
    <div className="flex items-center gap-2">
      <div className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-red-500'}`} />
      <span className="text-sm text-muted-foreground">
        {isConnected ? 'Live' : 'Disconnected'}
      </span>
    </div>
  );
}
