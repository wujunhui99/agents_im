import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';
import { createFeedbackApi } from './feedback';

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

describe('feedback API adapter', () => {
  it('submits feedback through a non-SPA JSON API route', async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      expect(fetchPath(input)).toBe('/api/feedback');
      return jsonResponse({
        code: 'OK',
        message: 'ok',
        data: { feedbackId: 'fb_1', status: 'new' },
      });
    });
    const api = createFeedbackApi(createApiClient({ fetchImpl }));

    await expect(
      api.submitFeedback({ category: 'bug', title: 'feedback 405', content: 'submit should not post to SPA route' }),
    ).resolves.toEqual({ feedbackId: 'fb_1', status: 'new' });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });
});
