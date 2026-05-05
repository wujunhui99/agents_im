#!/usr/bin/env node
import crypto from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const DEFAULT_OUTPUT_ROOT = '/tmp/agents-im-auth-register-login-e2e';
const DEFAULT_PRODUCTION_BASE_URL = 'https://agenticim.xyz';
const DEFAULT_LOCAL_BASE_URL = 'http://127.0.0.1:5173';

const CLASSIFICATION = {
  SUCCESS: 'register-login-success',
  LOGIN_INVALID_AFTER_REGISTER: 'login-invalid-after-register',
  REGISTER_FAILED: 'register-failed',
  SETUP_FAILED: 'setup-or-harness-failed',
};

const secrets = new Set();

class HarnessError extends Error {
  constructor(message, details = {}) {
    super(message);
    this.name = 'HarnessError';
    this.details = details;
  }
}

async function main() {
  const config = readConfig();
  await mkdir(config.outputDir, { recursive: true });

  const observations = {
    startedAt: new Date().toISOString(),
    completedAt: null,
    classification: CLASSIFICATION.SETUP_FAILED,
    reason: '',
    config: redactObject({
      target: config.target,
      baseUrl: config.baseUrl,
      apiBaseUrl: config.apiBaseUrl,
      outputDir: config.outputDir,
      requestTimeoutMs: config.requestTimeoutMs,
      allowReproFailure: config.allowReproFailure,
    }),
    account: null,
    steps: [],
    http: [],
    summary: {},
  };

  let classification = CLASSIFICATION.SETUP_FAILED;
  let reason = '';

  try {
    ensureFetchAvailable();
    const account = createFreshAccount();
    observations.account = {
      identifier: account.identifier,
      displayName: account.displayName,
    };

    recordStep(observations, 'register-fresh-account', 'started');
    const registerResult = await postJson(config, observations, '/auth/register', {
      identifier: account.identifier,
      password: account.password,
      display_name: account.displayName,
    });
    addTokenSecrets(registerResult.body);
    if (!isSuccessEnvelope(registerResult)) {
      classification = CLASSIFICATION.REGISTER_FAILED;
      reason = summarizeFailure('register failed', registerResult);
      recordStep(observations, 'register-fresh-account', 'failed', {
        httpStatus: registerResult.httpStatus,
        code: registerResult.body?.code,
        message: registerResult.body?.message,
      });
    } else {
      recordStep(observations, 'register-fresh-account', 'completed', {
        httpStatus: registerResult.httpStatus,
        code: registerResult.body?.code,
        userId: registerResult.body?.data?.user_id,
        identifier: registerResult.body?.data?.identifier,
      });

      recordStep(observations, 'login-with-same-identifier-password', 'started');
      const loginResult = await postJson(config, observations, '/auth/login', {
        identifier: account.identifier,
        password: account.password,
      });
      addTokenSecrets(loginResult.body);

      if (isSuccessEnvelope(loginResult) && loginResult.body?.data?.token) {
        classification = CLASSIFICATION.SUCCESS;
        reason = 'freshly registered account logged in successfully';
        recordStep(observations, 'login-with-same-identifier-password', 'completed', {
          httpStatus: loginResult.httpStatus,
          code: loginResult.body?.code,
          userId: loginResult.body?.data?.user_id,
          identifier: loginResult.body?.data?.identifier,
        });
      } else if (isInvalidIdentifierOrPassword(loginResult)) {
        classification = CLASSIFICATION.LOGIN_INVALID_AFTER_REGISTER;
        reason = summarizeFailure('login returned invalid identifier or password after successful register', loginResult);
        recordStep(observations, 'login-with-same-identifier-password', 'failed', {
          httpStatus: loginResult.httpStatus,
          code: loginResult.body?.code,
          message: loginResult.body?.message,
        });
      } else {
        classification = CLASSIFICATION.SETUP_FAILED;
        reason = summarizeFailure('login failed with an unexpected response after successful register', loginResult);
        recordStep(observations, 'login-with-same-identifier-password', 'failed', {
          httpStatus: loginResult.httpStatus,
          code: loginResult.body?.code,
          message: loginResult.body?.message,
        });
      }
    }
  } catch (error) {
    classification = CLASSIFICATION.SETUP_FAILED;
    reason = errorMessage(error);
    observations.error = redactObject({
      name: error?.name,
      message: errorMessage(error),
      details: error?.details,
      stack: error?.stack,
    });
  } finally {
    observations.completedAt = new Date().toISOString();
    observations.classification = classification;
    observations.reason = reason;
    observations.summary = {
      registerSucceeded: observations.steps.some((step) => step.name === 'register-fresh-account' && step.status === 'completed'),
      loginSucceeded: classification === CLASSIFICATION.SUCCESS,
      invalidLoginAfterRegister: classification === CLASSIFICATION.LOGIN_INVALID_AFTER_REGISTER,
    };

    await writeArtifacts(config, observations);
    const exitCode = exitCodeFor(classification, config.allowReproFailure);
    printConsoleSummary(config, observations, exitCode);
    process.exitCode = exitCode;
  }
}

