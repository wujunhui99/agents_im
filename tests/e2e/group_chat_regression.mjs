#!/usr/bin/env node
import crypto from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const DEFAULT_OUTPUT_ROOT = '/tmp/agents-im-group-chat-e2e';
const DEFAULT_PRODUCTION_BASE_URL = 'https://agenticim.xyz';
const DEFAULT_LOCAL_BASE_URL = 'http://127.0.0.1:5173';

const CLASSIFICATION = {
  SUCCESS: 'group-chat-success',
  HISTORY_SUCCESS: 'group-chat-history-success',
  PERMISSION_DENIED: 'group-chat-permission-denied',
  MAX_MEMBERS_REJECTED: 'group-chat-max-members-rejected',
  SETUP_FAILED: 'setup-or-harness-failed',
};

const secrets = new Set();

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
      wsBaseUrl: config.wsBaseUrl,
      outputDir: config.outputDir,
      requestTimeoutMs: config.requestTimeoutMs,
    }),
    steps: [],
    http: [],
    summary: {},
  };

  let classification = CLASSIFICATION.SETUP_FAILED;
  let reason = '';

  try {
    ensureFetchAvailable();
    ensureWebSocketAvailable();

    const alice = await register(config, observations, 'groupa');
    const bob = await register(config, observations, 'groupb');
    const carol = await register(config, observations, 'groupc');
    const outsider = await register(config, observations, 'groupx');

    recordStep(observations, 'create-group-with-selected-members', 'started');
    const groupResult = await postJson(config, observations, '/groups', alice.token, {
      name: `QA Group ${crypto.randomUUID().slice(0, 8)}`,
      member_user_ids: [bob.userId, carol.userId],
    });
    if (!isSuccessEnvelope(groupResult)) {
      throw new Error(`create group failed: ${groupResult.body?.code ?? groupResult.httpStatus}`);
    }
    const group = groupResult.body.data;
    recordStep(observations, 'create-group-with-selected-members', 'completed', { groupId: group.group_id, name: group.name });

    const bobSocket = await connectWebSocket(config, observations, bob.token, 'bob-live');
    const livePromise = waitForMessage(bobSocket, (event) => {
      return event?.type === 'message_received' && event?.data?.group_id === group.group_id && event?.data?.content === observations.summary.liveContent;
    });

    observations.summary.liveContent = `group live ${crypto.randomUUID()}`;
    recordStep(observations, 'send-group-message-over-websocket', 'started');
    const aliceSocket = await connectWebSocket(config, observations, alice.token, 'alice-send');
    aliceSocket.send(
      JSON.stringify({
        requestId: `req-${crypto.randomUUID()}`,
        command: 'send_message',
        payload: {
          chatType: 'group',
          groupId: group.group_id,
          clientMsgId: `group-live-${crypto.randomUUID()}`,
          contentType: 'text',
          content: observations.summary.liveContent,
        },
      }),
    );
    const liveEvent = await livePromise;
    recordStep(observations, 'send-group-message-over-websocket', 'completed', {
      serverMsgId: liveEvent.data.server_msg_id,
      conversationId: liveEvent.data.conversation_id,
      seq: liveEvent.data.seq,
    });
    classification = CLASSIFICATION.SUCCESS;
    reason = 'online active group member received WebSocket live push';

    recordStep(observations, 'offline-history-recovery', 'started');
    const history = await getJson(
      config,
      observations,
      `/conversations/${encodeURIComponent(`group:${group.group_id}`)}/messages?fromSeq=1&limit=50&order=asc`,
      carol.token,
    );
    const recovered = (history.body?.data?.messages ?? []).some((message) => message.content === observations.summary.liveContent);
    if (!isSuccessEnvelope(history) || !recovered) {
      throw new Error('offline member did not recover group message from history');
    }
    classification = CLASSIFICATION.HISTORY_SUCCESS;
    reason = 'offline or missed-push member recovered group message through history';
    recordStep(observations, 'offline-history-recovery', 'completed');

    recordStep(observations, 'permission-denied', 'started');
    const denied = await postJson(config, observations, '/messages', outsider.token, {
      chatType: 'group',
      groupId: group.group_id,
      clientMsgId: `group-denied-${crypto.randomUUID()}`,
      contentType: 'text',
      content: 'outsider should fail',
    });
    if (denied.httpStatus !== 403 && denied.body?.code !== 'FORBIDDEN') {
      throw new Error(`outsider send was not forbidden: ${denied.httpStatus} ${denied.body?.code ?? ''}`);
    }
    classification = CLASSIFICATION.PERMISSION_DENIED;
    reason = 'non-member group send was rejected';
    recordStep(observations, 'permission-denied', 'completed');

    recordStep(observations, 'max-members-rejected', 'started');
    const overLimit = await postJson(config, observations, '/groups', alice.token, {
      name: `Too Large ${crypto.randomUUID().slice(0, 8)}`,
      member_user_ids: Array.from({ length: 200 }, (_, index) => `missing_over_limit_${index}`),
    });
    if (overLimit.httpStatus !== 400 && overLimit.body?.code !== 'INVALID_ARGUMENT') {
      throw new Error(`over-limit group create was not rejected: ${overLimit.httpStatus} ${overLimit.body?.code ?? ''}`);
    }
    classification = CLASSIFICATION.MAX_MEMBERS_REJECTED;
    reason = 'group size greater than 200 was rejected';
    recordStep(observations, 'max-members-rejected', 'completed');

    aliceSocket.close();
    bobSocket.close();
  } catch (error) {
    if (classification === CLASSIFICATION.SETUP_FAILED) {
      reason = error instanceof Error ? error.message : String(error);
    } else {
      reason = `${reason}; later step failed: ${error instanceof Error ? error.message : String(error)}`;
    }
    recordStep(observations, 'harness-error', 'failed', { message: reason });
  }

  observations.completedAt = new Date().toISOString();
  observations.classification = classification;
  observations.reason = reason;
  await writeFile(path.join(config.outputDir, 'observations.json'), JSON.stringify(redactObject(observations), null, 2));

  console.log(JSON.stringify({ classification, reason, outputDir: config.outputDir }, null, 2));
  process.exit(classification === CLASSIFICATION.SETUP_FAILED ? 1 : 0);
}

