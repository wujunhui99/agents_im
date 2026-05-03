#!/usr/bin/env node
import crypto from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { createRequire } from 'node:module';

const AUTH_STORAGE_KEY = 'agents_im.auth.v1';
const DEFAULT_OUTPUT_ROOT = '/tmp/agents-im-ws-live-push-e2e';
const DEFAULT_PRODUCTION_BASE_URL = 'https://agenticim.xyz';
const DEFAULT_LOCAL_BASE_URL = 'http://127.0.0.1:5173';

const CLASSIFICATION = {
  LIVE_PUSH_SUCCESS: 'live-push-success',
  FRAME_WITHOUT_UI: 'ws-frame-received-ui-not-displayed',
  STILL_FAILS: 'live-push-still-fails',
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
      livePushWaitMs: config.livePushWaitMs,
      handshakeTimeoutMs: config.handshakeTimeoutMs,
      requestTimeoutMs: config.requestTimeoutMs,
      allowReproFailure: config.allowReproFailure,
      headless: config.headless,
    }),
    accounts: [],
    steps: [],
    http: [],
    send: null,
    conversationId: null,
    uniqueText: null,
    summary: {},
    screenshots: {},
  };
  const wsEvents = [];
  const consoleEvents = [];

  let browser = null;
  let bPage = null;
  let classification = CLASSIFICATION.SETUP_FAILED;
  let reason = '';

  try {
    ensureFetchAvailable();
    const playwright = loadPlaywright();

    recordStep(observations, 'register-account-a', 'started');
    const accountA = await registerAccount(config, observations, 'A');
    recordStep(observations, 'register-account-a', 'completed', publicAccount(accountA));

    recordStep(observations, 'register-account-b', 'started');
    const accountB = await registerAccount(config, observations, 'B');
    recordStep(observations, 'register-account-b', 'completed', publicAccount(accountB));
    observations.accounts = [publicAccount(accountA), publicAccount(accountB)];

    recordStep(observations, 'establish-friendship', 'started');
    await establishFriendship(config, observations, accountA, accountB);
    recordStep(observations, 'establish-friendship', 'completed');

    browser = await launchBrowser(playwright, config);
    const bContext = await browser.newContext();
    await installBrowserSession(bContext, accountB);
    bPage = await bContext.newPage();
    capturePageEvents(bPage, consoleEvents);
    await captureChromiumWebSocketEvents(bContext, bPage, wsEvents);

    recordStep(observations, 'load-b-frontend', 'started');
    await bPage.goto(config.baseUrl, { waitUntil: 'domcontentloaded', timeout: config.navigationTimeoutMs });
    await bPage.waitForLoadState('networkidle', { timeout: config.navigationTimeoutMs }).catch(() => {});
    await waitForCondition(
      () => websocketHandshakeStatuses(wsEvents).includes(101),
      config.handshakeTimeoutMs,
      'B WebSocket did not observe a 101 Switching Protocols handshake',
    );
    recordStep(observations, 'load-b-frontend', 'completed', {
      handshakeStatuses: websocketHandshakeStatuses(wsEvents),
    });
    await takeScreenshot(bPage, config, observations, 'b-before-send.png');

    const uniqueText = `ws-live-push-regression-${new Date().toISOString()}-${shortId()}`;
    observations.uniqueText = uniqueText;
    const conversationId = singleConversationId(accountA.userId, accountB.userId);
    observations.conversationId = conversationId;

    recordStep(observations, 'send-unique-message-a-to-b', 'started');
    const sendStartedAt = Date.now();
    const sendResult = await sendMessage(config, observations, accountA, accountB, uniqueText);
    observations.send = redactObject({
      httpStatus: sendResult.httpStatus,
      clientMsgId: sendResult.clientMsgId,
      serverMsgId: sendResult.message?.serverMsgId,
      conversationId: sendResult.message?.conversationId,
      seq: sendResult.message?.seq,
      deduplicated: sendResult.deduplicated,
    });
    recordStep(observations, 'send-unique-message-a-to-b', 'completed', observations.send);

    await bPage.waitForTimeout(config.livePushWaitMs);
    const uiContainsMessage = await pageBodyContains(bPage, uniqueText);
    await takeScreenshot(bPage, config, observations, 'b-after-wait-no-refresh.png');

    recordStep(observations, 'pull-b-history', 'started');
    const history = await pullHistory(config, observations, accountB, conversationId);
    const historyContainsMessage = history.messages.some((message) => String(message.content ?? '').includes(uniqueText));
    recordStep(observations, 'pull-b-history', 'completed', {
      conversationId,
      historyContainsMessage,
      messageCount: history.messages.length,
    });

    const summary = summarizeRun({
      wsEvents,
      sendStartedAt,
      uniqueText,
      uiContainsMessage,
      historyContainsMessage,
      sendHttpStatus: sendResult.httpStatus,
    });
    observations.summary = summary;

    const classified = classify(summary);
    classification = classified.classification;
    reason = classified.reason;
  } catch (error) {
    classification = CLASSIFICATION.SETUP_FAILED;
    reason = errorMessage(error);
    observations.error = redactObject({
      name: error?.name,
      message: errorMessage(error),
      details: error?.details,
      stack: error?.stack,
    });
    if (bPage) {
      await takeScreenshot(bPage, config, observations, 'b-setup-failure.png').catch(() => {});
    }
  } finally {
    if (browser) {
      await browser.close().catch(() => {});
    }

    observations.completedAt = new Date().toISOString();
    observations.classification = classification;
    observations.reason = reason;

    await writeArtifacts(config, observations, wsEvents, consoleEvents);
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
    livePushWaitMs: positiveInt(args.waitMs ?? process.env.AGENTS_IM_E2E_WAIT_MS, 10_000, 'waitMs'),
    handshakeTimeoutMs: positiveInt(
      args.handshakeTimeoutMs ?? process.env.AGENTS_IM_E2E_HANDSHAKE_TIMEOUT_MS,
      10_000,
      'handshakeTimeoutMs',
    ),
    navigationTimeoutMs: positiveInt(
      args.navigationTimeoutMs ?? process.env.AGENTS_IM_E2E_NAVIGATION_TIMEOUT_MS,
      20_000,
      'navigationTimeoutMs',
    ),
    requestTimeoutMs: positiveInt(args.requestTimeoutMs ?? process.env.AGENTS_IM_E2E_REQUEST_TIMEOUT_MS, 15_000, 'requestTimeoutMs'),
    allowReproFailure: truthy(args.allowReproFailure ?? process.env.AGENTS_IM_E2E_ALLOW_REPRO_FAILURE),
    headless: !truthy(args.headed ?? process.env.AGENTS_IM_E2E_HEADED),
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
  if (value === true) {
    return true;
  }
  if (value === false || value === undefined || value === null) {
    return false;
  }
  return ['1', 'true', 'yes', 'y', 'on'].includes(String(value).trim().toLowerCase());
}

