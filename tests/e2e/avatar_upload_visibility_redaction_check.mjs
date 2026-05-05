#!/usr/bin/env node
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const harnessPath = path.join(__dirname, 'avatar_upload_visibility_regression.mjs');
const source = await readFile(harnessPath, 'utf8');
const start = source.indexOf('const secrets = new Set();');
const end = source.indexOf('function recordStep');

if (start < 0 || end < 0 || end <= start) {
  throw new Error('could not isolate avatar harness redaction helpers');
}

const helpers = new Function(`${source.slice(start, end)}; return { redactObject };`)();
const redacted = JSON.stringify(
  helpers.redactObject({
    userId: '1001',
    receiverId: '2002',
    expectedFriendId: '2003',
    mediaId: 'med_avatar_1',
    expectedMediaId: 'med_avatar_2',
    avatar_media_id: 'med_avatar_3',
    conversationId: 'single:1001:2002',
    serverMsgId: 'srv_123',
    path: '/friends/requests/1001/accept',
    mediaPath: '/media/uploads/med_avatar_1/complete',
    uploadUrl: 'https://storage.example.com/upload?X-Amz-Signature=abc123',
  }),
);

for (const forbidden of ['1001', '2002', '2003', 'med_avatar_1', 'med_avatar_2', 'med_avatar_3', 'single:1001:2002', 'srv_123', 'abc123']) {
  assert.equal(redacted.includes(forbidden), false, `redaction output leaked ${forbidden}: ${redacted}`);
}

assert.match(redacted, /\[REDACTED\]/);
