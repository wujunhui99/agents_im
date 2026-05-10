import { createApiClient, type ApiClient } from './client';

export type AuthResponse = {
  user_id: string;
  identifier: string;
  display_name?: string;
  name?: string;
  gender?: string;
  birth_date?: string;
  region?: string;
  account_type?: 'user' | 'agent' | 'admin';
  avatar_media_id?: string;
  avatar_url?: string;
  token: string;
  expires_at?: string;
};

export type LoginRequest = {
  identifier: string;
  password: string;
};

export type RegisterRequest = LoginRequest & {
  email: string;
  emailVerificationCode: string;
  displayName: string;
};

export type RegistrationEmailCodeData = {
  email: string;
  expire_minutes: number;
};

export type AuthApi = {
  login(input: LoginRequest): Promise<AuthResponse>;
  register(input: RegisterRequest): Promise<AuthResponse>;
  requestRegistrationEmailCode(email: string): Promise<RegistrationEmailCodeData>;
};

export function createAuthApi(api: ApiClient = createApiClient()): AuthApi {
  return {
    login(input) {
      return api.post<AuthResponse>(
        '/auth/login',
        { identifier: input.identifier, password: input.password },
        { auth: false },
      );
    },
    register(input) {
      return api.post<AuthResponse>(
        '/auth/register',
        {
          identifier: input.identifier,
          email: input.email,
          email_verification_code: input.emailVerificationCode,
          password: input.password,
          display_name: input.displayName,
        },
        { auth: false },
      );
    },
    requestRegistrationEmailCode(email) {
      return api.post<RegistrationEmailCodeData>('/auth/register/email-code', { email }, { auth: false });
    },
  };
}
