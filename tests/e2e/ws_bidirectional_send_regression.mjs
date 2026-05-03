#!/usr/bin/env node
import crypto from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { createRequire } from 'node:module';

const AUTH_STORAGE_KEY = 'agents_im.auth.v1';
const DEFAULT_OUTPUT_ROOT = '/tmp/agents-im-ws-bidirectional-send-e2e';
const DEFAULT_PRODUCTION_BASE_URL = 'https://agenticim.xyz';
const DEFAULT_LOCAL_BASE_URL = 'http://127.0.0.1:5173';

const CLASSIFICATION = {
  SUCCESS: 'bidirectional-send-success',
  REVERSE_BAD_REQUEST: 'reverse-send-bad-request',
  REVERSE_UI_DISABLED_OR_MISSING_TARGET: 'reverse-send-ui-disabled-or-missing-target',
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
      replyWaitMs: config.replyWaitMs,
      handshakeTimeoutMs: config.handshakeTimeoutMs,
      requestTimeoutMs: config.requestTimeoutMs,
      allowReproFailure: config.allowReproFailure,
      headless: config.headless,
    }),
    accounts: [],
    steps: [],
    http: [],
    network: [],
    console: [],
    ws: [],
    seedSend: null,
    aToBSend: null,
    reverseSend: null,
    conversationId: null,
    seedText: null,
    aToBText: null,
    bToAText: null,
    summary: {},
    screenshots: {},
  };

  let browser = null;
  let aPage = null;
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

    const conversationId = singleConversationId(accountA.userId, accountB.userId);
    const runId = `${timestampForText(new Date())}-${shortId()}`;
    const seedText = `ws-bidirectional-seed-${runId}`;
    const aToBText = `ws-bidirectional-a-to-b-${runId}`;
    const bToAText = `ws-bidirectional-b-to-a-${runId}`;
    observations.conversationId = conversationId;
    observations.seedText = seedText;
    observations.aToBText = aToBText;
    observations.bToAText = bToAText;

    recordStep(observations, 'seed-existing-conversation-a-to-b', 'started');
    const seedSend = await sendMessage(config, observations, {
      label: 'A seeds existing conversation with B',
      sender: accountA,
      receiver: accountB,
      content: seedText,
    });
    observations.seedSend = publicSendResult(seedSend);
    recordStep(observations, 'seed-existing-conversation-a-to-b', 'completed', observations.seedSend);

    browser = await launchBrowser(playwright, config);

    const aContext = await browser.newContext();
    await installBrowserSession(aContext, accountA);
    aPage = await aContext.newPage();
    capturePageEvents('A', aPage, observations);
    await captureChromiumWebSocketEvents('A', aContext, aPage, observations);

    const bContext = await browser.newContext();
    await installBrowserSession(bContext, accountB);
    bPage = await bContext.newPage();
    capturePageEvents('B', bPage, observations);
    await captureChromiumWebSocketEvents('B', bContext, bPage, observations);

    recordStep(observations, 'open-a-existing-conversation', 'started');
    await openExistingConversation(config, observations, aPage, 'A', seedText);
    recordStep(observations, 'open-a-existing-conversation', 'completed', {
      handshakeStatuses: websocketHandshakeStatuses(observations.ws, 'A'),
    });

    recordStep(observations, 'open-b-existing-conversation', 'started');
    await openExistingConversation(config, observations, bPage, 'B', seedText);
    recordStep(observations, 'open-b-existing-conversation', 'completed', {
      handshakeStatuses: websocketHandshakeStatuses(observations.ws, 'B'),
    });

    await takeScreenshot(bPage, config, observations, 'b-chat-before-live-send.png');
    await takeScreenshot(aPage, config, observations, 'a-chat-before-reverse-send.png');

    recordStep(observations, 'send-live-message-a-to-b', 'started');
    const aToBSendStartedAt = Date.now();
    const aToBSend = await sendMessage(config, observations, {
      label: 'A sends no-refresh message to B',
      sender: accountA,
      receiver: accountB,
      content: aToBText,
    });
    observations.aToBSend = publicSendResult(aToBSend);
    await bPage.waitForTimeout(config.livePushWaitMs);
    const bNoRefreshContainsAToB = await pageBodyContains(bPage, aToBText);
    await takeScreenshot(bPage, config, observations, 'b-chat-after-a-to-b-live.png');
    recordStep(observations, 'send-live-message-a-to-b', 'completed', {
      ...observations.aToBSend,
      bNoRefreshContainsAToB,
    });

    let bHistoryContainsAToB = false;
    if (!bNoRefreshContainsAToB) {
      const bHistory = await pullHistory(config, observations, accountB, conversationId, 'B verifies missed A->B history');
      bHistoryContainsAToB = historyContainsText(bHistory, aToBText);
      observations.summary = summarizeRun({
        observations,
        aToBSendStartedAt,
        bNoRefreshContainsAToB,
        bHistoryContainsAToB,
      });
      throw new HarnessError(
        'B did not display the A->B message without refresh, so the reverse no-refresh send precondition was not met.',
        { bHistoryContainsAToB },
      );
    }
    const bHistory = await pullHistory(config, observations, accountB, conversationId, 'B verifies A->B history');
    bHistoryContainsAToB = historyContainsText(bHistory, aToBText);

    recordStep(observations, 'reverse-send-b-to-a-same-page', 'started');
    const reverseSendStartedAt = Date.now();
    const reverseResult = await sendFromCurrentConversationUi(config, observations, bPage, bToAText);
    observations.reverseSend = reverseResult;
    await takeScreenshot(bPage, config, observations, 'b-chat-after-reverse-send-attempt.png');

    let aNoRefreshContainsReply = false;
    let aHistoryContainsReply = false;
    if (reverseResult?.httpStatus === 200 && reverseResult?.envelopeCode === 'OK') {
      await aPage.waitForTimeout(config.replyWaitMs);
      aNoRefreshContainsReply = await pageBodyContains(aPage, bToAText);
      const aHistory = await pullHistory(config, observations, accountA, conversationId, 'A verifies B reply history');
      aHistoryContainsReply = historyContainsText(aHistory, bToAText);
      await takeScreenshot(aPage, config, observations, 'a-chat-after-b-reply.png');
    }

    observations.summary = summarizeRun({
      observations,
      aToBSendStartedAt,
      reverseSendStartedAt,
      bNoRefreshContainsAToB,
      bHistoryContainsAToB,
      aNoRefreshContainsReply,
      aHistoryContainsReply,
    });

    const classified = classify(observations.summary, reverseResult);
    classification = classified.classification;
    reason = classified.reason;
    recordStep(observations, 'reverse-send-b-to-a-same-page', 'completed', {
      reverseSend: reverseResult,
      aNoRefreshContainsReply,
      aHistoryContainsReply,
    });
  } catch (error) {
    if (classification === CLASSIFICATION.SETUP_FAILED) {
      classification = CLASSIFICATION.SETUP_FAILED;
    }
    reason = reason || errorMessage(error);
    observations.error = redactObject({
      name: error?.name,
      message: errorMessage(error),
      details: error?.details,
      stack: error?.stack,
    });
    if (bPage) {
      await takeScreenshot(bPage, config, observations, 'b-failure.png').catch(() => {});
    }
    if (aPage) {
      await takeScreenshot(aPage, config, observations, 'a-failure.png').catch(() => {});
    }
  } finally {
    if (browser) {
      await browser.close().catch(() => {});
    }

    observations.completedAt = new Date().toISOString();
    observations.classification = classification;
    observations.reason = reason;

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
    livePushWaitMs: positiveInt(args.waitMs ?? process.env.AGENTS_IM_E2E_WAIT_MS, 10_000, 'waitMs'),
    replyWaitMs: positiveInt(args.replyWaitMs ?? process.env.AGENTS_IM_E2E_REPLY_WAIT_MS, 5_000, 'replyWaitMs'),
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
    reverseResponseTimeoutMs: positiveInt(
      args.reverseResponseTimeoutMs ?? process.env.AGENTS_IM_E2E_REVERSE_RESPONSE_TIMEOUT_MS,
      8_000,
      'reverseResponseTimeoutMs',
    ),
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
        '  NODE_PATH=/tmp/ws-e2e-run/node_modules node tests/e2e/ws_bidirectional_send_regression.mjs',
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
  const identifier = `qabd_${role.toLowerCase()}_${suffix}`.slice(0, 32);
  const displayName = `QA BD ${role} ${suffix}`;
  const password = `QaBd-${suffix}-${crypto.randomBytes(8).toString('hex')}`;
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

async function sendMessage(config, observations, { label, sender, receiver, content }) {
  const clientMsgId = `qa-bd-${Date.now()}-${shortId(8)}`;
  const response = await apiRequest(config, observations, {
    label,
    method: 'POST',
    path: '/messages',
    token: sender.token,
    body: {
      receiverId: receiver.userId,
      chatType: 'single',
      clientMsgId,
      contentType: 'text',
      content,
    },
  });

  return {
    httpStatus: response.status,
    clientMsgId,
    message: response.data?.message,
    deduplicated: response.data?.deduplicated,
  };
}

async function pullHistory(config, observations, account, conversationId, label) {
  const response = await apiRequest(config, observations, {
    label,
    method: 'GET',
    path: `/conversations/${encodeURIComponent(conversationId)}/messages?fromSeq=1&limit=100&order=asc`,
    token: account.token,
  });
  return {
    messages: Array.isArray(response.data?.messages) ? response.data.messages : [],
  };
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

function capturePageEvents(role, page, observations) {
  page.on('console', (message) => {
    observations.console.push(
      redactObject({
        at: new Date().toISOString(),
        role,
        event: 'console',
        type: message.type(),
        text: message.text(),
        location: message.location(),
      }),
    );
  });
  page.on('pageerror', (error) => {
    observations.console.push(
      redactObject({
        at: new Date().toISOString(),
        role,
        event: 'pageerror',
        message: errorMessage(error),
        stack: error?.stack,
      }),
    );
  });
  page.on('requestfailed', (request) => {
    observations.network.push(
      redactObject({
        at: new Date().toISOString(),
        role,
        event: 'requestfailed',
        url: request.url(),
        method: request.method(),
        failure: request.failure(),
      }),
    );
  });
  page.on('response', async (response) => {
    let parsedUrl;
    try {
      parsedUrl = new URL(response.url());
    } catch {
      return;
    }
    if (parsedUrl.pathname !== '/messages' || response.request().method() !== 'POST') {
      return;
    }

    let responseBody = '';
    try {
      responseBody = await response.text();
    } catch {
      responseBody = '';
    }

    observations.network.push(
      redactObject({
        at: new Date().toISOString(),
        role,
        event: 'post-messages-response',
        url: response.url(),
        method: response.request().method(),
        status: response.status(),
        statusText: response.statusText(),
        requestPostData: response.request().postData(),
        responseBody,
      }),
    );
  });
}

async function captureChromiumWebSocketEvents(role, context, page, observations) {
  const cdp = await context.newCDPSession(page);
  const wsRequestIds = new Set();
  await cdp.send('Network.enable');

  cdp.on('Network.webSocketCreated', (event) => {
    wsRequestIds.add(event.requestId);
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        role,
        event: 'webSocketCreated',
        requestId: event.requestId,
        url: event.url,
      }),
    );
  });

  cdp.on('Network.webSocketWillSendHandshakeRequest', (event) => {
    wsRequestIds.add(event.requestId);
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        role,
        event: 'webSocketWillSendHandshakeRequest',
        requestId: event.requestId,
        url: event.request?.url,
        headers: event.request?.headers,
      }),
    );
  });

  cdp.on('Network.webSocketHandshakeResponseReceived', (event) => {
    wsRequestIds.add(event.requestId);
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        role,
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
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        role,
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
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        role,
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
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        role,
        event: 'webSocketFrameError',
        requestId: event.requestId,
        errorMessage: event.errorMessage,
      }),
    );
  });

  cdp.on('Network.webSocketClosed', (event) => {
    wsRequestIds.add(event.requestId);
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        role,
        event: 'webSocketClosed',
        requestId: event.requestId,
      }),
    );
  });

  cdp.on('Network.loadingFailed', (event) => {
    if (!wsRequestIds.has(event.requestId)) {
      return;
    }
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        epochMs: Date.now(),
        role,
        event: 'loadingFailed',
        requestId: event.requestId,
        errorText: event.errorText,
        canceled: event.canceled,
        blockedReason: event.blockedReason,
      }),
    );
  });
}

