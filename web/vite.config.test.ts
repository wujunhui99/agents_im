import { describe, expect, it } from 'vitest';
import viteConfig from './vite.config';

describe('Vite local backend proxy', () => {
  it('proxies every backend contract prefix to the single gateway', () => {
    const proxy = viteConfig.server?.proxy ?? {};

    // 单一 HTTP 入口：所有域前缀都指向 apigateway（默认 8080）。
    for (const prefix of [
      '/admin/dashboard',
      '/admin/llm-traces',
      '/admin/conversations',
      '/admin/users',
      '/admin/test-accounts',
      '/api/admin/feedback',
      '/api/admin/task-reports',
      '/api/feedback',
      '/auth',
      '/me',
      '/users',
      '/accounts',
      '/media',
      '/friends',
      '/messages',
      '/conversations',
      '/groups',
      '/agents',
    ]) {
      expect(proxy[prefix]).toMatchObject({ target: 'http://127.0.0.1:8080' });
    }
    expect(proxy['/ws']).toMatchObject({ target: 'ws://127.0.0.1:8084', ws: true });
  });

  it('routes /messages before the shorter /me prefix', () => {
    const proxyPrefixes = Object.keys(viteConfig.server?.proxy ?? {});

    expect(proxyPrefixes.indexOf('/messages')).toBeGreaterThanOrEqual(0);
    expect(proxyPrefixes.indexOf('/me')).toBeGreaterThanOrEqual(0);
    expect(proxyPrefixes.indexOf('/messages')).toBeLessThan(proxyPrefixes.indexOf('/me'));
    expect(proxyPrefixes).not.toContain('/admin');
  });
});