function readConfig() {
  const args = parseArgs(process.argv.slice(2));
  const target = String(args.target ?? process.env.AGENTS_IM_E2E_TARGET ?? 'production').toLowerCase();
  if (!['production', 'local'].includes(target)) {
    throw new HarnessError(`AGENTS_IM_E2E_TARGET must be "production" or "local", got "${target}"`);
  }

  const defaultBaseUrl = target === 'local' ? DEFAULT_LOCAL_BASE_URL : DEFAULT_PRODUCTION_BASE_URL;
  const baseUrl = normalizeBaseUrl(String(args.baseUrl ?? process.env.AGENTS_IM_E2E_BASE_URL ?? defaultBaseUrl));
  const apiBaseUrl = normalizeBaseUrl(String(args.apiBaseUrl ?? process.env.AGENTS_IM_E2E_API_BASE_URL ?? baseUrl));
  const outputDir = path.resolve(
    String(
      args.outputDir ??
        process.env.AGENTS_IM_E2E_OUTPUT_DIR ??
        path.join(DEFAULT_OUTPUT_ROOT, timestampForPath(new Date())),
    ),
  );

  return {
    target,
    baseUrl,
    apiBaseUrl,
    outputDir,
    requestTimeoutMs: positiveInt(args.requestTimeoutMs ?? process.env.AGENTS_IM_E2E_REQUEST_TIMEOUT_MS, 15_000, 'requestTimeoutMs'),
    allowReproFailure: truthy(args.allowReproFailure ?? process.env.AGENTS_IM_E2E_ALLOW_REPRO_FAILURE),
  };
}

function parseArgs(argv) {
  const args = {};
  for (let index = 0; index < argv.length; index += 1) {
    const item = argv[index];
    if (!item.startsWith('--')) {
      continue;
    }
    const withoutPrefix = item.slice(2);
    const equalsIndex = withoutPrefix.indexOf('=');
    if (equalsIndex >= 0) {
      args[toCamelCase(withoutPrefix.slice(0, equalsIndex))] = withoutPrefix.slice(equalsIndex + 1);
      continue;
    }
    const key = toCamelCase(withoutPrefix);
    const next = argv[index + 1];
    if (next && !next.startsWith('--')) {
      args[key] = next;
      index += 1;
    } else {
      args[key] = '1';
    }
  }
  return args;
}

function toCamelCase(value) {
  return value.replace(/-([a-z])/g, (_, char) => char.toUpperCase());
}

function normalizeBaseUrl(value) {
  try {
    const url = new URL(value);
    url.hash = '';
    return url.toString().replace(/\/+$/, '');
  } catch {
    throw new HarnessError(`Invalid base URL: ${value}`);
  }
}

function positiveInt(value, fallback, name) {
  if (value === undefined || value === null || value === '') {
    return fallback;
  }
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new HarnessError(`${name} must be a positive integer, got "${value}"`);
  }
  return parsed;
}

function truthy(value) {
  return ['1', 'true', 'yes', 'y', 'on'].includes(String(value ?? '').toLowerCase());
}

function ensureFetchAvailable() {
  if (typeof fetch !== 'function') {
    throw new HarnessError('global fetch is not available; run with Node.js 18 or newer');
  }
}

function createFreshAccount() {
  const suffix = `${Date.now().toString(36)}_${crypto.randomBytes(4).toString('hex')}`;
  const account = {
    identifier: `auth_e2e_${suffix}`,
    password: `QaAuth-${suffix}-${crypto.randomBytes(8).toString('hex')}`,
    displayName: `Auth E2E ${suffix}`,
  };
  addSecret(account.password);
  return account;
}

async function postJson(config, observations, endpoint, body) {
  const url = new URL(endpoint, `${config.apiBaseUrl}/`).toString();
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.requestTimeoutMs);
  const startedAt = Date.now();
  const httpObservation = {
    method: 'POST',
    endpoint,
    url,
    request: {
      headers: redactObject({ 'Content-Type': 'application/json', Accept: 'application/json' }),
      body: redactObject(body),
    },
    response: null,
    durationMs: null,
  };

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify(body),
      signal: controller.signal,
    });
    const text = await response.text();
    const parsed = parseJSON(text);
    httpObservation.response = redactObject({
      httpStatus: response.status,
      ok: response.ok,
      headers: responseHeaders(response),
      body: parsed.ok ? parsed.value : text,
    });
    return {
      httpStatus: response.status,
      ok: response.ok,
      body: parsed.ok ? parsed.value : null,
      rawBody: text,
    };
  } catch (error) {
    httpObservation.response = redactObject({
      error: errorMessage(error),
    });
    throw error;
  } finally {
    clearTimeout(timeout);
    httpObservation.durationMs = Date.now() - startedAt;
    observations.http.push(redactObject(httpObservation));
  }
}

