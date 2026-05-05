export type ApiEnvelope<T> = {
  code: string;
  message: string;
  data: T;
};

export type ApiClientOptions = {
  baseUrl?: string;
  getToken?: () => string | null | undefined;
  fetchImpl?: typeof fetch;
};

export type ApiRequestOptions = {
  method?: string;
  body?: unknown;
  headers?: HeadersInit;
  signal?: AbortSignal;
  auth?: boolean;
  token?: string | null;
};

export class ApiError extends Error {
  code: string;
  status: number;
  data: unknown;

  constructor({ code, message, status, data }: { code: string; message: string; status: number; data: unknown }) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.data = data;
  }
}

export type ApiClient = {
  request<T>(path: string, options?: ApiRequestOptions): Promise<T>;
  get<T>(path: string, options?: Omit<ApiRequestOptions, 'method' | 'body'>): Promise<T>;
  post<T>(path: string, body?: unknown, options?: Omit<ApiRequestOptions, 'method' | 'body'>): Promise<T>;
  put<T>(path: string, body?: unknown, options?: Omit<ApiRequestOptions, 'method' | 'body'>): Promise<T>;
  patch<T>(path: string, body?: unknown, options?: Omit<ApiRequestOptions, 'method' | 'body'>): Promise<T>;
  delete<T>(path: string, options?: Omit<ApiRequestOptions, 'method' | 'body'>): Promise<T>;
};

export function createApiClient(options: ApiClientOptions = {}): ApiClient {
  const baseUrl = normalizeBaseUrl(options.baseUrl ?? import.meta.env.VITE_API_BASE_URL ?? '');
  const fetchImpl = options.fetchImpl ?? fetch;

  async function request<T>(path: string, requestOptions: ApiRequestOptions = {}): Promise<T> {
    const method = requestOptions.method ?? 'GET';
    const headers = toHeaderRecord(requestOptions.headers);
    headers.Accept = headers.Accept ?? 'application/json';

    const token = requestOptions.token ?? (requestOptions.auth === false ? null : options.getToken?.());
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }

    let body: BodyInit | undefined;
    if (requestOptions.body !== undefined) {
      if (requestOptions.body instanceof FormData) {
        body = requestOptions.body;
      } else {
        headers['Content-Type'] = headers['Content-Type'] ?? 'application/json';
        body = JSON.stringify(requestOptions.body);
      }
    }

    const response = await fetchImpl(buildUrl(baseUrl, path), {
      method,
      headers,
      body,
      signal: requestOptions.signal,
    });
    const envelope = await readEnvelope<T>(response);

    if (!response.ok || envelope.code !== 'OK') {
      throw new ApiError({
        code: envelope.code || 'HTTP_ERROR',
        message: envelope.message || response.statusText || 'Request failed',
        status: response.status,
        data: envelope.data,
      });
    }

    return envelope.data;
  }

  return {
    request,
    get: (path, requestOptions) => request(path, { ...requestOptions, method: 'GET' }),
    post: (path, body, requestOptions) => request(path, { ...requestOptions, method: 'POST', body }),
    put: (path, body, requestOptions) => request(path, { ...requestOptions, method: 'PUT', body }),
    patch: (path, body, requestOptions) => request(path, { ...requestOptions, method: 'PATCH', body }),
    delete: (path, requestOptions) => request(path, { ...requestOptions, method: 'DELETE' }),
  };
}

function normalizeBaseUrl(baseUrl: string) {
  return baseUrl.replace(/\/+$/, '');
}

function buildUrl(baseUrl: string, path: string) {
  if (/^https?:\/\//i.test(path)) {
    return path;
  }

  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${baseUrl}${normalizedPath}`;
}

function toHeaderRecord(headers?: HeadersInit): Record<string, string> {
  if (!headers) {
    return {};
  }

  if (headers instanceof Headers) {
    const record: Record<string, string> = {};
    headers.forEach((value, key) => {
      record[key] = value;
    });
    return record;
  }

  if (Array.isArray(headers)) {
    return Object.fromEntries(headers);
  }

  return { ...headers };
}

async function readEnvelope<T>(response: Response): Promise<ApiEnvelope<T>> {
  try {
    return (await response.json()) as ApiEnvelope<T>;
  } catch {
    throw new ApiError({
      code: 'INVALID_RESPONSE',
      message: 'Response is not a valid JSON envelope',
      status: response.status,
      data: null,
    });
  }
}
