/* Optional browser integration check against an isolated real --demo service.
 * Build first: go build -o internal/webui/.qa/rf-button-manager ./cmd/rf-button-manager
 * Requires Playwright + Chromium. RFB_UI_BINARY and RFB_UI_BROWSER override paths.
 * Signals are synthetic; no hardware or webhook execution is tested here.
 */
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {spawn} = require('node:child_process');
const {setTimeout: delay} = require('node:timers/promises');
const {chromium} = require(require.resolve('playwright', {paths: [process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES || process.cwd(), process.cwd()]}));
const qa = path.join(__dirname, '.qa');
fs.mkdirSync(qa, {recursive: true});
const data = fs.mkdtempSync(path.join(qa, 'ui-state-'));
const binary = process.env.RFB_UI_BINARY || path.join(qa, 'rf-button-manager');
const browserPath = process.env.RFB_UI_BROWSER || (fs.existsSync(path.join(qa, 'chromium')) ? path.join(qa, 'chromium') : undefined);
const port = process.env.RFB_UI_PORT || '18788';
const base = `http://127.0.0.1:${port}`;
const server = spawn(binary, ['--demo', '--listen', `127.0.0.1:${port}`, '--data-dir', data], {stdio: ['ignore', 'pipe', 'pipe']});
let log = '';
server.stdout.on('data', b => { log += b; });
server.stderr.on('data', b => { log += b; });
let browser;
const checks = [];
async function check(name, fn) { await fn(); checks.push(name); console.log(`PASS ${name}`); }
async function visible(locator) { await locator.waitFor({state: 'visible', timeout: 20000}); }
async function hidden(locator) { await locator.waitFor({state: 'hidden', timeout: 20000}); }
(async () => {
  for (let i = 0; i < 50; i++) {
    try { if ((await fetch(`${base}/healthz`)).ok) break; } catch (_) {}
    if (server.exitCode !== null) throw new Error(`Service exited: ${log}`);
    await delay(100);
  }
  const token = fs.readFileSync(path.join(data, 'management.token'), 'utf8').trim();
  browser = await chromium.launch({headless:true, executablePath:browserPath, args:['--no-sandbox', '--disable-dev-shm-usage']});
  const context = await browser.newContext({viewport:{width:1440,height:1100}, acceptDownloads:true});
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  page.on('console', message => { if (message.type() === 'error' && /Content Security Policy|Refused to/i.test(message.text())) errors.push(message.text()); });
  await check('token login and explicit demo status', async () => {
    await page.goto(base);
    await visible(page.locator('#login-dialog'));
    await page.locator('#management-token').fill(token);
    await page.locator('#login-submit').click();
    await hidden(page.locator('#login-dialog'));
    await visible(page.locator('#demo-banner'));
    assert.match(await page.locator('#demo-banner').textContent(), /simulated/);
  });
  await check('create six-button remote and edit action', async () => {
    await page.locator('#remote-create-empty').click();
    await page.locator('#remote-submit').click();
    await hidden(page.locator('#remote-dialog'));
    assert.equal(await page.locator('.rf-button').count(), 6);
    await page.locator('.rf-button').first().click();
    await visible(page.locator('#button-editor'));
    assert.equal(await page.locator('#button-name').inputValue(), 'A ON');
    await page.locator('#webhook-url').fill('http://127.0.0.1:8123/api/webhook/demo-only');
    await page.locator('#webhook-payload').fill('{"source":"rf-demo"}');
    await page.locator('#button-save').click();
    await page.waitForFunction(() => document.getElementById('unsaved-status').textContent === '');
    assert.equal(await page.locator('#webhook-test').isDisabled(), true, 'Demo must disable real action test');
  });
  await check('polling preserves unsaved button edits', async () => {
    await page.locator('#button-name').fill('A ON edited');
    await delay(1500);
    assert.equal(await page.locator('#button-name').inputValue(), 'A ON edited');
    assert.equal(await page.locator('#learn-start').isDisabled(), true);
    await page.locator('#button-save').click();
    await page.waitForFunction(() => document.getElementById('unsaved-status').textContent === '');
  });
  await check('three-press learning with explicit arm and confirm', async () => {
    await page.locator('#learn-start').click();
    await visible(page.locator('#learning-dialog'));
    await visible(page.locator('#learning-arm'));
    await delay(1100);
    await page.locator('#learning-arm').click();
    await hidden(page.locator('#learning-arm'));
    await visible(page.locator('#learning-arm'));
    await delay(1100);
    await page.locator('#learning-arm').click();
    await visible(page.locator('#learning-confirm'));
    await page.locator('#learning-confirm').click();
    await hidden(page.locator('#learning-dialog'));
    assert.equal((await page.locator('#mapping-status').textContent()).trim(), 'Signal learned');
    assert.equal(await page.locator('#button-enabled').isChecked(), false, 'Learned action remains disabled');
  });
  await check('HTML-like remote labels render as text', async () => {
    await page.locator('#button-name').fill('<img src=x onerror=alert(1)>');
    await page.locator('#button-save').click();
    await page.waitForFunction(() => document.getElementById('editor-heading').textContent.startsWith('<img'));
    assert.equal(await page.locator('#button-grid img').count(), 0);
    await page.locator('#button-name').fill('A ON');
    await page.locator('#button-save').click();
    await page.waitForFunction(() => document.getElementById('editor-heading').textContent === 'A ON');
  });
  await check('settings, backup download and restore disable actions', async () => {
    await page.locator('#settings-open').click();
    await visible(page.locator('#settings-dialog'));
    const downloading = page.waitForEvent('download');
    await page.locator('#backup-download').click();
    const download = await downloading;
    const content = JSON.parse(fs.readFileSync(await download.path(), 'utf8'));
    assert.equal(content.schema_version, 1);
    assert.equal(content.remotes[0].buttons.length, 6);
    content.remotes[0].buttons[0].enabled = true;
    await page.locator('#backup-file').setInputFiles({name:'ui-backup.json', mimeType:'application/json', buffer:Buffer.from(JSON.stringify(content))});
    await visible(page.locator('#confirm-dialog'));
    await page.locator('#confirm-yes').click();
    await hidden(page.locator('#settings-dialog'));
    await page.locator('.rf-button').first().click();
    assert.equal(await page.locator('#button-enabled').isChecked(), false);
  });
  await check('duplicate remote clears learned mapping', async () => {
    await page.locator('#remote-edit-open').click();
    await page.locator('#remote-duplicate').click();
    await hidden(page.locator('#remote-dialog'));
    assert.equal(await page.locator('.remote-nav').count(), 2);
    await page.locator('.rf-button').first().click();
    assert.equal((await page.locator('#mapping-status').textContent()).trim(), 'Not learned');
    await page.locator('.remote-nav').first().click();
    await page.locator('.rf-button').first().click();
  });
  await check('desktop and mobile layout; light and dark themes', async () => {
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    const screenshots = path.resolve(__dirname, '../../docs/screenshots');
    fs.mkdirSync(screenshots, {recursive:true});
    await hidden(page.locator('#toast'));
    await page.screenshot({path:path.join(screenshots,'dashboard.png'),fullPage:true});
    await page.locator('#theme-toggle').click();
    assert.equal(await page.locator('html').getAttribute('data-theme'), 'light');
    await page.setViewportSize({width:390,height:844});
    await delay(250);
    await visible(page.locator('#receiver-state'));
    assert.equal(await page.locator('.rf-button').nth(1).evaluate(el => getComputedStyle(el).backgroundColor), 'rgb(255, 255, 255)');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.screenshot({path:path.join(screenshots,'mobile.png'),fullPage:true});
    await page.locator('#theme-toggle').click();
  });
  await check('lock removes token and clears visible secrets', async () => {
    await page.locator('#logout').click();
    await visible(page.locator('#login-dialog'));
    assert.equal(await page.evaluate(() => sessionStorage.getItem('rfbm-token')), null);
    assert.equal(await page.locator('#webhook-url').inputValue(), '');
  });
  assert.deepEqual(errors, [], `Browser errors: ${errors.join('; ')}`);
  fs.writeFileSync(path.join(qa, 'ui-test-results.json'), JSON.stringify({mode:'actual local --demo service; synthetic RF only',checks,errors},null,2));
  console.log(`${checks.length} browser integration checks passed; no hardware or webhook delivery tested.`);
})().catch(error => { console.error(error); console.error(log); process.exitCode=1; }).finally(async () => {
  if (browser) await browser.close();
  server.kill('SIGTERM');
  await Promise.race([new Promise(resolve => server.once('exit',resolve)), delay(3000)]);
  if (server.exitCode === null) server.kill('SIGKILL');
  fs.rmSync(data,{recursive:true,force:true});
});