function ensureFetchAvailable() {
  if (typeof fetch !== 'function') {
    throw new HarnessError('This harness requires Node.js with global fetch support. Use Node.js 18 or newer.');
  }
}

function loadPlaywright() {
  const require = createRequire(import.meta.url);
  try {
    return require('playwright');
  } catch (error) {
    throw new HarnessError(
      [
        'Playwright is required for this browser E2E harness, but it is not installed for this checkout.',
        'Install it outside the repo and expose it with NODE_PATH, for example:',
        '  mkdir -p /tmp/ws-e2e-run',
        '  npm --prefix /tmp/ws-e2e-run install playwright',
        '  NODE_PATH=/tmp/ws-e2e-run/node_modules node tests/e2e/ws_live_push_regression.mjs',
        'If you intentionally add a repo dev dependency instead, install it with the project package manager and rerun this script.',
      ].join('\n'),
      { cause: errorMessage(error) },
    );
  }
}

async function launchBrowser(playwright, config) {
  try {
    return await playwright.chromium.launch({ headless: config.headless });
  } catch (error) {
    throw new HarnessError(
      [
        `Failed to launch Playwright Chromium: ${errorMessage(error)}`,
        'If the browser binary is missing, run:',
        '  NODE_PATH=/tmp/ws-e2e-run/node_modules npx --prefix /tmp/ws-e2e-run playwright install chromium',
      ].join('\n'),
    );
  }
}

