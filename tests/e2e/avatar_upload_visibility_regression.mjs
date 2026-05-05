#!/usr/bin/env node
import crypto from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const DEFAULT_OUTPUT_ROOT = '/tmp/agents-im-avatar-upload-e2e';
const DEFAULT_PRODUCTION_BASE_URL = 'https://agenticim.xyz';
const DEFAULT_LOCAL_BASE_URL = 'http://127.0.0.1:5173';
const PNG_1X1 = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=',
  'base64',
);

const CLASSIFICATION = {
  AVATAR_UPLOAD_SUCCESS: 'avatar-upload-success',
  AVATAR_VALIDATION_FAILED: 'avatar-validation-failed',
  AVATAR_VISIBILITY_SUCCESS: 'avatar-visibility-success',
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
    }),
    accounts: {},
    steps: [],
    http: [],
    summary: {},
  };

  let classification = CLASSIFICATION.SETUP_FAILED;
  let reason = '';

  try {
    ensureFetchAvailable();
    const alice = createFreshAccount('avatar_a');
    const bob = createFreshAccount('avatar_b');
    observations.accounts = {
      alice: { identifier: alice.identifier, displayName: alice.displayName },
      bob: { identifier: bob.identifier, displayName: bob.displayName },
    };

    recordStep(observations, 'register-alice', 'started');
    alice.session = await register(config, observations, alice);
    recordStep(observations, 'register-alice', 'completed', publicSession(alice.session));

    recordStep(observations, 'register-bob', 'started');
    bob.session = await register(config, observations, bob);
    recordStep(observations, 'register-bob', 'completed', publicSession(bob.session));

    recordStep(observations, 'avatar-validation-failed', 'started');
    const invalidIntent = await postJson(config, observations, '/media/uploads', {
      purpose: 'avatar',
      filename: 'animated.gif',
      contentType: 'image/gif',
      sizeBytes: 1024,
    }, alice.session.token);
    if (isSuccessEnvelope(invalidIntent)) {
      throw new HarnessError('avatar GIF upload intent unexpectedly succeeded');
    }
    classification = CLASSIFICATION.AVATAR_VALIDATION_FAILED;
    recordStep(observations, 'avatar-validation-failed', 'completed', {
      httpStatus: invalidIntent.httpStatus,
      code: invalidIntent.body?.code,
      message: invalidIntent.body?.message,
    });

    recordStep(observations, 'establish-friendship', 'started');
    await establishFriendship(config, observations, alice.session, bob.session);
    recordStep(observations, 'establish-friendship', 'completed');

    recordStep(observations, 'upload-avatar-success', 'started');
    const avatar = await uploadAvatar(config, observations, alice.session);
    classification = CLASSIFICATION.AVATAR_UPLOAD_SUCCESS;
    recordStep(observations, 'upload-avatar-success', 'completed', avatar);

    recordStep(observations, 'seed-direct-conversation', 'started');
    const message = await sendMessage(config, observations, alice.session, bob.session.userId);
    recordStep(observations, 'seed-direct-conversation', 'completed', message);

    recordStep(observations, 'avatar-visibility-success', 'started');
    const visibility = await verifyContactVisibility(config, observations, bob.session, alice.session.userId, avatar.mediaId);
    classification = CLASSIFICATION.AVATAR_VISIBILITY_SUCCESS;
    reason = 'avatar upload succeeded and accepted contact received avatar display data';
    recordStep(observations, 'avatar-visibility-success', 'completed', visibility);
  } catch (error) {
    if (!reason) {
      reason = errorMessage(error);
    }
    observations.error = redactObject({
      name: error?.name,
      message: errorMessage(error),
      details: error?.details,
      stack: error?.stack,
    });
  } finally {
    observations.completedAt = new Date().toISOString();
    observations.classification = classification;
    observations.reason = reason || observations.reason;
    observations.summary = {
      validationRejected: observations.steps.some((step) => step.name === 'avatar-validation-failed' && step.status === 'completed'),
      avatarUploaded: observations.steps.some((step) => step.name === 'upload-avatar-success' && step.status === 'completed'),
      contactVisibilityVerified: observations.steps.some((step) => step.name === 'avatar-visibility-success' && step.status === 'completed'),
    };
    await writeArtifacts(config, observations);
    const exitCode = classification === CLASSIFICATION.SETUP_FAILED ? 1 : 0;
    printConsoleSummary(config, observations, exitCode);
    process.exitCode = exitCode;
  }
}

