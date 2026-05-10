import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';
import { createAuthApi } from './auth';

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

describe('auth API adapter', () => {
  it('requests a registration email code through the public auth endpoint', async () => {
    const fetcher = vi.fn<typeof fetch>(async () =>
      jsonResponse({
        code: 'OK',
        message: 'ok',
        data: { email: 'alice@example.com', expire_minutes: 10 },
      }),
    );
    const client = createApiClient({ baseUrl: 'http://api.test', fetchImpl: fetcher });
    const api = createAuthApi(client);

    await expect(api.requestRegistrationEmailCode('alice@example.com')).resolves.toEqual({
      email: 'alice@example.com',
      expire_minutes: 10,
    });

    expect(fetcher).toHaveBeenCalledWith(
      'http://api.test/auth/register/email-code',
      expect.objectContaining({
        method: 'POST',
        headers: expect.not.objectContaining({
          Authorization: expect.any(String),
        }),
        body: JSON.stringify({ email: 'alice@example.com' }),
      }),
    );
  });

  it('registers with email and email verification code through the public auth endpoint', async () => {
    const fetcher = vi.fn<typeof fetch>(async () =>
      jsonResponse({
        code: 'OK',
        message: 'ok',
        data: {
          user_id: '1001',
          identifier: 'alice_email',
          display_name: 'Alice',
          token: 'register-token',
          expires_at: '2026-04-30T12:00:00Z',
        },
      }),
    );
    const client = createApiClient({ baseUrl: 'http://api.test', getToken: () => 'stale-token', fetchImpl: fetcher });
    const api = createAuthApi(client);

    await api.register({
      identifier: 'alice_email',
      email: 'alice@example.com',
      emailVerificationCode: '123456',
      password: 'test-password',
      displayName: 'Alice',
    });

    expect(fetcher).toHaveBeenCalledWith(
      'http://api.test/auth/register',
      expect.objectContaining({
        method: 'POST',
        headers: expect.not.objectContaining({
          Authorization: expect.any(String),
        }),
        body: JSON.stringify({
          identifier: 'alice_email',
          email: 'alice@example.com',
          email_verification_code: '123456',
          password: 'test-password',
          display_name: 'Alice',
        }),
      }),
    );
  });
});
