#!/usr/bin/env node
import crypto from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { createRequire } from 'node:module';

const AUTH_STORAGE_KEY = 'agents_im.auth.v1';
const DEFAULT_OUTPUT_ROOT = '/tmp/agents-im-image-message-e2e';
const DEFAULT_PRODUCTION_BASE_URL = 'https://agenticim.xyz';
const DEFAULT_LOCAL_BASE_URL = 'http://127.0.0.1:5173';
const IMAGE_FILENAME = 'qa-image-message.png';
const IMAGE_LABEL = `图片 ${IMAGE_FILENAME}`;

const CLASSIFICATION = {
  SUCCESS: 'image-message-success',
  UPLOAD_VALIDATION_FAILED: 'image-upload-validation-failed',
  PREVIEW_DOWNLOAD_FAILED: 'image-preview-download-failed',
  SETUP_FAILED: 'setup-or-harness-failed',
};

const tinyPng = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lwQm7wAAAABJRU5ErkJggg==',
  'base64',
);
const secrets = new Set();

class HarnessError extends Error {
  constructor(message, details = {}, classification = CLASSIFICATION.SETUP_FAILED) {
    super(message);
    this.name = 'HarnessError';
    this.details = details;
    this.classification = classification;
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
      navigationTimeoutMs: config.navigationTimeoutMs,
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
    imageSend: null,
    download: null,
    conversationId: null,
    seedText: null,
    imageFilename: IMAGE_FILENAME,
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

    const runId = `${timestampForText(new Date())}-${shortId()}`;
    const conversationId = singleConversationId(accountA.userId, accountB.userId);
    const seedText = `image-message-seed-${runId}`;
    observations.conversationId = conversationId;
    observations.seedText = seedText;

    recordStep(observations, 'seed-existing-conversation-a-to-b', 'started');
    const seedSend = await sendTextMessage(config, observations, {
      label: 'A seeds image-message conversation with B',
      sender: accountA,
      receiver: accountB,
      content: seedText,
    });
    observations.seedSend = publicSendResult(seedSend);
    recordStep(observations, 'seed-existing-conversation-a-to-b', 'completed', observations.seedSend);

    browser = await launchBrowser(playwright, config);

    const aContext = await browser.newContext({ acceptDownloads: true });
    await installBrowserSession(aContext, accountA);
    aPage = await aContext.newPage();
    capturePageEvents('A', aPage, observations);
    await captureChromiumWebSocketEvents('A', aContext, aPage, observations);

    const bContext = await browser.newContext({ acceptDownloads: true });
    await installBrowserSession(bContext, accountB);
    bPage = await bContext.newPage();
    capturePageEvents('B', bPage, observations);
    await captureChromiumWebSocketEvents('B', bContext, bPage, observations);

    recordStep(observations, 'open-a-existing-conversation', 'started');
    await openExistingConversation(config, observations, aPage, 'A', [seedText]);
    recordStep(observations, 'open-a-existing-conversation', 'completed', {
      handshakeStatuses: websocketHandshakeStatuses(observations.ws, 'A'),
    });

    recordStep(observations, 'open-b-existing-conversation', 'started');
    await openExistingConversation(config, observations, bPage, 'B', [seedText]);
    recordStep(observations, 'open-b-existing-conversation', 'completed', {
      handshakeStatuses: websocketHandshakeStatuses(observations.ws, 'B'),
    });

    await takeScreenshot(aPage, config, observations, 'a-chat-before-image.png');
    await takeScreenshot(bPage, config, observations, 'b-chat-before-image.png');

    recordStep(observations, 'reject-unsupported-image-before-upload', 'started');
    await assertImageValidationRejectsBeforeUpload(aPage, observations, {
      filename: 'notes.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('not an image'),
      expectedStatus: '请选择 JPG、PNG、WebP 或 GIF 图片',
    });
    recordStep(observations, 'reject-unsupported-image-before-upload', 'completed');

    recordStep(observations, 'reject-oversized-image-before-upload', 'started');
    await assertImageValidationRejectsBeforeUpload(aPage, observations, {
      filename: 'too-large.jpg',
      mimeType: 'image/jpeg',
      buffer: Buffer.alloc(15 * 1024 * 1024 + 1),
      expectedStatus: '图片不能超过 15 MiB',
    });
    recordStep(observations, 'reject-oversized-image-before-upload', 'completed');

    recordStep(observations, 'send-valid-image-through-ui', 'started');
    const imageSend = await sendImageFromUi(config, observations, aPage);
    observations.imageSend = redactObject(imageSend);
    recordStep(observations, 'send-valid-image-through-ui', 'completed', observations.imageSend);

    recordStep(observations, 'sender-renders-image-bubble', 'started');
    await waitForImageBubble(aPage, IMAGE_LABEL, config.livePushWaitMs);
    recordStep(observations, 'sender-renders-image-bubble', 'completed');

    recordStep(observations, 'receiver-renders-live-image-without-refresh', 'started');
    await waitForImageBubble(bPage, IMAGE_LABEL, config.livePushWaitMs);
    await takeScreenshot(bPage, config, observations, 'b-chat-after-live-image.png');
    recordStep(observations, 'receiver-renders-live-image-without-refresh', 'completed');

    recordStep(observations, 'receiver-history-renders-image-after-reload', 'started');
    await bPage.reload({ waitUntil: 'domcontentloaded', timeout: config.navigationTimeoutMs });
    await bPage.waitForLoadState('networkidle', { timeout: config.navigationTimeoutMs }).catch(() => {});
    await openExistingConversation(config, observations, bPage, 'B', [IMAGE_LABEL, seedText]);
    await waitForImageBubble(bPage, IMAGE_LABEL, config.livePushWaitMs);
    await takeScreenshot(bPage, config, observations, 'b-chat-history-image.png');
    recordStep(observations, 'receiver-history-renders-image-after-reload', 'completed');

    recordStep(observations, 'preview-open-close', 'started');
    await openAndClosePreview(bPage);
    recordStep(observations, 'preview-open-close', 'completed');

    recordStep(observations, 'authorized-download-url-works-for-receiver', 'started');
    const downloadResult = await verifyDownload(config, observations, bPage, accountB, imageSend.mediaId);
    observations.download = redactObject(downloadResult);
    recordStep(observations, 'authorized-download-url-works-for-receiver', 'completed', observations.download);

    const aBody = await bodyText(aPage);
    const bBody = await bodyText(bPage);
    observations.summary = {
      validationRejectedBeforeUpload: true,
      senderSawImage: true,
      receiverSawLiveImage: true,
      receiverHistorySawImage: true,
      previewOpenedAndClosed: true,
      downloadUrlWorked: true,
      noRawInternalIdsInVisibleText: !aBody.includes(accountB.userId) && !bBody.includes(accountA.userId),
    };
    classification = CLASSIFICATION.SUCCESS;
    reason = 'image message upload, live receive, history render, preview, and download URL all passed';
  } catch (error) {
    classification = error?.classification ?? CLASSIFICATION.SETUP_FAILED;
    reason = errorMessage(error);
    observations.error = redactObject({
      name: error?.name,
      message: errorMessage(error),
      details: error?.details,
      stack: error?.stack,
    });
    if (aPage) {
      await takeScreenshot(aPage, config, observations, 'a-failure.png').catch(() => {});
    }
    if (bPage) {
      await takeScreenshot(bPage, config, observations, 'b-failure.png').catch(() => {});
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
        '  NODE_PATH=/tmp/ws-e2e-run/node_modules node tests/e2e/image_message_regression.mjs',
        'If the browser binary is missing, also run:',
        '  NODE_PATH=/tmp/ws-e2e-run/node_modules npx --prefix /tmp/ws-e2e-run playwright install chromium',
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
  const identifier = `qaimg_${role.toLowerCase()}_${suffix}`.slice(0, 32);
  const displayName = `QA IMG ${role} ${suffix}`;
  const password = `QaImg-${suffix}-${crypto.randomBytes(8).toString('hex')}`;
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
  });

