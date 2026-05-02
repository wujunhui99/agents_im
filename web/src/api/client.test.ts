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
});