async function register(config, observations, account) {
  const result = await postJson(config, observations, '/auth/register', {
    identifier: account.identifier,
    password: account.password,
    display_name: account.displayName,
  });
  addTokenSecrets(result.body);
  if (!isSuccessEnvelope(result) || !result.body?.data?.token) {
    throw new HarnessError('register failed', summarizeHttp(result));
  }
  return {
    userId: String(result.body.data.user_id),
    identifier: String(result.body.data.identifier),
    token: String(result.body.data.token),
  };
}

async function establishFriendship(config, observations, alice, bob) {
  const add = await postJson(config, observations, '/friends', { user_id: bob.userId }, alice.token);
  if (!isSuccessEnvelope(add)) {
    throw new HarnessError('add friend failed', summarizeHttp(add));
  }
  const accept = await postJson(config, observations, `/friends/requests/${encodeURIComponent(alice.userId)}/accept`, {}, bob.token);
  if (!isSuccessEnvelope(accept)) {
    throw new HarnessError('accept friend request failed', summarizeHttp(accept));
  }
}

async function uploadAvatar(config, observations, owner) {
  const sha256 = crypto.createHash('sha256').update(PNG_1X1).digest('hex');
  const intent = await postJson(config, observations, '/media/uploads', {
    purpose: 'avatar',
    filename: 'avatar.png',
    contentType: 'image/png',
    sizeBytes: PNG_1X1.length,
    sha256,
    width: 1,
    height: 1,
  }, owner.token);
  addURLSecret(intent.body?.data?.uploadUrl);
  if (!isSuccessEnvelope(intent) || !intent.body?.data?.mediaId || !intent.body?.data?.uploadUrl) {
    throw new HarnessError('avatar upload intent failed', summarizeHttp(intent));
  }

  const put = await fetchWithTimeout(String(intent.body.data.uploadUrl), {
    method: 'PUT',
    headers: { 'Content-Type': 'image/png' },
    body: PNG_1X1,
  }, config.requestTimeoutMs);
  observations.http.push(redactObject({
    label: 'PUT avatar bytes to presigned upload URL',
    method: 'PUT',
    path: '[REDACTED_PRESIGNED_UPLOAD_URL]',
    httpStatus: put.status,
    ok: put.ok,
  }));
  if (!put.ok) {
    throw new HarnessError('avatar byte upload failed', { httpStatus: put.status });
  }

  const mediaId = String(intent.body.data.mediaId);
  const complete = await postJson(config, observations, `/media/uploads/${encodeURIComponent(mediaId)}/complete`, {}, owner.token);
  if (!isSuccessEnvelope(complete)) {
    throw new HarnessError('complete avatar upload failed', summarizeHttp(complete));
  }

  const patch = await patchJson(config, observations, '/me/avatar', { mediaId }, owner.token);
  addURLSecret(patch.body?.data?.avatar_url);
  if (!isSuccessEnvelope(patch) || patch.body?.data?.avatar_media_id !== mediaId || !patch.body?.data?.avatar_url) {
    throw new HarnessError('profile avatar update failed', summarizeHttp(patch));
  }

  return {
    mediaId,
    profileHasAvatarUrl: Boolean(patch.body?.data?.avatar_url),
    avatarUrlExpiresAt: patch.body?.data?.avatar_url_expires_at ?? null,
  };
}

async function sendMessage(config, observations, sender, receiverId) {
  const clientMsgId = `avatar-e2e-${Date.now()}-${crypto.randomBytes(3).toString('hex')}`;
  const result = await postJson(config, observations, '/messages', {
    receiverId,
    chatType: 'single',
    clientMsgId,
    contentType: 'text',
    content: 'avatar visibility regression',
  }, sender.token);
  if (!isSuccessEnvelope(result)) {
    throw new HarnessError('seed direct message failed', summarizeHttp(result));
  }
  return {
    conversationId: result.body?.data?.message?.conversationId ?? null,
    serverMsgId: result.body?.data?.message?.serverMsgId ?? null,
  };
}

