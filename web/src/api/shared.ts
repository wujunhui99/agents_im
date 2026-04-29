export type ApiEnvelope<TData> = {
  code: string;
  message: string;
  data: TData;
};

export type ApiClientOptions = {
  baseUrl?: string;
  token?: string;
  fetcher?: typeof fetch;
};

export async function requestEnvelope<TData>(
  options: ApiClientOptions,
  path: string,
  init: RequestInit = {},
): Promise<TData> {
  const fetcher = options.fetcher ?? fetch;
  const headers = new Headers(init.headers);

  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  if (options.token) {
    headers.set('Authorization', `Bearer ${options.token}`);
  }

  const response = await fetcher(buildUrl(options.baseUrl ?? '', path), {
    ...init,
    headers,
  });

  if (!response.ok) {
    throw new Error(`Request failed with status ${response.status}`);
  }

  const envelope = (await response.json()) as ApiEnvelope<TData>;
  if (envelope.code !== 'OK') {
    throw new Error(envelope.message || envelope.code);
  }

  return envelope.data;
}

export function buildUrl(baseUrl: string, path: string) {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;

  if (!baseUrl) {
    return normalizedPath;
  }

  return `${baseUrl.replace(/\/+$/, '')}${normalizedPath}`;
}