function readConfig() {
  const target = process.env.AGENTS_IM_E2E_TARGET || 'local';
  const baseUrl =
    process.env.AGENTS_IM_E2E_BASE_URL || (target === 'production' ? DEFAULT_PRODUCTION_BASE_URL : DEFAULT_LOCAL_BASE_URL);
  const apiBaseUrl = process.env.AGENTS_IM_E2E_API_BASE_URL || baseUrl;
  const wsBaseUrl = process.env.AGENTS_IM_E2E_WS_BASE_URL || apiBaseUrl.replace(/^http/i, 'ws');
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  return {
    target,
    baseUrl,
    apiBaseUrl: apiBaseUrl.replace(/\/+$/, ''),
    wsBaseUrl: wsBaseUrl.replace(/\/+$/, ''),
    outputDir: process.env.AGENTS_IM_E2E_OUTPUT_DIR || path.join(DEFAULT_OUTPUT_ROOT, timestamp),
    requestTimeoutMs: Number(process.env.AGENTS_IM_E2E_REQUEST_TIMEOUT_MS || 15000),
  };
}

function ensureFetchAvailable() {
  if (typeof fetch !== 'function') {
    throw new Error('global fetch is unavailable; run with a Node version that provides fetch');
  }
}

function ensureWebSocketAvailable() {
  if (typeof WebSocket !== 'function') {
    throw new Error('global WebSocket is unavailable; run with a Node version that provides WebSocket');
  }
}

async function register(config, observations, prefix) {
  const suffix = crypto.randomUUID().replace(/-/g, '').slice(0, 12);
  const account = {
    identifier: `${prefix}_${suffix}`,
    displayName: `${prefix.toUpperCase()} ${suffix.slice(0, 4)}`,
    password: `Pw-${crypto.randomUUID()}!`,
  };
  secrets.add(account.password);
  const result = await postJson(config, observations, '/auth/register', null, {
    identifier: account.identifier,
    password: account.password,
    display_name: account.displayName,
  });
  addTokenSecrets(result.body);
  if (!isSuccessEnvelope(result)) {
    throw new Error(`register ${prefix} failed: ${result.body?.code ?? result.httpStatus}`);
  }
  return {
    identifier: account.identifier,
    displayName: account.displayName,
    userId: result.body.data.user_id,
    token: result.body.data.token,
  };
}

async function postJson(config, observations, route, token, body) {
  return requestJson(config, observations, route, { method: 'POST', token, body });
}

async function getJson(config, observations, route, token) {
  return requestJson(config, observations, route, { method: 'GET', token });
}

async function requestJson(config, observations, route, { method, token, body }) {
  if (token) secrets.add(token);
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.requestTimeoutMs);
  const headers = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (token) headers.Authorization = `Bearer ${token}`;
  const url = `${config.apiBaseUrl}${route}`;
  try {
    const response = await fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
    });
    const parsed = await response.json().catch(() => null);
    addTokenSecrets(parsed);
    observations.http.push(redactObject({ method, route, status: response.status, body: parsed }));
    return { httpStatus: response.status, body: parsed };
  } finally {
    clearTimeout(timeout);
  }
}

async function connectWebSocket(config, observations, token, label) {
  secrets.add(token);
  const socket = new WebSocket(`${config.wsBaseUrl}/ws?token=${encodeURIComponent(token)}`);
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`${label} websocket timeout`)), config.requestTimeoutMs);
    socket.onopen = () => {
      clearTimeout(timer);
      observations.steps.push({ name: `websocket-${label}`, status: 'opened' });
      resolve();
    };
    socket.onerror = () => {
      clearTimeout(timer);
      reject(new Error(`${label} websocket error`));
    };
  });
  return socket;
}

function waitForMessage(socket, predicate) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('timed out waiting for group live message')), 15000);
    socket.onmessage = (event) => {
      const parsed = safeJson(event.data);
      if (predicate(parsed)) {
        clearTimeout(timer);
        resolve(parsed);
      }
    };
  });
}

function isSuccessEnvelope(result) {
  return result.httpStatus >= 200 && result.httpStatus < 300 && result.body?.code === 'OK';
}

function recordStep(observations, name, status, detail = {}) {
  observations.steps.push(redactObject({ name, status, ...detail }));
}

function safeJson(value) {
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
}

function addTokenSecrets(value) {
  if (!value || typeof value !== 'object') return;
  for (const [key, item] of Object.entries(value)) {
    if (/token|password|cookie|authorization/i.test(key) && typeof item === 'string' && item) {
      secrets.add(item);
    }
    addTokenSecrets(item);
  }
}

function redactObject(value) {
  if (typeof value === 'string') {
    let redacted = value;
    for (const secret of secrets) {
      if (secret) redacted = redacted.split(secret).join('[REDACTED]');
    }
    return redacted.replace(/Bearer\s+[A-Za-z0-9._~+/=-]+/g, 'Bearer [REDACTED]');
  }
  if (Array.isArray(value)) return value.map(redactObject);
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [
        key,
        /token|password|cookie|authorization/i.test(key) ? '[REDACTED]' : redactObject(item),
      ]),
    );
  }
  return value;
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
});