async function verifyContactVisibility(config, observations, viewer, expectedFriendId, expectedMediaId) {
  const result = await getJson(config, observations, '/friends', viewer.token);
  if (!isSuccessEnvelope(result)) {
    throw new HarnessError('list friends failed while verifying avatar visibility', summarizeHttp(result));
  }
  const friends = Array.isArray(result.body?.data?.friends) ? result.body.data.friends : [];
  const friend = friends.find((item) => String(item?.friend_id ?? '') === String(expectedFriendId));
  if (!friend) {
    throw new HarnessError('accepted friend was not visible in contact list');
  }
  const profile = friend.friend ?? friend.friend_profile ?? friend.profile ?? {};
  addURLSecret(profile.avatar_url);
  if (profile.avatar_media_id !== expectedMediaId || !profile.avatar_url) {
    throw new HarnessError('accepted contact avatar display data missing', {
      hasAvatarUrl: Boolean(profile.avatar_url),
      avatarMediaIDMatches: profile.avatar_media_id === expectedMediaId,
    });
  }
  return {
    friendIdentifier: profile.identifier ?? null,
    avatarMediaIDMatches: true,
    hasAvatarUrl: true,
    avatarUrlExpiresAt: profile.avatar_url_expires_at ?? null,
  };
}

async function postJson(config, observations, apiPath, body, token) {
  return requestJson(config, observations, 'POST', apiPath, body, token);
}

async function patchJson(config, observations, apiPath, body, token) {
  return requestJson(config, observations, 'PATCH', apiPath, body, token);
}

async function getJson(config, observations, apiPath, token) {
  return requestJson(config, observations, 'GET', apiPath, undefined, token);
}

async function requestJson(config, observations, method, apiPath, body, token) {
  const headers = { Accept: 'application/json' };
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  const response = await fetchWithTimeout(config.apiBaseUrl + apiPath, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  }, config.requestTimeoutMs);
  const text = await response.text();
  let parsed = null;
  try {
    parsed = text ? JSON.parse(text) : null;
  } catch {
    parsed = { invalidJson: text.slice(0, 200) };
  }
  addTokenSecrets(parsed);
  addURLSecret(parsed?.data?.uploadUrl);
  addURLSecret(parsed?.data?.downloadUrl);
  addURLSecret(parsed?.data?.avatar_url);
  observations.http.push(redactObject({
    method,
    path: apiPath,
    httpStatus: response.status,
    ok: response.ok,
    code: parsed?.code,
    message: parsed?.message,
    dataShape: summarizeDataShape(parsed?.data),
  }));
  return { httpStatus: response.status, ok: response.ok, body: parsed };
}

async function fetchWithTimeout(url, options, timeoutMs) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...options, signal: controller.signal });
  } finally {
    clearTimeout(timeout);
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
  const outputDir = path.resolve(String(args.outputDir ?? process.env.AGENTS_IM_E2E_OUTPUT_DIR ?? path.join(DEFAULT_OUTPUT_ROOT, timestampForPath(new Date()))));
  const requestTimeoutMs = Number(args.requestTimeoutMs ?? process.env.AGENTS_IM_E2E_REQUEST_TIMEOUT_MS ?? 15000);
  return { target, baseUrl, apiBaseUrl, outputDir, requestTimeoutMs };
}

function createFreshAccount(prefix) {
  const suffix = `${Date.now()}_${crypto.randomBytes(3).toString('hex')}`;
  return {
    identifier: `${prefix}_${suffix}`,
    displayName: `${prefix}_${suffix}`,
    password: crypto.randomBytes(18).toString('base64url'),
  };
}

function isSuccessEnvelope(result) {
  return result.ok && result.body?.code === 'OK';
}