  const data = response.data;
  if (!data?.user_id || !data?.identifier || !data?.token) {
    throw new HarnessError(`register account ${role} returned an invalid auth payload`);
  }
  addSecret(String(data.token));
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

async function sendTextMessage(config, observations, { label, sender, receiver, content }) {
  const clientMsgId = `qa-img-seed-${Date.now()}-${shortId(8)}`;
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
  page.on('request', (request) => {
    const pathname = safePathname(request.url());
    if (pathname === '/media/uploads' && request.method() === 'POST') {
      observations.network.push(
        redactObject({
          at: new Date().toISOString(),
          role,
          event: 'media-upload-intent-request',
          url: request.url(),
          method: request.method(),
          postData: request.postData(),
        }),
      );
    }
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
    const pathname = safePathname(response.url());
    if (
      !(
        (pathname === '/messages' && response.request().method() === 'POST') ||
        (pathname.includes('/download-url') && response.request().method() === 'GET')
      )
    ) {
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
        event: pathname === '/messages' ? 'post-messages-response' : 'media-download-url-response',
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
  await cdp.send('Network.enable');

  cdp.on('Network.webSocketCreated', (event) => {
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

  cdp.on('Network.webSocketHandshakeResponseReceived', (event) => {
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        role,
        event: 'webSocketHandshakeResponseReceived',
        requestId: event.requestId,
        url: event.response?.url,
        status: event.response?.status,
        statusText: event.response?.statusText,
      }),
    );
  });

  cdp.on('Network.webSocketFrameReceived', (event) => {
    observations.ws.push(
      redactObject({
        at: new Date().toISOString(),
        role,
        event: 'webSocketFrameReceived',
        requestId: event.requestId,
        payloadData: event.response?.payloadData,
      }),
    );
  });
}

async function openExistingConversation(config, observations, page, role, labels) {
  await page.goto(config.baseUrl, { waitUntil: 'domcontentloaded', timeout: config.navigationTimeoutMs });
  await page.waitForLoadState('networkidle', { timeout: config.navigationTimeoutMs }).catch(() => {});
  const row = await firstVisibleButtonWithAnyText(page, labels, config.navigationTimeoutMs);
  if (!row) {
    throw new HarnessError(`${role} could not find an existing conversation row for ${labels.join(' or ')}`);
  }
  await row.click();
  await page.getByRole('log', { name: '聊天消息' }).waitFor({ timeout: config.navigationTimeoutMs });
}

async function firstVisibleButtonWithAnyText(page, labels, timeoutMs) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    for (const label of labels) {
      const row = page.getByRole('button').filter({ hasText: label }).first();
      if ((await row.count()) > 0 && (await row.isVisible().catch(() => false))) {
        return row;
      }
    }
    await page.waitForTimeout(250);
  }
  return null;
}

async function assertImageValidationRejectsBeforeUpload(page, observations, payload) {
  const before = mediaUploadIntentRequestCount(observations, 'A');
  await page.locator('input[aria-label="发送图片"]').setInputFiles({
    name: payload.filename,
    mimeType: payload.mimeType,
    buffer: payload.buffer,
  });
  await page.getByRole('status').filter({ hasText: payload.expectedStatus }).waitFor({ timeout: 5_000 });
  await page.waitForTimeout(750);
  const after = mediaUploadIntentRequestCount(observations, 'A');
  if (after !== before) {
    throw new HarnessError(
      `${payload.filename} triggered a media upload intent instead of being rejected before upload`,
      { before, after, expectedStatus: payload.expectedStatus },
      CLASSIFICATION.UPLOAD_VALIDATION_FAILED,
    );
  }
}

async function sendImageFromUi(config, observations, page) {
  const responsePromise = page.waitForResponse(
    (response) => safePathname(response.url()) === '/messages' && response.request().method() === 'POST',
    { timeout: config.requestTimeoutMs * 3 },
  );

  await page.locator('input[aria-label="发送图片"]').setInputFiles({
    name: IMAGE_FILENAME,
    mimeType: 'image/png',
    buffer: tinyPng,
  });

  const response = await responsePromise;
  const responseText = await response.text();
  const envelope = parseJson(responseText);
  if (!response.ok() || envelope?.code !== 'OK') {
    throw new HarnessError(
      `image send returned ${response.status()} ${envelope?.code ?? ''} ${envelope?.message ?? ''}`.trim(),
      { httpStatus: response.status(), envelope },
      CLASSIFICATION.PREVIEW_DOWNLOAD_FAILED,
    );
  }

  const message = envelope?.data?.message;
  const content = parseJson(String(message?.content ?? ''));
  const mediaId = String(content?.mediaId ?? '');
  if (!mediaId) {
    throw new HarnessError('image send response did not contain content.mediaId', { message }, CLASSIFICATION.PREVIEW_DOWNLOAD_FAILED);
  }
  return {
    httpStatus: response.status(),
    serverMsgId: message?.serverMsgId,
    conversationId: message?.conversationId,
    seq: message?.seq,
    mediaId,
    filename: content?.filename,
    contentType: content?.contentType,
    sizeBytes: content?.sizeBytes,
  };
}

async function waitForImageBubble(page, label, timeoutMs) {
  await page.getByRole('img', { name: label }).first().waitFor({ timeout: timeoutMs });
}

async function openAndClosePreview(page) {
  await page.getByRole('button', { name: `预览${IMAGE_LABEL}` }).first().click();
  const dialog = page.getByRole('dialog', { name: '图片预览' });
  await dialog.waitFor({ timeout: 5_000 });
  await dialog.getByRole('img', { name: `预览${IMAGE_LABEL}` }).waitFor({ timeout: 5_000 });
  await dialog.getByRole('button', { name: '关闭预览' }).click();
  await dialog.waitFor({ state: 'detached', timeout: 5_000 });
}

async function verifyDownload(config, observations, page, account, mediaId) {
  if (!mediaId) {
    throw new HarnessError('mediaId is required for download verification', {}, CLASSIFICATION.PREVIEW_DOWNLOAD_FAILED);
  }

  await page.getByRole('button', { name: `预览${IMAGE_LABEL}` }).first().click();
  const dialog = page.getByRole('dialog', { name: '图片预览' });
  await dialog.waitFor({ timeout: 5_000 });

  const downloadUrlResponsePromise = page
    .waitForResponse(
      (response) =>
        safePathname(response.url()) === `/media/${mediaId}/download-url` && response.request().method() === 'GET',
      { timeout: config.requestTimeoutMs },
    )
    .catch(() => null);
  const popupPromise = page.waitForEvent('popup', { timeout: 2_000 }).catch(() => null);
  const downloadPromise = page.waitForEvent('download', { timeout: 2_000 }).catch(() => null);

  await dialog.getByRole('button', { name: `下载${IMAGE_LABEL}` }).click();
  const downloadUrlResponse = await downloadUrlResponsePromise;
  const popup = await popupPromise;
  const browserDownload = await downloadPromise;
  if (popup) {
    await popup.close().catch(() => {});
  }
  if (!downloadUrlResponse || !downloadUrlResponse.ok()) {
    throw new HarnessError(
      'preview download action did not fetch an authorized media download URL successfully',
      { status: downloadUrlResponse?.status() },
      CLASSIFICATION.PREVIEW_DOWNLOAD_FAILED,
    );
  }

  const response = await apiRequest(config, observations, {
    label: 'B requests authorized image download URL',
    method: 'GET',
    path: `/media/${encodeURIComponent(mediaId)}/download-url`,
    token: account.token,
  });
  const downloadUrl = String(response.data?.downloadUrl ?? '');
  if (!downloadUrl) {
    throw new HarnessError('download-url response did not include downloadUrl', { mediaId }, CLASSIFICATION.PREVIEW_DOWNLOAD_FAILED);
  }

  const objectResponse = await fetchWithTimeout(downloadUrl, { method: 'GET' }, config.requestTimeoutMs);
  const bytes = await objectResponse.arrayBuffer();
  if (!objectResponse.ok || bytes.byteLength <= 0) {
    throw new HarnessError(
      'presigned image download URL did not return image bytes',
      { status: objectResponse.status, byteLength: bytes.byteLength },
      CLASSIFICATION.PREVIEW_DOWNLOAD_FAILED,
    );
  }

  return {
    mediaId,
    browserDownloadSuggestedFilename: browserDownload ? await browserDownload.suggestedFilename() : null,
    downloadUrlStatus: response.status,
    objectStatus: objectResponse.status,
    byteLength: bytes.byteLength,
  };
}

async function apiRequest(config, observations, request) {
  const url = buildUrl(config.apiBaseUrl, request.path);
  const startedAt = Date.now();
  const headers = { Accept: 'application/json' };
  if (request.body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }
  if (request.token) {
    headers.Authorization = `Bearer ${request.token}`;
  }

  let response;
  let responseText = '';
  try {
    response = await fetchWithTimeout(
      url,
      {
        method: request.method,
        headers,
        body: request.body === undefined ? undefined : JSON.stringify(request.body),
      },
      config.requestTimeoutMs,
    );
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
      request: {
        headers,
        body: request.body,
      },
      response: envelope ?? responseText,
      durationMs: Date.now() - startedAt,
    }),
  );

  if (!response.ok || envelope?.code !== 'OK') {
    throw new HarnessError(
      `${request.label} returned ${response.status} ${envelope?.code ?? ''} ${envelope?.message ?? ''}`.trim(),
      { status: response.status, envelope },
    );
  }
  return {
    status: response.status,
    data: envelope.data,
    envelope,
  };
}

