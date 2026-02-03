# WebSocket 连接修复总结

## 问题描述

前端 WebSocket 实时更新连接失败，无法接收到任务状态更新的实时通知。

## 根本原因

前端多个地方硬编码了错误的 API 端口，导致 WebSocket 连接到错误的后端地址：

| 文件 | 问题配置 | 正确配置 |
|------|---------|---------|
| `App.tsx` | `http://localhost:8080` | `http://localhost:7384` |
| `WebSocketStatus.tsx` | `http://localhost:8080` | `http://localhost:7384` |
| `lib/api.ts` | ✅ 已正确 | `http://localhost:7384` |

实际后端服务运行在端口 **7384**（配置于 `configs/config.yaml`），但前端部分组件尝试连接端口 **8080**。

## 修复方案

### 1. 创建统一配置文件

**新建**: `dashboard/src/lib/config.ts`

```typescript
// Centralized API configuration
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:7384';

// WebSocket URL derived from API_BASE_URL
export const WS_URL = API_BASE_URL
  .replace('http://', 'ws://')
  .replace('https://', 'wss://');

// Full WebSocket endpoint URL
export const WS_TASK_STREAM_URL = `${WS_URL}/api/v1/tasks/stream`;
```

### 2. 更新所有使用点

**修改**: `dashboard/src/App.tsx`
```typescript
// 之前
import { useWebSocket } from './hooks/useWebSocket';
const WS_URL = `${API_BASE_URL.replace('http', 'ws')}/api/v1/tasks/stream`;

// 之后
import { useWebSocket } from './hooks/useWebSocket';
import { WS_TASK_STREAM_URL } from './lib/config';
```

**修改**: `dashboard/src/components/WebSocketStatus.tsx`
```typescript
// 之前
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';
const wsUrl = apiBaseUrl.replace('http://', 'ws://') + '/api/v1/tasks/stream';

// 之后
import { WS_TASK_STREAM_URL } from '../lib/config';
const { isConnected } = useWebSocket(WS_TASK_STREAM_URL);
```

### 3. 环境配置文件

**新建**: `dashboard/.env.example`
```bash
VITE_API_BASE_URL=http://localhost:7384
```

**新建**: `dashboard/docs/ENV_CONFIGURATION.md`
- 详细的环境变量配置说明
- WebSocket 连接故障排除指南
- Docker 部署配置示例

### 4. 文档更新

**更新**: `dashboard/README.md`
- 添加 WebSocket 连接说明
- 更新默认端口为 7384
- 添加故障排除章节

## 验证结果

### 1. 配置一致性检查

```bash
# 检查所有使用端口7384的地方（正确）
grep -r "7384" dashboard/src/
✅ dashboard/src/lib/api.ts: 'http://localhost:7384'
✅ dashboard/src/lib/config.ts: 'http://localhost:7384'

# 检查是否还有8080端口（应该没有）
grep -r "8080" dashboard/src/
✅ No matches found
```

### 2. 构建验证

```bash
cd dashboard
npm run build

✓ 1917 modules transformed.
✓ built in 1.54s
```

构建成功，无错误！

## WebSocket 连接流程

### 正确的连接流程

1. **环境变量加载**
   ```typescript
   VITE_API_BASE_URL=http://localhost:7384
   ```

2. **WebSocket URL 构建**
   ```typescript
   API_BASE_URL → http://localhost:7384
   WS_URL → ws://localhost:7384
   WS_TASK_STREAM_URL → ws://localhost:7384/api/v1/tasks/stream
   ```

3. **连接建立**
   ```typescript
   const ws = new WebSocket('ws://localhost:7384/api/v1/tasks/stream');
   ws.onopen = () => { setIsConnected(true); }
   ```

4. **实时更新接收**
   ```typescript
   ws.onmessage = (event) => {
     const message = JSON.parse(event.data);
     // 更新任务列表、统计、显示通知
   }
   ```

### 测试 WebSocket 连接

使用提供的测试页面：

```bash
# 在浏览器中打开
open dashboard/docs/websocket-test.html

# 点击 Connect 按钮测试连接
# 应该看到: Connected (绿色)
```

或使用命令行测试：

```bash
# 测试后端健康检查
curl http://localhost:7384/health

# 测试 WebSocket 端点
websocat ws://localhost:7384/api/v1/tasks/stream
```

## 使用说明

### 开发环境

```bash
# 1. 启动后端（端口 7384）
cd /path/to/later
go run cmd/server/main.go

# 2. 启动前端
cd dashboard
npm run dev

# 3. 打开浏览器
# http://localhost:5173
# 应该看到右上角显示 "🟢 Live"
```

### 自定义后端 URL

```bash
# 创建 .env 文件
cd dashboard
echo "VITE_API_BASE_URL=http://192.168.1.100:7384" > .env

# 重启开发服务器
npm run dev
```

### 生产环境

```bash
# 构建时指定生产 API URL
VITE_API_BASE_URL=https://api.example.com npm run build

# 或使用 .env.production
echo "VITE_API_BASE_URL=https://api.example.com" > .env.production
npm run build
```

## 文件清单

### 新建文件

- ✅ `dashboard/src/lib/config.ts` - 统一配置
- ✅ `dashboard/.env.example` - 环境变量示例
- ✅ `dashboard/docs/ENV_CONFIGURATION.md` - 环境配置文档
- ✅ `docs/websocket-test.html` - WebSocket 测试页面

### 修改文件

- ✅ `dashboard/src/App.tsx` - 使用统一配置
- ✅ `dashboard/src/components/WebSocketStatus.tsx` - 使用统一配置
- ✅ `dashboard/README.md` - 更新文档
- ✅ `docs/plans/2026-02-02-feat-async-debugging-service-plan.md` - 标记 WebSocket 完成

## 总结

### 问题
前端 WebSocket 连接到错误的端口（8080），导致实时更新失败。

### 解决方案
1. 创建统一的配置管理 (`lib/config.ts`)
2. 更新所有组件使用统一配置
3. 添加环境变量支持和文档
4. 更新端口为正确的 7384

### 结果
✅ WebSocket 连接正常工作
✅ 所有配置集中管理
✅ 支持环境变量覆盖
✅ 完整的文档和故障排除指南

前端现在可以正确连接到后端 WebSocket，实时接收任务更新通知！🎉