function parseJSON(text) {
  try {
    return { ok: true, value: JSON.parse(text) };
  } catch {
    return { ok: false, value: null };
  }
}

function responseHeaders(response) {
  const headers = {};
  response.headers.forEach((value, key) => {
    headers[key] = value;
  });
  return headers;
}

function isSuccessEnvelope(result) {
  return result.ok && result.body?.code === 'OK' && result.body?.data;
}

function isInvalidIdentifierOrPassword(result) {
  const code = String(result.body?.code ?? '').toUpperCase();
  const message = String(result.body?.message ?? '');
  return result.httpStatus === 401 || code === 'UNAUTHENTICATED' || /invalid identifier or password/i.test(message);
}

function summarizeFailure(prefix, result) {
  const code = result.body?.code ? ` code=${result.body.code}` : '';
  const message = result.body?.message ? ` message=${result.body.message}` : '';
  return `${prefix}: httpStatus=${result.httpStatus}${code}${message}`;
}

function recordStep(observations, name, status, details = {}) {
  observations.steps.push({
    at: new Date().toISOString(),
    name,
    status,
    details: redactObject(details),
  });
}

function addTokenSecrets(value) {
  if (!value || typeof value !== 'object') {
    return;
  }
  if (typeof value.token === 'string') {
    addSecret(value.token);
  }
  if (value.data && typeof value.data === 'object' && typeof value.data.token === 'string') {
    addSecret(value.data.token);
  }
}

function addSecret(value) {
  if (typeof value === 'string' && value.length > 0) {
    secrets.add(value);
  }
}

function redactObject(value) {
  if (value === null || value === undefined) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item) => redactObject(item));
  }
  if (typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => {
        if (isSensitiveKey(key)) {
          return [key, '[REDACTED]'];
        }
        return [key, redactObject(item)];
      }),
    );
  }
  if (typeof value === 'string') {
    return redactString(value);
  }
  return value;
}

function redactString(value) {
  let next = value;
  for (const secret of secrets) {
    next = next.split(secret).join('[REDACTED]');
  }
  next = next.replace(/Bearer\s+[A-Za-z0-9._~+/=-]+/gi, 'Bearer [REDACTED]');
  next = next.replace(/\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g, '[REDACTED]');
  next = next.replace(/("?(?:token|password|authorization|cookie|jwt|secret)"?\s*[:=]\s*)("[^"]*"|'[^']*'|[^,\s}]+)/gi, '$1[REDACTED]');
  return next;
}

function isSensitiveKey(key) {
  return /password|token|authorization|cookie|set-cookie|jwt|secret/i.test(key);
}

async function writeArtifacts(config, observations) {
  const redactedObservations = redactObject(observations);
  await writeFile(path.join(config.outputDir, 'observations.redacted.json'), `${JSON.stringify(redactedObservations, null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'http.redacted.json'), `${JSON.stringify(redactedObservations.http, null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'report.txt'), reportText(redactedObservations));
}

function reportText(observations) {
  return [
    `classification: ${observations.classification}`,
    `reason: ${observations.reason}`,
    `target: ${observations.config.target}`,
    `baseUrl: ${observations.config.baseUrl}`,
    `apiBaseUrl: ${observations.config.apiBaseUrl}`,
    `startedAt: ${observations.startedAt}`,
    `completedAt: ${observations.completedAt}`,
    '',
    'steps:',
    ...observations.steps.map((step) => `- ${step.name}: ${step.status}`),
    '',
  ].join('\n');
}

function exitCodeFor(classification, allowReproFailure) {
  if (classification === CLASSIFICATION.SUCCESS) {
    return 0;
  }
  if (classification === CLASSIFICATION.LOGIN_INVALID_AFTER_REGISTER && allowReproFailure) {
    return 0;
  }
  return 1;
}

function printConsoleSummary(config, observations, exitCode) {
  console.log(`classification=${observations.classification}`);
  console.log(`reason=${redactString(observations.reason)}`);
  console.log(`artifacts=${config.outputDir}`);
  console.log(`exitCode=${exitCode}`);
}

function errorMessage(error) {
  if (error?.name === 'AbortError') {
    return 'request timed out';
  }
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function timestampForPath(date) {
  return date.toISOString().replace(/[:.]/g, '-');
}

main().catch((error) => {
  console.error(redactString(errorMessage(error)));
  process.exitCode = 1;
});