async function fetchWithTimeout(url, options, timeoutMs) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, {
      ...options,
      signal: controller.signal,
    });
  } finally {
    clearTimeout(timeout);
  }
}

function buildUrl(baseUrl, requestPath) {
  return new URL(requestPath, `${baseUrl}/`).toString();
}

function safePathname(value) {
  try {
    return new URL(value).pathname;
  } catch {
    return '';
  }
}

function mediaUploadIntentRequestCount(observations, role) {
  return observations.network.filter((item) => item.role === role && item.event === 'media-upload-intent-request').length;
}

function singleConversationId(userA, userB) {
  return ['single', ...[userA, userB].sort()].join(':');
}

function publicAccount(account) {
  return {
    role: account.role,
    userId: account.userId,
    identifier: account.identifier,
    displayName: account.displayName,
    expiresAt: account.expiresAt,
  };
}

function publicSendResult(result) {
  return redactObject({
    httpStatus: result.httpStatus,
    clientMsgId: result.clientMsgId,
    serverMsgId: result.message?.serverMsgId,
    conversationId: result.message?.conversationId,
    seq: result.message?.seq,
    deduplicated: result.deduplicated,
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

async function takeScreenshot(page, config, observations, filename) {
  const filePath = path.join(config.outputDir, filename);
  await page.screenshot({ path: filePath, fullPage: true });
  observations.screenshots[filename] = filePath;
}

async function writeArtifacts(config, observations) {
  await writeFile(path.join(config.outputDir, 'observations.json'), `${JSON.stringify(redactObject(observations), null, 2)}\n`);
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

function websocketHandshakeStatuses(events, role) {
  return events
    .filter((event) => event.role === role && event.event === 'webSocketHandshakeResponseReceived')
    .map((event) => event.status)
    .filter((status) => status !== undefined);
}

async function bodyText(page) {
  return page.locator('body').innerText({ timeout: 5_000 }).catch(() => '');
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
    if (/token|password|authorization|cookie|set-cookie|jwt|secret|signature|credential/i.test(key)) {
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
  next = next.replace(/\bBearer\s+[A-Za-z0-9._~+/=-]+/gi, 'Bearer [REDACTED]');
  next = next.replace(/\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g, '[REDACTED]');
  next = next.replace(/([?&](?:token|X-Amz-Signature|X-Amz-Credential|X-Amz-Security-Token|Signature|Expires)=)[^&\s"'<>)]*/gi, '$1[REDACTED]');
  next = next.replace(/(https?:\/\/[^\s"'<>)]*\?)[^\s"'<>)]*/gi, '$1[REDACTED]');
  next = next.replace(/("?(?:token|password|authorization|cookie|jwt|secret|signature|credential)"?\s*[:=]\s*)("[^"]*"|'[^']*'|[^,\s}]+)/gi, '$1[REDACTED]');
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

function timestampForText(date) {
  return date.toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
}

function timestampForPath(date) {
  return date.toISOString().replace(/[:.]/g, '-');
}

main().catch((error) => {
  process.stderr.write(`${errorMessage(error)}\n`);
  process.exitCode = 1;
});
