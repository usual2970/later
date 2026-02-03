# Environment Configuration

## Backend API URL

The dashboard needs to connect to the backend API. Configure the API URL using the `VITE_API_BASE_URL` environment variable.

### Development

By default, the dashboard connects to `http://localhost:7384`.

To use a different backend URL:

```bash
# Create .env file
cp .env.example .env

# Edit the URL
echo "VITE_API_BASE_URL=http://localhost:7384" > .env

# Start the dev server
npm run dev
```

### Production

For production deployment, set the environment variable before building:

```bash
# Build with production API URL
VITE_API_BASE_URL=https://api.example.com npm run build

# Or create .env.production
echo "VITE_API_BASE_URL=https://api.example.com" > .env.production
npm run build
```

### Docker

When using Docker Compose, pass the environment variable:

```yaml
version: '3.8'
services:
  dashboard:
    build: ./dashboard
    environment:
      - VITE_API_BASE_URL=http://server:7384
    ports:
      - "5173:5173"
```

## WebSocket Connection

The WebSocket endpoint is automatically derived from `VITE_API_BASE_URL`:
- `http://localhost:7384` → `ws://localhost:7384/api/v1/tasks/stream`
- `https://api.example.com` → `wss://api.example.com/api/v1/tasks/stream`

No separate WebSocket configuration is needed.

## Available Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_BASE_URL` | `http://localhost:7384` | Backend API base URL |

## Troubleshooting

### WebSocket Connection Failed

If you see "Disconnected" status:

1. **Check backend is running**
   ```bash
   curl http://localhost:7384/health
   ```

2. **Verify API URL in .env**
   ```bash
   cat .env
   # Should show: VITE_API_BASE_URL=http://localhost:7384
   ```

3. **Check browser console for errors**
   - Open DevTools (F12)
   - Look for WebSocket connection errors

4. **Ensure correct port**
   - Backend default: 7384 (check `configs/config.yaml`)
   - If using different port, update `.env`

### CORS Errors

If you see CORS errors:

1. Check backend CORS configuration allows your frontend origin
2. Ensure backend is running with proper CORS middleware
3. Verify no firewall is blocking the connection
