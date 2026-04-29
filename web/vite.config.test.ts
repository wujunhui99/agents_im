import { describe, expect, it } from 'vitest';
import viteConfig from './vite.config';

describe('Vite local backend proxy', () => {
  it('proxies every backend contract prefix to its local service port', () => {
    const proxy = viteConfig.server?.proxy ?? {};

    expect(proxy['/auth']).toMatchObject({ target: 'http://127.0.0.1:8081' });
    expect(proxy['/me']).toMatchObject({ target: 'http://127.0.0.1:8080' });
    expect(proxy['/users']).toMatchObject({ target: 'http://127.0.0.1:8080' });
    expect(proxy['/friends']).toMatchObject({ target: 'http://127.0.0.1:8082' });
    expect(proxy['/messages']).toMatchObject({ target: 'http://127.0.0.1:8083' });
    expect(proxy['/conversations']).toMatchObject({ target: 'http://127.0.0.1:8083' });
    expect(proxy['/groups']).toMatchObject({ target: 'http://127.0.0.1:8085' });
    expect(proxy['/ws']).toMatchObject({ target: 'ws://127.0.0.1:8084', ws: true });
  });
});
