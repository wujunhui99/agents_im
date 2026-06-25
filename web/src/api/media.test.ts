import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';
import { createMediaApi } from './media';

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

function fetchPath(input: RequestInfo | URL) {
  if (typeof input === 'string') {
    return input;
  }
  if (input instanceof URL) {
    return `${input.pathname}${input.search}`;
  }
  return input.url;
}

describe('media API adapter', () => {
  it('adds msg_id when requesting a message attachment download URL', async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      expect(fetchPath(input)).toBe('/media/med_1/download-url?msg_id=srv_1');
      return jsonResponse({
        code: 'OK',
        message: 'ok',
        data: { mediaId: 'med_1', downloadUrl: 'https://media.test/download/med_1', expiresAt: 1777465000000 },
      });
    });
    const api = createMediaApi(createApiClient({ fetchImpl }));

    await expect(api.getDownloadURL('med_1', { msgId: 'srv_1' })).resolves.toEqual({
      mediaId: 'med_1',
      downloadUrl: 'https://media.test/download/med_1',
      expiresAt: 1777465000000,
    });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });
});
