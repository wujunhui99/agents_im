#!/usr/bin/env node
// Email registration E2E: request a code -> captured by the local mock SES ->
// register with that code -> login. Exercises the real auth-rpc -> resource-rpc
// (mail) -> SES client path; only the final HTTPS call is mocked.
//
// Prerequisites:
//   MOCK_SES=1 scripts/dev-up.sh --services-only
//
// Usage:
//   node tests/e2e/email_register_login_regression.mjs
//   AGENTS_IM_E2E_BASE_URL=http://127.0.0.1:8080 \
//   AGENTS_IM_MOCK_SES_DIR=.dev/mock-ses \
//   node tests/e2e/email_register_login_regression.mjs

import crypto from 'node:crypto';
import { readFile, mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const config = {
  baseUrl: process.env.AGENTS_IM_E2E_BASE_URL ?? 'http://127.0.0.1:8080',
  mockDir: process.env.AGENTS_IM_MOCK_SES_DIR ?? '.dev/mock-ses',
  outputDir: process.env.AGENTS_IM_E2E_OUTPUT_DIR ?? `/tmp/agents-im-email-register-e2e/${Date.now()}`,
  timeoutMs: Number(process.env.AGENTS_IM_E2E_TIMEOUT_MS ?? 20000),
};

const steps = [];
function record(name, status, detail = {}) {
  steps.push({ name, status, at: new Date().toISOString(), ...detail });
}

async function postJson(endpoint, body) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), config.timeoutMs);
  try {
    const res = await fetch(new URL(endpoint, config.baseUrl), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(body),
      signal: controller.signal,
    });
    const text = await res.text();
    let json = null;
    try { json = text ? JSON.parse(text) : null; } catch { /* keep raw */ }
    return { status: res.status, body: json, raw: text };
  } finally {
    clearTimeout(timer);
  }
}

function ok(result) {
  return result.status >= 200 && result.status < 300 && result.body?.code === 'OK';
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** Poll the mock SES capture file for the verification code sent to `email`. */
async function waitForCode(email) {
  const file = path.join(config.mockDir, 'requests.jsonl');
  const deadline = Date.now() + config.timeoutMs;
  while (Date.now() < deadline) {
    let raw = '';
    try { raw = await readFile(file, 'utf8'); } catch { /* not yet */ }
    for (const line of raw.split('\n').filter(Boolean).reverse()) {
      let payload;
      try { payload = JSON.parse(line); } catch { continue; }
      const destinations = payload?.Destination ?? [];
      if (!destinations.some((d) => String(d).toLowerCase() === email.toLowerCase())) continue;
      const templateData = payload?.Template?.TemplateData;
      if (!TemplateDataHasCode(templateData)) continue;
      const parsed = JSON.parse(templateData);
      const code = parsed.code ?? parsed.Code ?? parsed.verification_code;
      if (code) return String(code);
    }
    await sleep(300);
  }
  throw new Error(`verification code for ${email} not captured in ${file}`);
}

function TemplateDataHasCode(value) {
  return typeof value === 'string' && value.includes('code');
}

async function main() {
  await mkdir(config.outputDir, { recursive: true });
  const suffix = `${Date.now().toString(36)}_${crypto.randomBytes(3).toString('hex')}`;
  const account = {
    identifier: `e2e_mail_${suffix}`,
    email: `e2e_${suffix}@example.test`,
    password: `QaMail-${suffix}-${crypto.randomBytes(6).toString('hex')}`,
    displayName: `E2E Mail ${suffix}`,
  };

  let classification = 'setup-or-harness-failed';
  let reason = '';

  try {
    record('request-registration-code', 'started', { email: account.email });
    const codeResult = await postJson('/auth/register/email-code', { email: account.email });
    if (!ok(codeResult)) throw new Error(`request code failed: status=${codeResult.status} body=${codeResult.raw.slice(0, 200)}`);
    record('request-registration-code', 'completed');

    const code = await waitForCode(account.email);
    record('capture-code-from-mock-ses', 'completed', { codeLength: code.length });

    const registerResult = await postJson('/auth/register', {
      identifier: account.identifier,
      email: account.email,
      email_verification_code: code,
      password: account.password,
      display_name: account.displayName,
    });
    if (!ok(registerResult)) throw new Error(`register failed: status=${registerResult.status} body=${registerResult.raw.slice(0, 300)}`);
    record('register-with-email-code', 'completed', { userId: registerResult.body?.data?.user_id });

    const loginResult = await postJson('/auth/login', { identifier: account.identifier, password: account.password });
    if (!ok(loginResult)) throw new Error(`login failed: status=${loginResult.status} body=${loginResult.raw.slice(0, 300)}`);
    const token = loginResult.body?.data?.token ?? '';
    if (!token) throw new Error('login succeeded without token');
    record('login-with-email-registered-account', 'completed');

    classification = 'email-register-login-success';
  } catch (error) {
    classification = 'email-register-login-failed';
    reason = error instanceof Error ? error.message : String(error);
  }

  const report = {
    classification,
    reason,
    account: { identifier: account.identifier, email: account.email },
    config: { baseUrl: config.baseUrl, mockDir: config.mockDir },
    steps,
  };
  await writeFile(path.join(config.outputDir, 'report.json'), JSON.stringify(report, null, 2));
  console.log(`classification=${classification}`);
  if (reason) console.log(`reason=${reason}`);
  console.log(`artifacts=${config.outputDir}`);
  process.exit(classification === 'email-register-login-success' ? 0 : 1);
}

main();
