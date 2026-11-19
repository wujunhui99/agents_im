import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

const proxyHost = process.env.AGENTS_IM_DEV_PROXY_HOST ?? '127.0.0.1';
const httpTarget = (portEnvName: string, fallbackPort: number) =>
  `http://${proxyHost}:${process.env[portEnvName] ?? String(fallbackPort)}`;
const wsTarget = (portEnvName: string, fallbackPort: number) => `ws://${proxyHost}:${process.env[portEnvName] ?? String(fallbackPort)}`;

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // 单一 HTTP 入口：原 8 个 *-api 已合并进 apigateway（默认 8080）。
      // /ws 仍走独立的 msggateway（WebSocket 长连接）。
      '/admin/dashboard': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/admin/llm-traces': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/admin/conversations': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/admin/users': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/admin/test-accounts': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/api/admin/feedback': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/api/admin/task-reports': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/api/feedback': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/auth': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/messages': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/conversations': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/me': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/users': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/accounts': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/media': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/friends': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/groups': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/agents': { target: httpTarget('GATEWAY_HTTP_PORT', 8080), changeOrigin: true },
      '/ws': { target: wsTarget('GATEWAY_WS_PORT', 8084), ws: true, changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './vitest.setup.ts',
  },
});