async function openExistingConversation(config, observations, page, role, seedText) {
  await page.goto(config.baseUrl, { waitUntil: 'domcontentloaded', timeout: config.navigationTimeoutMs });
  await page.waitForLoadState('networkidle', { timeout: config.navigationTimeoutMs }).catch(() => {});
  await waitForCondition(
    () => websocketHandshakeStatuses(observations.ws, role).includes(101),
    config.handshakeTimeoutMs,
    `${role} business WebSocket (/ws) did not observe a 101 Switching Protocols handshake`,
  );
  await waitForCondition(
    () => pageBodyContains(page, seedText),
    config.navigationTimeoutMs,
    `${role} did not display the seeded conversation before opening it`,
  );
  await takeScreenshot(page, config, observations, `${role.toLowerCase()}-conversation-list-seeded.png`);
  await page.getByText(seedText).first().click({ timeout: config.navigationTimeoutMs });
  await waitForCondition(
    () => pageBodyContains(page, seedText),
    config.navigationTimeoutMs,
    `${role} did not display the seeded conversation after opening it`,
  );
}

async function sendFromCurrentConversationUi(config, observations, page, content) {
  const input = page.getByRole('textbox', { name: '输入消息' });
  const sendButton = page.getByRole('button', { name: /^发送$/ });
  const sendingButton = page.getByRole('button', { name: /发送中/ });

  const inputVisible = await input.isVisible().catch(() => false);
  const inputEnabled = inputVisible ? await input.isEnabled().catch(() => false) : false;
  if (!inputVisible || !inputEnabled) {
    return {
      classification: CLASSIFICATION.REVERSE_UI_DISABLED_OR_MISSING_TARGET,
      reason: 'B reply input was missing or disabled before typing.',
      inputVisible,
      inputEnabled,
      sendButtonVisible: await sendButton.isVisible().catch(() => false),
      sendButtonEnabled: await sendButton.isEnabled().catch(() => false),
    };
  }

  await input.fill(content);
  const sendButtonVisible = await sendButton.isVisible().catch(() => false);
  const sendButtonEnabled = sendButtonVisible ? await sendButton.isEnabled().catch(() => false) : false;
  if (!sendButtonVisible || !sendButtonEnabled) {
    return {
      classification: CLASSIFICATION.REVERSE_UI_DISABLED_OR_MISSING_TARGET,
      reason: 'B send button was missing or disabled after typing.',
      inputVisible,
      inputEnabled,
      sendButtonVisible,
      sendButtonEnabled,
      sendingButtonVisible: await sendingButton.isVisible().catch(() => false),
    };
  }

  const responsePromise = page
    .waitForResponse((response) => {
      try {
        return new URL(response.url()).pathname === '/messages' && response.request().method() === 'POST';
      } catch {
        return false;
      }
    }, { timeout: config.reverseResponseTimeoutMs })
    .catch(() => null);

  await sendButton.click({ timeout: config.reverseResponseTimeoutMs });
  const response = await responsePromise;
  if (!response) {
    return {
      classification: CLASSIFICATION.REVERSE_UI_DISABLED_OR_MISSING_TARGET,
      reason: 'B send click did not issue POST /messages.',
      inputVisible,
      inputEnabled,
      sendButtonVisible,
      sendButtonEnabled,
    };
  }

  const responseBody = await response.text().catch(() => '');
  const envelope = parseJson(responseBody);
  return redactObject({
    httpStatus: response.status(),
    statusText: response.statusText(),
    envelopeCode: envelope?.code,
    envelopeMessage: envelope?.message,
    responseBody,
    requestPostData: response.request().postData(),
  });
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
    if (await predicate()) {
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

function summarizeRun({
  observations,
  aToBSendStartedAt,
  reverseSendStartedAt,
  bNoRefreshContainsAToB,
  bHistoryContainsAToB,
  aNoRefreshContainsReply = false,
  aHistoryContainsReply = false,
}) {
  const bWsEvents = businessWebSocketEvents(observations.ws, 'B');
  const aWsEvents = businessWebSocketEvents(observations.ws, 'A');
  const bIncomingFrames = bWsEvents.filter((event) => event.event === 'webSocketFrameReceived');
  const bIncomingFramesAfterAToB = bIncomingFrames.filter((event) => Number(event.epochMs ?? 0) >= aToBSendStartedAt);
  const bMatchedAToBFrames = bIncomingFramesAfterAToB.filter((event) => frameContainsText(event, observations.aToBText));
  const aIncomingFrames = aWsEvents.filter((event) => event.event === 'webSocketFrameReceived');
  const aIncomingFramesAfterReverse = reverseSendStartedAt
    ? aIncomingFrames.filter((event) => Number(event.epochMs ?? 0) >= reverseSendStartedAt)
    : [];
  const aMatchedReplyFrames = aIncomingFramesAfterReverse.filter((event) => frameContainsText(event, observations.bToAText));

  return {
    aWebSocketHandshakeStatuses: websocketHandshakeStatuses(observations.ws, 'A'),
    bWebSocketHandshakeStatuses: websocketHandshakeStatuses(observations.ws, 'B'),
    aWebSocketObserved101: websocketHandshakeStatuses(observations.ws, 'A').includes(101),
    bWebSocketObserved101: websocketHandshakeStatuses(observations.ws, 'B').includes(101),
    bIncomingFrameCountAfterAToB: bIncomingFramesAfterAToB.length,
    bMatchedIncomingFrameCountAfterAToB: bMatchedAToBFrames.length,
    bNoRefreshDisplayContainsAToB: bNoRefreshContainsAToB,
    bPullHistoryContainsAToB: bHistoryContainsAToB,
    reverseSendHttpStatus: observations.reverseSend?.httpStatus,
    reverseSendEnvelopeCode: observations.reverseSend?.envelopeCode,
    reverseSendEnvelopeMessage: observations.reverseSend?.envelopeMessage,
    reverseSendRequestPostData: observations.reverseSend?.requestPostData,
    reverseSendResponseBody: observations.reverseSend?.responseBody,
    aIncomingFrameCountAfterReverse: aIncomingFramesAfterReverse.length,
    aMatchedIncomingFrameCountAfterReverse: aMatchedReplyFrames.length,
    aNoRefreshDisplayContainsReply: aNoRefreshContainsReply,
    aPullHistoryContainsReply: aHistoryContainsReply,
  };
}

function classify(summary, reverseResult) {
  if (reverseResult?.classification === CLASSIFICATION.REVERSE_UI_DISABLED_OR_MISSING_TARGET) {
    return {
      classification: CLASSIFICATION.REVERSE_UI_DISABLED_OR_MISSING_TARGET,
      reason: reverseResult.reason,
    };
  }

  if (summary.reverseSendHttpStatus === 400) {
    return {
      classification: CLASSIFICATION.REVERSE_BAD_REQUEST,
      reason: 'B stayed on the same chat page, sent a reply, and POST /messages returned HTTP 400.',
    };
  }

  if (
    summary.reverseSendHttpStatus === 200 &&
    summary.reverseSendEnvelopeCode === 'OK' &&
    summary.aNoRefreshDisplayContainsReply &&
    summary.aPullHistoryContainsReply
  ) {
    return {
      classification: CLASSIFICATION.SUCCESS,
      reason: 'B replied from the unchanged chat page; A displayed and could pull the reply.',
    };
  }

  return {
    classification: CLASSIFICATION.SETUP_FAILED,
    reason:
      'The harness reached the reverse send step, but the response/display outcome did not match success, HTTP 400, or UI-disabled classifications.',
  };
}

function websocketHandshakeStatuses(wsEvents, role) {
  return businessWebSocketEvents(wsEvents, role)
    .filter((event) => event.event === 'webSocketHandshakeResponseReceived')
    .map((event) => Number(event.status))
    .filter((status) => Number.isFinite(status));
}

function businessWebSocketEvents(wsEvents, role) {
  const businessRequestIds = new Set(
    wsEvents
      .filter((event) => event.role === role && event.event === 'webSocketCreated' && isBusinessWebSocketUrl(event.url))
      .map((event) => event.requestId)
      .filter(Boolean),
  );
  return wsEvents.filter((event) => event.role === role && businessRequestIds.has(event.requestId));
}

function isBusinessWebSocketUrl(value) {
  if (typeof value !== 'string' || value.length === 0) {
    return false;
  }
  try {
    return new URL(value).pathname === '/ws';
  } catch {
    return false;
  }
}

function frameContainsText(event, text) {
  if (typeof event.payloadData === 'string' && event.payloadData.includes(text)) {
    return true;
  }
  return JSON.stringify(event.parsed ?? '').includes(text);
}

function historyContainsText(history, text) {
  return history.messages.some((message) => String(message.content ?? '').includes(text));
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

function publicSendResult(sendResult) {
  return redactObject({
    httpStatus: sendResult.httpStatus,
    clientMsgId: sendResult.clientMsgId,
    serverMsgId: sendResult.message?.serverMsgId,
    conversationId: sendResult.message?.conversationId,
    seq: sendResult.message?.seq,
    deduplicated: sendResult.deduplicated,
  });
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

async function writeArtifacts(config, observations) {
  await mkdir(config.outputDir, { recursive: true });
  await writeFile(path.join(config.outputDir, 'observations.redacted.json'), `${JSON.stringify(redactObject(observations), null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'network.redacted.json'), `${JSON.stringify(redactObject(observations.network), null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'console.redacted.json'), `${JSON.stringify(redactObject(observations.console), null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'ws-events.redacted.json'), `${JSON.stringify(redactObject(observations.ws), null, 2)}\n`);
  await writeFile(path.join(config.outputDir, 'report.txt'), reportText(config, observations));
}

function reportText(config, observations) {
  const summary = observations.summary ?? {};
  const lines = [
    'WebSocket bidirectional send E2E regression report',
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
    `seed text: ${observations.seedText ?? '(not reached)'}`,
    `A->B text: ${observations.aToBText ?? '(not reached)'}`,
    `B->A reply text: ${observations.bToAText ?? '(not reached)'}`,
    '',
    `B websocket handshake statuses: ${formatList(summary.bWebSocketHandshakeStatuses)}`,
    `A websocket handshake statuses: ${formatList(summary.aWebSocketHandshakeStatuses)}`,
    `B matched A->B incoming frame count: ${summary.bMatchedIncomingFrameCountAfterAToB ?? 0}`,
    `B no-refresh display contains A->B: ${summary.bNoRefreshDisplayContainsAToB ?? false}`,
    `B pull/history contains A->B: ${summary.bPullHistoryContainsAToB ?? false}`,
    `reverse /messages HTTP status: ${summary.reverseSendHttpStatus ?? observations.reverseSend?.httpStatus ?? '(none)'}`,
    `reverse /messages envelope code: ${summary.reverseSendEnvelopeCode ?? observations.reverseSend?.envelopeCode ?? '(none)'}`,
    `reverse /messages envelope message: ${summary.reverseSendEnvelopeMessage ?? observations.reverseSend?.envelopeMessage ?? '(none)'}`,
    `reverse /messages request body: ${summary.reverseSendRequestPostData ?? observations.reverseSend?.requestPostData ?? '(none)'}`,
    `reverse /messages response body: ${summary.reverseSendResponseBody ?? observations.reverseSend?.responseBody ?? '(none)'}`,
    `A matched B->A incoming frame count: ${summary.aMatchedIncomingFrameCountAfterReverse ?? 0}`,
    `A no-refresh display contains B->A reply: ${summary.aNoRefreshDisplayContainsReply ?? false}`,
    `A pull/history contains B->A reply: ${summary.aPullHistoryContainsReply ?? false}`,
    '',
    'Artifacts:',
    '- report.txt',
    '- observations.redacted.json',
    '- network.redacted.json',
    '- console.redacted.json',
    '- ws-events.redacted.json',
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
  if (classification === CLASSIFICATION.SUCCESS) {
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

function timestampForText(date) {
  return date.toISOString().replace(/[:.]/g, '-');
}

await main();
