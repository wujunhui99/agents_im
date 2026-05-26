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
      '/admin/dashboard': { target: httpTarget('MESSAGE_API_PORT', 8083), changeOrigin: true },
      '/admin/llm-traces': { target: httpTarget('MESSAGE_API_PORT', 8083), changeOrigin: true },
      '/admin/conversations': { target: httpTarget('MESSAGE_API_PORT', 8083), changeOrigin: true },
      '/admin/users': { target: httpTarget('MESSAGE_API_PORT', 8083), changeOrigin: true },
      '/api/admin/feedback': { target: httpTarget('MESSAGE_API_PORT', 8083), changeOrigin: true },
      '/api/admin/task-reports': { target: httpTarget('MESSAGE_API_PORT', 8083), changeOrigin: true },
      '/auth': { target: httpTarget('AUTH_API_PORT', 8081), changeOrigin: true },
      '/messages': { target: httpTarget('MESSAGE_API_PORT', 8083), changeOrigin: true },
      '/conversations': { target: httpTarget('MESSAGE_API_PORT', 8083), changeOrigin: true },
      '/me': { target: httpTarget('USER_API_PORT', 8080), changeOrigin: true },
      '/users': { target: httpTarget('USER_API_PORT', 8080), changeOrigin: true },
      '/media': { target: httpTarget('USER_API_PORT', 8080), changeOrigin: true },
      '/friends': { target: httpTarget('FRIENDS_API_PORT', 8082), changeOrigin: true },
      '/groups': { target: httpTarget('GROUPS_API_PORT', 8085), changeOrigin: true },
      '/ws': { target: wsTarget('GATEWAY_WS_PORT', 8084), ws: true, changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './vitest.setup.ts',
  },
});
