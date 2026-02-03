// Centralized API configuration
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:7384';

// WebSocket URL derived from API_BASE_URL
export const WS_URL = API_BASE_URL
  .replace('http://', 'ws://')
  .replace('https://', 'wss://');

// Full WebSocket endpoint URL
export const WS_TASK_STREAM_URL = `${WS_URL}/api/v1/tasks/stream`;