async function registerAccount(config, observations, role) {
  const suffix = `${base36Now()}${shortId(5)}`.toLowerCase();
  const identifier = `qaws_${role.toLowerCase()}_${suffix}`.slice(0, 32);
  const displayName = `QA WS ${role} ${suffix}`;
  const password = `QaWs-${suffix}-${crypto.randomBytes(8).toString('hex')}`;
  addSecret(password);

  const response = await apiRequest(config, observations, {
    label: `register account ${role}`,
    method: 'POST',
    path: '/auth/register',
    body: {
      identifier,
      password,
      display_name: displayName,
    },
    auth: false,
  });

  const data = response.data;
  if (!data?.user_id || !data?.identifier || !data?.token) {
    throw new HarnessError(`register account ${role} returned an invalid auth payload`);
  }
  addSecret(data.token);
  return {
    role,
    userId: String(data.user_id),
    identifier: String(data.identifier),
    displayName,
    token: String(data.token),
    expiresAt: data.expires_at ? String(data.expires_at) : undefined,
  };
}

async function establishFriendship(config, observations, accountA, accountB) {
  const addResult = await apiRequest(config, observations, {
    label: 'A sends friend request to B',
    method: 'POST',
    path: '/friends',
    token: accountA.token,
    body: { user_id: accountB.userId },
  });

  const addStatus = addResult.data?.friendship?.status;
  if (addStatus !== 'accepted' && addStatus !== 'active') {
    await apiRequest(config, observations, {
      label: 'B accepts A friend request',
      method: 'POST',
      path: `/friends/requests/${encodeURIComponent(accountA.userId)}/accept`,
      token: accountB.token,
      body: {},
    });
  }

  await assertAcceptedFriend(config, observations, accountA, accountB);
  await assertAcceptedFriend(config, observations, accountB, accountA);
}

async function assertAcceptedFriend(config, observations, owner, expectedFriend) {
  const result = await apiRequest(config, observations, {
    label: `${owner.role} lists accepted friends`,
    method: 'GET',
    path: '/friends',
    token: owner.token,
  });
  const friends = Array.isArray(result.data?.friends) ? result.data.friends : [];
  const found = friends.some((friendship) => {
    const status = friendship?.status;
    return (
      String(friendship?.friend_id ?? '') === expectedFriend.userId &&
      (status === 'accepted' || status === 'active' || friendship?.is_friend === true)
    );
  });
  if (!found) {
    throw new HarnessError(`${owner.role} friend list does not contain accepted friend ${expectedFriend.role}`);
  }
}

async function installBrowserSession(context, account) {
  const session = {
    token: account.token,
    expiresAt: account.expiresAt,
    user: {
      userId: account.userId,
      identifier: account.identifier,
      displayName: account.displayName || account.identifier,
    },
  };

  await context.addInitScript(
    ({ key, value }) => {
      window.localStorage.setItem(key, JSON.stringify(value));
    },
    { key: AUTH_STORAGE_KEY, value: session },
  );
}

function capturePageEvents(page, consoleEvents) {
  page.on('console', (message) => {
    consoleEvents.push(
      redactObject({
        at: new Date().toISOString(),
        event: 'console',
        type: message.type(),
        text: message.text(),
        location: message.location(),
      }),
    );
  });
  page.on('pageerror', (error) => {
    consoleEvents.push(
      redactObject({
        at: new Date().toISOString(),
        event: 'pageerror',
        message: errorMessage(error),
        stack: error?.stack,
      }),
    );
  });
  page.on('requestfailed', (request) => {
    consoleEvents.push(
      redactObject({
        at: new Date().toISOString(),
        event: 'requestfailed',
        url: request.url(),
        method: request.method(),
        failure: request.failure(),
      }),
    );
  });
}

