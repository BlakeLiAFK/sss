// Real Chromium smoke/regression test. All state lives in an isolated temporary directory.
// Run: cd web && npx playwright install chromium && npm run test:e2e
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { mkdtemp, mkdir, cp, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { randomBytes } from 'node:crypto';
import { createServer } from 'node:net';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(join(root, 'web/package.json'));
const { chromium } = require('playwright');
const work = await mkdtemp(join(tmpdir(), 'sss-browser-e2e-'));
const evidence = process.env.E2E_ARTIFACT_DIR ? resolve(process.env.E2E_ARTIFACT_DIR) : join(work, 'evidence');
await mkdir(evidence, { recursive: true });
const steps = [], errors = [];
let server, browser, page, serverLog = '';
function command(cmd, args, cwd = root) {
  const result = spawnSync(cmd, args, { cwd, encoding: 'utf8', stdio: 'pipe' });
  if (result.status !== 0) throw new Error(`${cmd} ${args.join(' ')} failed:\n${result.stdout}\n${result.stderr}`);
}
async function step(name, fn) {
  await fn(); steps.push(name); console.log(`PASS ${name}`);
}
async function freePort() {
  const s = createServer(); await new Promise(r => s.listen(0, '127.0.0.1', r));
  const port = s.address().port; await new Promise(r => s.close(r)); return port;
}
try {
  if (!process.env.E2E_SKIP_BUILD) command('npm', ['run', 'build'], join(root, 'web'));
  const binary = join(work, process.platform === 'win32' ? 'sss.exe' : 'sss');
  command('go', ['build', '-o', binary, './cmd/server']);
  await mkdir(join(work, 'data'), { recursive: true });
  await cp(join(root, 'data/static'), join(work, 'data/static'), { recursive: true });
  const port = await freePort(), base = `http://127.0.0.1:${port}`;
  server = spawn(binary, ['-host', '127.0.0.1', '-port', String(port), '-db', join(work, 'metadata.db'), '-data', join(work, 'buckets')], { cwd: work });
  for (const stream of [server.stdout, server.stderr]) stream.on('data', b => { serverLog += b.toString(); });
  for (let i = 0; ; i++) {
    try { if ((await fetch(`${base}/api/health`)).ok) break; } catch {}
    if (i >= 100 || server.exitCode !== null) throw new Error(`Server failed to start: ${serverLog}`);
    await new Promise(r => setTimeout(r, 100));
  }
  browser = await chromium.launch({ headless: true, ...(process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH } : {}) });
  const context = await browser.newContext({ locale: 'en-US', viewport: { width: 1440, height: 1000 }, acceptDownloads: true });
  await context.tracing.start({ screenshots: true, snapshots: true });
  page = await context.newPage(); page.setDefaultTimeout(15000);
  page.on('pageerror', e => errors.push(e.message));
  const username = `e2e${randomBytes(4).toString('hex')}`, password = `Aa1!${randomBytes(24).toString('base64url')}`;
  const bucket = `e2e-${randomBytes(5).toString('hex')}`, contents = `isolated browser fixture ${randomBytes(12).toString('hex')}\n`;
  await step('first-run installation in browser', async () => {
    await page.goto(base); await page.waitForURL('**/#/setup');
    await page.getByRole('button', { name: 'Start Configuration', exact: true }).click();
    await page.locator('input').nth(0).fill(username);
    await page.locator('input[type=password]').nth(0).fill(password);
    await page.locator('input[type=password]').nth(1).fill(password);
    await page.getByRole('button', { name: 'Complete Installation', exact: true }).click();
    await page.getByRole('button', { name: 'Go to Login', exact: true }).click();
    await page.waitForURL('**/#/login');
  });
  async function login(pass) {
    await page.getByPlaceholder('Username', { exact: true }).fill(username);
    await page.getByPlaceholder('Password', { exact: true }).fill(pass);
    await page.getByRole('button', { name: 'Login', exact: true }).click();
    await page.waitForURL(`${base}/#/`);
  }
  await step('login and authenticated dashboard', () => login(password));
  await step('create private bucket', async () => {
    await page.goto(`${base}/#/buckets`);
    await page.getByRole('button', { name: 'Create', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: 'Create Bucket' });
    await dialog.locator('input').fill(bucket);
    await dialog.getByRole('button', { name: 'Create', exact: true }).click();
    await page.getByRole('link', { name: bucket, exact: true }).click();
    await page.waitForURL(`**/#/bucket/${bucket}`);
  });
  await step('upload and object list', async () => {
    await page.locator('input[type=file]').first().setInputFiles({ name: 'browser-proof.txt', mimeType: 'text/plain', buffer: Buffer.from(contents) });
    await page.getByRole('button', { name: 'Upload 1 files', exact: true }).click();
    await page.locator('.file-name').filter({ hasText: 'browser-proof.txt' }).waitFor();
  });
  await step('private object preview', async () => {
    await page.getByRole('button', { name: 'Preview', exact: true }).click();
    await page.locator('.preview-text').filter({ hasText: contents.trim() }).waitFor();
    await page.getByRole('dialog').locator('.el-dialog__headerbtn').click();
    await page.getByRole('dialog').waitFor({ state: 'hidden' });
  });
  await step('download exact uploaded bytes', async () => {
    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.locator('tbody').getByRole('button', { name: 'Download', exact: true }).click()
    ]);
    assert.equal(download.suggestedFilename(), 'browser-proof.txt');
    assert.equal(await readFile(await download.path(), 'utf8'), contents);
  });
  await step('ECharts rendering and Chinese/English locale switching', async () => {
    await page.goto(`${base}/#/`);
    await page.locator('.chart-container canvas').first().waitFor();
    assert.equal(await page.locator('.chart-container canvas').count(), 2);
    await page.locator('.el-dropdown button').click();
    await page.getByRole('menuitem').filter({ hasText: 'Chinese (Simplified)' }).click();
    await page.getByRole('button', { name: '退出', exact: true }).waitFor();
    await page.locator('.el-dropdown button').click();
    await page.getByRole('menuitem').filter({ hasText: 'English' }).click();
    await page.getByRole('button', { name: 'Logout', exact: true }).waitFor();
  });
  await step('save settings and persist after reload', async () => {
    await page.goto(`${base}/#/settings`);
    await page.getByRole('button', { name: 'Edit Settings', exact: true }).click();
    await page.getByPlaceholder('us-east-1', { exact: true }).fill('e2e-region-1');
    const saved = page.waitForResponse(r => r.url().endsWith('/api/admin/settings') && r.request().method() === 'PUT');
    await page.getByRole('button', { name: 'Save Settings', exact: true }).click();
    assert.equal((await saved).status(), 200);
    await page.reload();
    await page.getByPlaceholder('us-east-1', { exact: true }).waitFor();
    assert.equal(await page.getByPlaceholder('us-east-1', { exact: true }).inputValue(), 'e2e-region-1');
  });
  await step('logout and protected-route redirect', async () => {
    await page.getByRole('button', { name: 'Logout', exact: true }).click();
    await page.waitForURL('**/#/login');
    await page.goto(`${base}/#/settings`); await page.waitForURL('**/#/login');
    assert.equal((await fetch(`${base}/api/admin/settings`)).status, 401);
  });
  await step('recovery requires local authorization and discloses no absolute path', async () => {
    const response = await fetch(`${base}/api/setup/reset-password/check`);
    assert.deepEqual(await response.json(), { file_exists: false });
    await page.getByRole('button', { name: 'Forgot password?', exact: true }).click();
    await page.getByRole('button', { name: 'Check File', exact: true }).waitFor();
    assert.ok(!(await page.getByRole('dialog').innerText()).includes(work));
    assert.equal((await fetch(`${base}/api/setup/reset-password`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ new_password: 'UnauthorizedChange123!' }) })).status, 403);
    await writeFile(join(work, 'data/.reset_password'), '');
    await page.getByRole('button', { name: 'Check File', exact: true }).click();
    await page.getByPlaceholder('At least 8 characters, including uppercase, lowercase and a number', { exact: true }).waitFor();
  });
  const newPassword = `Aa1!${randomBytes(24).toString('base64url')}`;
  await step('recovery form preserves input on simulated server failure and retries', async () => {
    const dialog = page.getByRole('dialog', { name: 'Reset Admin Password', exact: true });
    await dialog.locator('input[type=password]').nth(0).fill(newPassword);
    await dialog.locator('input[type=password]').nth(1).fill(newPassword);
    // Only the first browser response is fault-injected. The retry reaches the real server.
    const resetURL = `${base}/api/setup/reset-password`;
    await page.route(resetURL, route => route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'Injected retryable failure' }) }), { times: 1 });
    const failed = page.waitForResponse(r => r.url() === resetURL && r.status() === 500);
    await dialog.getByRole('button', { name: 'Reset Admin Password', exact: true }).click();
    await failed;
    assert.equal(await dialog.locator('input[type=password]').nth(0).inputValue(), newPassword);
    const succeeded = page.waitForResponse(r => r.url() === resetURL && r.status() === 200);
    await dialog.getByRole('button', { name: 'Reset Admin Password', exact: true }).click();
    await succeeded;
    await dialog.waitFor({ state: 'hidden' });
    await login(newPassword);
    assert.deepEqual(await (await fetch(`${base}/api/setup/reset-password/check`)).json(), { file_exists: false });
    await page.getByRole('button', { name: 'Logout', exact: true }).click();
    await page.waitForURL('**/#/login');
  });
  assert.deepEqual(errors, [], 'browser runtime errors');
  await context.tracing.stop({ path: join(evidence, 'trace.zip') });
  await writeFile(join(evidence, 'result.json'), JSON.stringify({ status: 'passed', platform: process.platform, arch: process.arch, browser: await browser.version(), steps, runtimeErrors: errors }, null, 2));
  console.log(`Browser evidence: ${evidence}`);
} catch (e) {
  if (page) await page.screenshot({ path: join(evidence, 'failure.png'), fullPage: true }).catch(() => {});
  await writeFile(join(evidence, 'result.json'), JSON.stringify({ status: 'failed', steps, error: String(e), runtimeErrors: errors }, null, 2));
  console.error(e); console.error(`Failure evidence: ${evidence}`); process.exitCode = 1;
} finally {
  if (browser) await browser.close();
  if (server && server.exitCode === null) { server.kill('SIGTERM'); await new Promise(r => server.once('exit', r)); }
  // Server logs may include temporary credentials, so never publish them as artifacts.
  if (process.env.E2E_ARTIFACT_DIR && !process.env.E2E_KEEP_TEMP) await rm(work, { recursive: true, force: true });
}
