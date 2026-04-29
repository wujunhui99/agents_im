import { createApiClient } from './client';

export type ApiClientOptions = Parameters<typeof createApiClient>[0];

export function requestEnvelope<TData>(options: ApiClientOptions, path: string, init: RequestInit = {}): Promise<TData> {
  const api = createApiClient(options);
  const method = (init.method ?? 'GET').toUpperCase();
  const body = typeof init.body === 'string' ? JSON.parse(init.body) : init.body;

  if (method === 'POST') {
    return api.post<TData>(path, body);
  }
  if (method === 'PATCH') {
    return api.patch<TData>(path, body);
  }
  if (method === 'DELETE') {
    return api.delete<TData>(path);
  }

  return api.get<TData>(path);
}