async function captureChromiumWebSocketEvents(context, page, wsEvents) {
  const cdp = await context.newCDPSession(page);
  const wsRequestIds = new Set();
  await cdp.send('Network.enable');

  cdp.on('Network.webSocketCreated', (event) => {
    wsRequestIds.add(event.requestId);
    wsEvents.push(
      redactObject({
        at: new Date().toISOString(),
        event: 'webSocketCreated',
        requestId: event.requestId,
        url: event.url,
      }),
    );
  });

  cdp.on('Network.webSocketWillSendHandshakeRequest', (event) => {
    wsRequestIds.add(event.requestId);
    wsEvents.push(
      redactObject({
        at: new Date().toISOString(),
        event: 'webSocketWillSendHandshakeRequest',
        requestId: event.requestId,
        url: event.request?.url,
        headers: event.request?.headers,
      }),
    );
  });

  cdp.on('Network.webSocketHandshakeResponseReceived', (event) => {
    wsRequestIds.add(event.requestId);
    wsEvents.push(
      redactObject({
        at: new Date().toISOString(),
        event: 'webSocketHandshakeResponseReceived',
        requestId: event.requestId,
        url: event.response?.url,
        status: event.response?.status,
        statusText: event.response?.statusText,
        headers: event.response?.headers,
      }),
    );
  });

  cdp.on('Network.webSocketFrameReceived', (event) => {
    wsRequestIds.add(event.requestId);
    const payloadData = event.response?.payloadData ?? '';
    wsEvents.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        event: 'webSocketFrameReceived',
        requestId: event.requestId,
        opcode: event.response?.opcode,
        mask: event.response?.mask,
        payloadData,
        parsed: parseJson(payloadData),
      }),
    );
  });

  cdp.on('Network.webSocketFrameSent', (event) => {
    wsRequestIds.add(event.requestId);
    const payloadData = event.response?.payloadData ?? '';
    wsEvents.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        event: 'webSocketFrameSent',
        requestId: event.requestId,
        opcode: event.response?.opcode,
        mask: event.response?.mask,
        payloadData,
        parsed: parseJson(payloadData),
      }),
    );
  });

  cdp.on('Network.webSocketFrameError', (event) => {
    wsRequestIds.add(event.requestId);
    wsEvents.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        event: 'webSocketFrameError',
        requestId: event.requestId,
        errorMessage: event.errorMessage,
      }),
    );
  });

  cdp.on('Network.webSocketClosed', (event) => {
    wsRequestIds.add(event.requestId);
    wsEvents.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        event: 'webSocketClosed',
        requestId: event.requestId,
      }),
    );
  });

  cdp.on('Network.loadingFailed', (event) => {
    if (!wsRequestIds.has(event.requestId)) {
      return;
    }
    wsEvents.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        event: 'loadingFailed',
        requestId: event.requestId,
        errorText: event.errorText,
        canceled: event.canceled,
        blockedReason: event.blockedReason,
      }),
    );
  });
}

async function sendMessage(config, observations, accountA, accountB, uniqueText) {
  const clientMsgId = `qa-ws-${Date.now()}-${shortId(8)}`;
  const response = await apiRequest(config, observations, {
    label: 'A sends unique message to B',
    method: 'POST',
    path: '/messages',
    token: accountA.token,
    body: {
      receiverId: accountB.userId,
      chatType: 'single',
      clientMsgId,
      contentType: 'text',
      content: uniqueText,
    },
  });

  return {
    httpStatus: response.status,
    clientMsgId,
    message: response.data?.message,
    deduplicated: response.data?.deduplicated,
  };
}

async function pullHistory(config, observations, accountB, conversationId) {
  const response = await apiRequest(config, observations, {
    label: 'B pulls single conversation history',
    method: 'GET',
    path: `/conversations/${encodeURIComponent(conversationId)}/messages?fromSeq=1&limit=100&order=asc`,
    token: accountB.token,
  });
  return {
    messages: Array.isArray(response.data?.messages) ? response.data.messages : [],
  };
}

