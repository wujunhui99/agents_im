import { ApiError, createApiClient } from './client';

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

describe('REST API client', () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('unwraps response envelopes and injects the bearer token', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        code: 'OK',
        message: 'ok',
        data: { user_id: '1001', identifier: 'alice_001' },
      }),
    );

    const client = createApiClient({
      baseUrl: 'https://api.example.test',
      getToken: () => 'mock-access-token',
    });

    await expect(client.get('/me')).resolves.toEqual({
      user_id: '1001',
      identifier: 'alice_001',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.example.test/me',
      expect.objectContaining({
        method: 'GET',
        headers: expect.objectContaining({
          Accept: 'application/json',
          Authorization: 'Bearer mock-access-token',
        }),
      }),
    );
  });

  it('throws typed API errors from failed envelopes', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        {
          code: 'UNAUTHENTICATED',
          message: 'token is invalid',
          data: null,
        },
        { status: 401 },
      ),
    );

    const client = createApiClient({ baseUrl: '' });

    await expect(client.get('/me')).rejects.toMatchObject({
      name: 'ApiError',
      code: 'UNAUTHENTICATED',
      message: 'token is invalid',
      status: 401,
    } satisfies Partial<ApiError>);
  });

  it('notifies auth failure handlers for protected stale-session responses', async () => {
    const onAuthFailure = vi.fn();
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        {
          code: 'UNAUTHENTICATED',
          message: 'invalid or missing bearer token',
          data: null,
        },
        { status: 401 },
      ),
    );

    const client = createApiClient({
      getToken: () => 'old-device-token',
      onAuthFailure,
    });

    await expect(client.get('/me')).rejects.toMatchObject({
      code: 'UNAUTHENTICATED',
      status: 401,
    });
    expect(onAuthFailure).toHaveBeenCalledWith(
      expect.objectContaining({
        token: 'old-device-token',
        path: '/me',
        error: expect.objectContaining({
          code: 'UNAUTHENTICATED',
          status: 401,
        }),
      }),
    );
  });

  it('does not notify auth failure handlers for public login failures', async () => {
    const onAuthFailure = vi.fn();
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        {
          code: 'UNAUTHENTICATED',
          message: 'invalid identifier or password',
          data: null,
        },
        { status: 401 },
      ),
    );

    const client = createApiClient({ onAuthFailure });

    await expect(client.post('/auth/login', { identifier: 'alice_001', password: 'wrong-password' }, { auth: false })).rejects.toMatchObject({
      code: 'UNAUTHENTICATED',
      status: 401,
    });
    expect(onAuthFailure).not.toHaveBeenCalled();
  });
});
