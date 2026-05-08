export type ApiEnvelope<T> = {
  code: string;
  message: string;
  data: T;
};

export type ApiClientOptions = {
  baseUrl?: string;
  getToken?: () => string | null | undefined;
  fetchImpl?: typeof fetch;
  onAuthFailure?: (context: ApiAuthFailureContext) => void;
};

export type ApiAuthFailureContext = {
  error: ApiError;
  token: string | null;
  path: string;
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

    const authEnabled = requestOptions.auth !== false;
    const token = requestOptions.token ?? (authEnabled ? options.getToken?.() ?? null : null);
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
    let envelope: ApiEnvelope<T>;
    try {
      envelope = await readEnvelope<T>(response);
    } catch (error) {
      notifyAuthFailure(options, error, authEnabled, token, path);
      throw error;
    }

    if (!response.ok || envelope.code !== 'OK') {
      const error = new ApiError({
        code: envelope.code || 'HTTP_ERROR',
        message: envelope.message || response.statusText || 'Request failed',
        status: response.status,
        data: envelope.data,
      });
      notifyAuthFailure(options, error, authEnabled, token, path);
      throw error;
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

const authFailureCodes = new Set([
  'UNAUTHENTICATED',
  'UNAUTHORIZED',
  'SESSION_INACTIVE',
  'SESSION_INVALID',
  'SESSION_REPLACED',
  'SESSION_EXPIRED',
  'TOKEN_INVALID',
  'TOKEN_EXPIRED',
]);

export function isAuthFailureError(error: unknown): error is ApiError {
  if (!(error instanceof ApiError)) {
    return false;
  }

  return error.status === 401 || authFailureCodes.has(error.code.toUpperCase()) || isAuthFailureMessage(error.message);
}

function notifyAuthFailure(
  options: ApiClientOptions,
  error: unknown,
  authEnabled: boolean,
  token: string | null,
  path: string,
) {
  if (!authEnabled || !options.onAuthFailure || !isAuthFailureError(error)) {
    return;
  }

  options.onAuthFailure({ error, token, path });
}

function isAuthFailureMessage(message: string) {
  const normalized = message.toLowerCase();
  return (
    normalized.includes('invalid or missing bearer token') ||
    normalized.includes('token session is not active') ||
    normalized.includes('session inactive') ||
    normalized.includes('session invalid') ||
    normalized.includes('session replaced') ||
    normalized.includes('token expired')
  );
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