async function apiRequest(config, observations, request) {
  const url = buildUrl(config.apiBaseUrl, request.path);
  const startedAt = Date.now();
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.requestTimeoutMs);
  const headers = {
    Accept: 'application/json',
  };
  if (request.body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }
  if (request.token) {
    headers.Authorization = `Bearer ${request.token}`;
  }

  let response;
  let responseText = '';
  try {
    response = await fetch(url, {
      method: request.method,
      headers,
      body: request.body === undefined ? undefined : JSON.stringify(request.body),
      signal: controller.signal,
    });
    responseText = await response.text();
  } catch (error) {
    observations.http.push(
      redactObject({
        label: request.label,
        method: request.method,
        path: request.path,
        url,
        failed: true,
        error: errorMessage(error),
        durationMs: Date.now() - startedAt,
      }),
    );
    throw new HarnessError(`${request.label} request failed: ${errorMessage(error)}`);
  } finally {
    clearTimeout(timeout);
  }

  const envelope = parseJson(responseText);
  observations.http.push(
    redactObject({
      label: request.label,
      method: request.method,
      path: request.path,
      url,
      status: response.status,
      ok: response.ok,
      envelopeCode: envelope?.code,
      envelopeMessage: envelope?.message,
      data: envelope?.data,
      durationMs: Date.now() - startedAt,
    }),
  );

  if (!response.ok || response.status !== 200 || !envelope || envelope.code !== 'OK') {
    throw new HarnessError(
      `${request.label} failed: HTTP ${response.status}, envelope code ${envelope?.code ?? 'INVALID_RESPONSE'}, message ${redactString(
        envelope?.message ?? response.statusText ?? responseText,
      )}`,
    );
  }

  return {
    status: response.status,
    data: envelope.data,
    envelope,
  };
}

function buildUrl(baseUrl, requestPath) {
  return new URL(requestPath, `${baseUrl}/`).toString();
}

async function waitForCondition(predicate, timeoutMs, message) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (predicate()) {
      return;
    }
    await sleep(100);
  }
  throw new HarnessError(message);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function pageBodyContains(page, text) {
  try {
    const bodyText = await page.locator('body').textContent({ timeout: 2_000 });
    return Boolean(bodyText?.includes(text));
  } catch {
    return false;
  }
}

async function takeScreenshot(page, config, observations, filename) {
  if (!page || page.isClosed()) {
    return;
  }
  const filePath = path.join(config.outputDir, filename);
  await page.screenshot({ path: filePath, fullPage: true });
  observations.screenshots[filename] = filePath;
}

function summarizeRun({ wsEvents, sendStartedAt, uniqueText, uiContainsMessage, historyContainsMessage, sendHttpStatus }) {
  const handshakeStatuses = websocketHandshakeStatuses(wsEvents);
  const incomingFrames = wsEvents.filter((event) => event.event === 'webSocketFrameReceived');
  const incomingFramesAfterSend = incomingFrames.filter((event) => Number(event.epochMs ?? 0) >= sendStartedAt);
  const messageReceivedFramesAfterSend = incomingFramesAfterSend.filter((event) => websocketFrameType(event) === 'message_received');
  const matchedIncomingFramesAfterSend = incomingFramesAfterSend.filter((event) => frameContainsText(event, uniqueText));
  const matchedMessageReceivedFramesAfterSend = messageReceivedFramesAfterSend.filter((event) => frameContainsText(event, uniqueText));
  const closeEvents = wsEvents.filter((event) => event.event === 'webSocketClosed');
  const loadingFailures = wsEvents.filter((event) => event.event === 'loadingFailed');
  const frameErrors = wsEvents.filter((event) => event.event === 'webSocketFrameError');

  return {
    bWebSocketHandshakeStatuses: handshakeStatuses,
    bWebSocketObserved101: handshakeStatuses.includes(101),
    bWebSocketCloseEventCount: closeEvents.length,
    bWebSocketLoadingFailedEventCount: loadingFailures.length,
    bWebSocketFrameErrorCount: frameErrors.length,
    bIncomingFrameCount: incomingFrames.length,
    bIncomingFrameCountAfterSend: incomingFramesAfterSend.length,
    bMessageReceivedFrameCountAfterSend: messageReceivedFramesAfterSend.length,
    bMatchedIncomingFrameCountAfterSend: matchedIncomingFramesAfterSend.length,
    bMatchedMessageReceivedFrameCountAfterSend: matchedMessageReceivedFramesAfterSend.length,
    bNoRefreshDisplayContainsMessage: uiContainsMessage,
    bPullHistoryContainsMessage: historyContainsMessage,
    aSendHttpStatus: sendHttpStatus,
  };
}

