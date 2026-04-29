import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/auth': { target: 'http://127.0.0.1:8081', changeOrigin: true },
      '/me': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/users': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/friends': { target: 'http://127.0.0.1:8082', changeOrigin: true },
      '/messages': { target: 'http://127.0.0.1:8083', changeOrigin: true },
      '/conversations': { target: 'http://127.0.0.1:8083', changeOrigin: true },
      '/groups': { target: 'http://127.0.0.1:8085', changeOrigin: true },
      '/ws': { target: 'ws://127.0.0.1:8084', ws: true, changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './vitest.setup.ts',
  },
});