function publicSession(session) {
  return { userId: session.userId, identifier: session.identifier, hasToken: Boolean(session.token) };
}

function summarizeHttp(result) {
  return {
    httpStatus: result.httpStatus,
    ok: result.ok,
    code: result.body?.code,
    message: result.body?.message,
  };
}

function summarizeDataShape(data) {
  if (!data || typeof data !== 'object') {
    return data === null ? null : typeof data;
  }
  return Object.fromEntries(Object.keys(data).map((key) => [key, key.toLowerCase().includes('url') || key.toLowerCase().includes('token') ? '[REDACTED]' : typeof data[key]]));
}

function addTokenSecrets(value) {
  if (!value || typeof value !== 'object') {
    return;
  }
  if (typeof value.token === 'string') {
    secrets.add(value.token);
  }
  if (value.data && typeof value.data === 'object' && typeof value.data.token === 'string') {
    secrets.add(value.data.token);
  }
}

function addURLSecret(value) {
  if (typeof value === 'string' && value) {
    secrets.add(value);
  }
}

function redactObject(value) {
  return JSON.parse(redactString(JSON.stringify(value ?? null)));
}

function redactString(value) {
  let redacted = value;
  for (const secret of secrets) {
    if (secret) {
      redacted = redacted.split(secret).join('[REDACTED]');
    }
  }
  redacted = redacted.replace(
    /"((?:userId|receiverId|expectedFriendId|user_id|friend_id|account_id|mediaId|expectedMediaId|avatar_media_id|conversationId|serverMsgId))"\s*:\s*"[^"]*"/g,
    '"$1":"[REDACTED]"',
  );
  redacted = redacted.replace(/(\/friends\/requests\/)[^/"\\]+(?=\/(?:accept|reject))/g, '$1[REDACTED]');
  redacted = redacted.replace(/(\/media\/uploads\/)[^/"\\]+(?=\/complete)/g, '$1[REDACTED]');
  redacted = redacted.replace(/([?&](?:X-Amz-Signature|signature|token|X-Amz-Credential)=)[^"&\s]+/gi, '$1[REDACTED]');
  redacted = redacted.replace(/"objectKey"\s*:\s*"[^"]*"/gi, '"objectKey":"[REDACTED]"');
  redacted = redacted.replace(/"uploadUrl"\s*:\s*"[^"]*"/gi, '"uploadUrl":"[REDACTED]"');
  redacted = redacted.replace(/"downloadUrl"\s*:\s*"[^"]*"/gi, '"downloadUrl":"[REDACTED]"');
  redacted = redacted.replace(/"avatar_url"\s*:\s*"[^"]*"/gi, '"avatar_url":"[REDACTED]"');
  return redacted;
}

function recordStep(observations, name, status, details = {}) {
  observations.steps.push(redactObject({ name, status, at: new Date().toISOString(), ...details }));
}

async function writeArtifacts(config, observations) {
  await writeFile(path.join(config.outputDir, 'observations.json'), `${JSON.stringify(redactObject(observations), null, 2)}\n`);
}

function printConsoleSummary(config, observations, exitCode) {
  const lines = [
    `classification: ${observations.classification}`,
    `reason: ${observations.reason || '(none)'}`,
    `output: ${config.outputDir}`,
    `exitCode: ${exitCode}`,
  ];
  console.log(lines.join('\n'));
}

function parseArgs(argv) {
  const args = {};
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (!arg.startsWith('--')) {
      continue;
    }
    const [key, inlineValue] = arg.slice(2).split('=', 2);
    args[key] = inlineValue ?? argv[index + 1];
    if (inlineValue === undefined) {
      index += 1;
    }
  }
  return args;
}

function normalizeBaseUrl(value) {
  return value.replace(/\/+$/, '');
}

function timestampForPath(date) {
  return date.toISOString().replace(/[:.]/g, '-');
}

function ensureFetchAvailable() {
  if (typeof fetch !== 'function') {
    throw new HarnessError('global fetch is unavailable; use Node.js 18 or newer');
  }
}

function errorMessage(error) {
  return error instanceof Error ? error.message : String(error);
}

main();