function classify(summary) {
  if (
    summary.bWebSocketObserved101 &&
    summary.aSendHttpStatus === 200 &&
    summary.bMatchedMessageReceivedFrameCountAfterSend > 0 &&
    summary.bNoRefreshDisplayContainsMessage
  ) {
    return {
      classification: CLASSIFICATION.LIVE_PUSH_SUCCESS,
      reason: 'B received a matching message_received frame and the no-refresh UI displayed the unique message.',
    };
  }

  if (
    summary.bWebSocketObserved101 &&
    summary.aSendHttpStatus === 200 &&
    summary.bMatchedMessageReceivedFrameCountAfterSend > 0 &&
    !summary.bNoRefreshDisplayContainsMessage
  ) {
    return {
      classification: CLASSIFICATION.FRAME_WITHOUT_UI,
      reason: 'B received a matching message_received frame, but the no-refresh UI did not display the unique message.',
    };
  }

  if (
    summary.bWebSocketObserved101 &&
    summary.aSendHttpStatus === 200 &&
    summary.bPullHistoryContainsMessage &&
    summary.bMatchedMessageReceivedFrameCountAfterSend === 0 &&
    !summary.bNoRefreshDisplayContainsMessage
  ) {
    return {
      classification: CLASSIFICATION.STILL_FAILS,
      reason:
        'B WebSocket reached 101 and A send returned 200; B history contains the message, but B received no matching message_received frame and the no-refresh UI did not update.',
    };
  }

  return {
    classification: CLASSIFICATION.SETUP_FAILED,
    reason:
      'Run did not meet a stable live-push regression classification. Check setup, handshake, send status, history pull, and captured browser events.',
  };
}

function websocketHandshakeStatuses(wsEvents) {
  return wsEvents
    .filter((event) => event.event === 'webSocketHandshakeResponseReceived')
    .map((event) => Number(event.status))
    .filter((status) => Number.isFinite(status));
}

function websocketFrameType(event) {
  const parsed = event.parsed;
  if (parsed && typeof parsed.type === 'string') {
    return parsed.type;
  }
  return undefined;
}

function frameContainsText(event, text) {
  if (typeof event.payloadData === 'string' && event.payloadData.includes(text)) {
    return true;
  }
  return JSON.stringify(event.parsed ?? '').includes(text);
}

function singleConversationId(userIdA, userIdB) {
  const [lower, higher] = compareUserIds(userIdA, userIdB) <= 0 ? [userIdA, userIdB] : [userIdB, userIdA];
  return `single:${lower}:${higher}`;
}

function compareUserIds(left, right) {
  const leftString = String(left);
  const rightString = String(right);
  if (/^\d+$/.test(leftString) && /^\d+$/.test(rightString)) {
    const leftBigInt = BigInt(leftString);
    const rightBigInt = BigInt(rightString);
    if (leftBigInt < rightBigInt) {
      return -1;
    }
    if (leftBigInt > rightBigInt) {
      return 1;
    }
    return 0;
  }
  return leftString.localeCompare(rightString);
}

function publicAccount(account) {
  return {
    role: account.role,
    userId: account.userId,
    identifier: account.identifier,
    displayName: account.displayName,
  };
}

function recordStep(observations, name, status, details = undefined) {
  observations.steps.push(
    redactObject({
      at: new Date().toISOString(),
      name,
      status,
      details,
    }),
  );
}

