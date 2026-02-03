import { useWebSocketContext } from '../contexts/WebSocketContext';

export function WebSocketStatus() {
  const { isConnected } = useWebSocketContext();

  return (
    <div className="flex items-center gap-2">
      <div className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-red-500'}`} />
      <span className="text-sm text-muted-foreground">
        {isConnected ? 'Live' : 'Disconnected'}
      </span>
    </div>
  );
}
