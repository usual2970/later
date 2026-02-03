# Task Service Dashboard

React-based dashboard for monitoring and managing asynchronous tasks.

## Features

- 📊 Real-time task statistics
- 📋 Task list with filtering and pagination
- 🔔 Real-time WebSocket updates
- 🎨 Modern UI with shadcn/ui components
- 📱 Responsive design

## Quick Start

### Prerequisites

- Node.js 20.19+ or 22.12+
- Backend API running on port 7384

### Installation

```bash
# Install dependencies
npm install
```

### Configuration

Create a `.env` file (optional):

```bash
cp .env.example .env
```

The default configuration connects to `http://localhost:7384`. To use a different backend:

```bash
echo "VITE_API_BASE_URL=http://your-backend:7384" > .env
```

### Development

```bash
# Start development server
npm run dev

# Open http://localhost:5173
```

### Build

```bash
# Build for production
npm run build

# Preview production build
npm run preview
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_BASE_URL` | `http://localhost:7384` | Backend API base URL |

## WebSocket Connection

The dashboard automatically connects to the WebSocket endpoint for real-time updates:

- URL: `<VITE_API_BASE_URL>/api/v1/tasks/stream`
- Example: `ws://localhost:7384/api/v1/tasks/stream`

Connection status is shown in the header (🟢 Live / 🔴 Disconnected).

## Pages

- **Dashboard** (`/`) - Statistics and recent tasks
- **Tasks** (`/tasks`) - Full task list with filters
- **Task Detail** (`/tasks/:id`) - Individual task details
- **Dead Letter** (`/dead-letter`) - Failed tasks queue

## Troubleshooting

### WebSocket Connection Failed

1. **Verify backend is running**
   ```bash
   curl http://localhost:7384/health
   ```

2. **Check the .env file**
   ```bash
   cat .env
   # Should show correct VITE_API_BASE_URL
   ```

3. **Check browser console** (F12) for WebSocket errors

4. **Verify port**
   - Backend default: 7384 (see `../configs/config.yaml`)
   - Frontend default: 7384 (see `.env.example`)

### Build Errors

If you encounter build errors:

```bash
# Clear cache and rebuild
rm -rf node_modules dist
npm install
npm run build
```

### TypeScript Errors

```bash
# Check TypeScript
npm run check
```

## Technology Stack

- **Framework**: React + TypeScript
- **Build Tool**: Vite
- **UI Components**: shadcn/ui (Radix UI primitives)
- **Styling**: Tailwind CSS
- **State Management**: React Query
- **Routing**: React Router v7
- **Real-time**: WebSocket API

## Documentation

- [Environment Configuration](./docs/ENV_CONFIGURATION.md)
- [Project Plan](../docs/plans/2026-02-02-feat-async-debugging-service-plan.md)