async function writeArtifacts(config, observations, wsEvents, consoleEvents) {
  await mkdir(config.outputDir, { recursive: true });
  await writeFile(path.join(config.outputDir, 'observations.redacted.json'), `${JSON.stringify(redactObject(observations), null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'ws-events.redacted.json'), `${JSON.stringify(redactObject(wsEvents), null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'console.redacted.json'), `${JSON.stringify(redactObject(consoleEvents), null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'report.txt'), reportText(config, observations));
}

function reportText(config, observations) {
  const summary = observations.summary ?? {};
  const lines = [
    'WebSocket live-push E2E regression report',
    '',
    `classification: ${observations.classification}`,
    `reason: ${observations.reason}`,
    `target: ${config.target}`,
    `base URL: ${config.baseUrl}`,
    `API base URL: ${config.apiBaseUrl}`,
    `output dir: ${config.outputDir}`,
    `started at: ${observations.startedAt}`,
    `completed at: ${observations.completedAt}`,
    '',
    `A account: ${accountLine(observations.accounts?.find((account) => account.role === 'A'))}`,
    `B account: ${accountLine(observations.accounts?.find((account) => account.role === 'B'))}`,
    `conversation ID: ${observations.conversationId ?? '(not reached)'}`,
    `unique text: ${observations.uniqueText ?? '(not reached)'}`,
    '',
    `A send HTTP status: ${summary.aSendHttpStatus ?? observations.send?.httpStatus ?? '(not reached)'}`,
    `B websocket handshake statuses: ${formatList(summary.bWebSocketHandshakeStatuses)}`,
    `B websocket close events: ${summary.bWebSocketCloseEventCount ?? 0}`,
    `B websocket loadingFailed events: ${summary.bWebSocketLoadingFailedEventCount ?? 0}`,
    `B websocket frame error events: ${summary.bWebSocketFrameErrorCount ?? 0}`,
    `B incoming frame count after send: ${summary.bIncomingFrameCountAfterSend ?? 0}`,
    `B message_received frame count after send: ${summary.bMessageReceivedFrameCountAfterSend ?? 0}`,
    `B matched message_received frame count after send: ${summary.bMatchedMessageReceivedFrameCountAfterSend ?? 0}`,
    `B no-refresh display contains message: ${summary.bNoRefreshDisplayContainsMessage ?? false}`,
    `B pull/history contains message: ${summary.bPullHistoryContainsMessage ?? false}`,
    '',
    'Artifacts:',
    '- report.txt',
    '- observations.redacted.json',
    '- ws-events.redacted.json',
    '- console.redacted.json',
  ];

  for (const filename of Object.keys(observations.screenshots ?? {})) {
    lines.push(`- ${filename}`);
  }

  return `${redactString(lines.join('\n'))}\n`;
}

function accountLine(account) {
  if (!account) {
    return '(not reached)';
  }
  return `${account.identifier} userId=${account.userId}`;
}

function formatList(values) {
  return Array.isArray(values) && values.length > 0 ? values.join(',') : '(none)';
}

function printConsoleSummary(config, observations, exitCode) {
  const lines = [
    `classification: ${observations.classification}`,
    `reason: ${observations.reason}`,
    `evidence: ${config.outputDir}`,
    `exitCode: ${exitCode}`,
  ];
  process.stdout.write(`${redactString(lines.join('\n'))}\n`);
}

function exitCodeFor(classification, allowReproFailure) {
  if (classification === CLASSIFICATION.LIVE_PUSH_SUCCESS) {
    return 0;
  }
  if (allowReproFailure && classification !== CLASSIFICATION.SETUP_FAILED) {
    return 0;
  }
  return 1;
}

function parseJson(value) {
  if (!value) {
    return null;
  }
  try {
    return JSON.parse(value);
  } catch {
    return null;
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
  if (typeof value === 'string') {
    return redactString(value);
  }
  if (typeof value !== 'object') {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item) => redactObject(item));
  }
  const next = {};
  for (const [key, item] of Object.entries(value)) {
    if (/token|password|authorization|cookie|set-cookie|jwt|secret/i.test(key)) {
      next[key] = '[REDACTED]';
    } else {
      next[key] = redactObject(item);
    }
  }
  return next;
}

function redactString(value) {
  let next = String(value);
  const orderedSecrets = [...secrets].filter(Boolean).sort((left, right) => right.length - left.length);
  for (const secret of orderedSecrets) {
    next = next.split(secret).join('[REDACTED]');
  }
  next = next.replace(/([?&]token=)[^&\s"'<>)]*/gi, '$1[REDACTED]');
  next = next.replace(/\bBearer\s+[A-Za-z0-9._~+/=-]+/gi, 'Bearer [REDACTED]');
  next = next.replace(/\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g, '[REDACTED]');
  next = next.replace(/("?(?:token|password|authorization|cookie|jwt|secret)"?\s*[:=]\s*)("[^"]*"|'[^']*'|[^,\s}]+)/gi, '$1[REDACTED]');
  return next;
}

function errorMessage(error) {
  if (error instanceof Error) {
    return redactString(error.message);
  }
  return redactString(String(error));
}

function base36Now() {
  return Date.now().toString(36);
}

function shortId(length = 6) {
  return crypto.randomBytes(Math.ceil(length / 2)).toString('hex').slice(0, length);
}

function timestampForPath(date) {
  return date.toISOString().replace(/[:.]/g, '-');
}

await main();
