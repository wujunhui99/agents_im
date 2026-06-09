import { describe, expect, it } from 'vitest';
import viteConfig from './vite.config';

describe('Vite local backend proxy', () => {
  it('proxies every backend contract prefix to its local service port', () => {
    const proxy = viteConfig.server?.proxy ?? {};

    expect(proxy['/admin/dashboard']).toMatchObject({ target: 'http://127.0.0.1:8088' });
    expect(proxy['/admin/llm-traces']).toMatchObject({ target: 'http://127.0.0.1:8088' });
    expect(proxy['/admin/conversations']).toMatchObject({ target: 'http://127.0.0.1:8088' });
    expect(proxy['/admin/users']).toMatchObject({ target: 'http://127.0.0.1:8088' });
    expect(proxy['/api/admin/feedback']).toMatchObject({ target: 'http://127.0.0.1:8088' });
    expect(proxy['/api/admin/task-reports']).toMatchObject({ target: 'http://127.0.0.1:8088' });
    expect(proxy['/api/feedback']).toMatchObject({ target: 'http://127.0.0.1:8088' });
    expect(proxy['/auth']).toMatchObject({ target: 'http://127.0.0.1:8081' });
    expect(proxy['/me']).toMatchObject({ target: 'http://127.0.0.1:8080' });
    expect(proxy['/users']).toMatchObject({ target: 'http://127.0.0.1:8080' });
    expect(proxy['/media']).toMatchObject({ target: 'http://127.0.0.1:8089' });
    expect(proxy['/friends']).toMatchObject({ target: 'http://127.0.0.1:8082' });
    expect(proxy['/messages']).toMatchObject({ target: 'http://127.0.0.1:8083' });
    expect(proxy['/conversations']).toMatchObject({ target: 'http://127.0.0.1:8083' });
    expect(proxy['/groups']).toMatchObject({ target: 'http://127.0.0.1:8085' });
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
