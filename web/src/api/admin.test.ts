import { describe, expect, it, vi } from 'vitest';
import { createAdminApi } from './admin';
import { createApiClient } from './client';

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

describe('admin API adapter', () => {
  it('fetches feedback from the non-conflicting JSON API route', async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      expect(fetchPath(input)).toBe('/api/admin/feedback?status=new&limit=25');
      return jsonResponse({
        code: 'OK',
        message: 'ok',
        data: { items: [] },
      });
    });
    const api = createAdminApi(createApiClient({ fetchImpl }));

    await expect(api.listFeedback({ status: 'new', limit: 25 })).resolves.toEqual({ items: [] });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('fetches task reports from the task-management JSON API route', async () => {
    const fetchImpl = vi.fn(async (input: RequestInfo | URL) => {
      expect(fetchPath(input)).toBe('/api/admin/task-reports?outcome=success&limit=100');
      return jsonResponse({
        code: 'OK',
        message: 'ok',
        data: { items: [] },
      });
    });
    const api = createAdminApi(createApiClient({ fetchImpl }));

    await expect(api.listTaskReports({ outcome: 'success', limit: 100 })).resolves.toEqual({ items: [] });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });
});
