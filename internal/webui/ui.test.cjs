/* Optional browser integration check against an isolated real --demo service.
 * Build first: go build -o internal/webui/.qa/rf433-button-manager ./cmd/rf433-button-manager
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
const binary = process.env.RFB_UI_BINARY || path.join(qa, 'rf433-button-manager');
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
  const context = await browser.newContext({viewport:{width:1440,height:1100}, acceptDownloads:true, hasTouch:true});
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
  await check('reception counters and sensitivity controls on desktop and touch layouts', async () => {
    // No host USB/sysfs is required. Settings persistence still uses the real API.
    const inventory = route => route.fulfill({json:{devices:[]}});
    await page.route('**/api/receivers', inventory);
    await page.locator('#view-remotes').click();
    await page.locator('.rf-button').first().click();
    await page.locator('#button-reception').evaluate(el=>{el.open=true;});
    await visible(page.locator('#reception-reset'));
    await page.locator('#reception-reset').click();
    await page.waitForFunction(()=>document.getElementById('reception-count').textContent==='0');
    assert.match(await page.locator('#reception-action').textContent(),/disabled/);
    assert.equal(await page.locator('#reception-signal').textContent(),'Not available','demo must not invent signal levels');
    await page.locator('#settings-open').click();
    assert.equal(await page.locator('#sensitivity-standard').isChecked(),true);
    await page.locator('#sensitivity-adaptive').check();
    await delay(1200);assert.equal(await page.locator('#sensitivity-adaptive').isChecked(),true);
    await page.locator('#settings-save').click();
    await hidden(page.locator('#settings-dialog'));
    const saved=await (await fetch(`${base}/api/backup`,{headers:{Authorization:`Bearer ${token}`}})).json();
    assert.equal(saved.settings.adaptive_sensitivity,true);
    await page.locator('#settings-open').click();assert.equal(await page.locator('#sensitivity-adaptive').isChecked(),true);
    await page.locator('#sensitivity-standard').check();await page.locator('#settings-save').click();
    await hidden(page.locator('#settings-dialog'));
    for(const size of [{width:1440,height:1100},{width:820,height:1180},{width:390,height:844},{width:320,height:740}]) {
      await page.setViewportSize(size);
      await page.locator('#view-remotes').click();
      // Phone navigation retains the selected editor and hides the remote grid.
      if (await page.locator('.rf-button').first().isVisible()) await page.locator('.rf-button').first().click();
      await page.locator('#button-reception').scrollIntoViewIfNeeded();
      assert.equal(await page.locator('#main-content').evaluate(el=>el.scrollWidth<=el.clientWidth+1),true);
      assert.ok((await page.locator('#reception-reset').boundingBox()).height>=48);
      if(size.width===390) await page.screenshot({path:path.join(qa,'reception-phone.png'),fullPage:true});
      if(size.width===1440) await page.screenshot({path:path.join(qa,'reception-desktop.png'),fullPage:true});
      await page.locator('#settings-open').click();
      await page.locator('#sensitivity-adaptive').scrollIntoViewIfNeeded();
      for(const choice of await page.locator('.sensitivity-choice').all()) {
        assert.ok((await choice.boundingBox()).height>=48);
        assert.equal(await choice.evaluate(el=>el.scrollWidth<=el.clientWidth+1),true);
      }
      assert.equal(await page.locator('#main-content').evaluate(el=>el.scrollWidth<=el.clientWidth+1),true);
      if(size.width===390) await page.screenshot({path:path.join(qa,'sensitivity-phone.png'),fullPage:true});
    }
    await page.setViewportSize({width:1440,height:1100});
    await page.locator('#view-remotes').click();
    await page.unroute('**/api/receivers', inventory);
  });
  await check('normal-use recording downloads evidence without pausing actions (synthetic demo)', async () => {
    const readState=async()=> (await fetch(`${base}/api/state`,{headers:{Authorization:`Bearer ${token}`}})).json();
    const before=await readState();
    await page.locator('#button-reception').evaluate(el=>{el.open=true;});
    await page.locator('#recording-start').click();
    await visible(page.locator('#recording-stop'));
    const during=await readState();
    assert.equal(during.reception_recording.active,true);
    assert.equal(during.actions_paused,false);
    assert.equal(during.receiver_generation,before.receiver_generation);
    assert.equal(during.learning,null);
    await page.waitForFunction(()=>/ [1-9]\d* decoded reports/.test(document.getElementById('recording-status').textContent));
    for(const size of [{width:1440,height:1100},{width:820,height:1180},{width:390,height:844},{width:320,height:740}]) {
      await page.setViewportSize(size);
      await page.locator('#recording-stop').scrollIntoViewIfNeeded();
      assert.ok((await page.locator('#recording-stop').boundingBox()).height>=48);
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
      if(size.width===390) await page.screenshot({path:path.join(qa,'recording-phone.png'),fullPage:true});
    }
    const downloadEvent=page.waitForEvent('download');
    await page.locator('#recording-stop').click();
    const download=await downloadEvent;
    assert.equal(download.suggestedFilename(),'rf433-button-manager-reception.json');
    const raw=fs.readFileSync(await download.path(),'utf8'), report=JSON.parse(raw);
    assert.equal(report.format,'rf433-button-manager-reception-v1');
    assert.equal(report.active,false);assert.equal(report.stop_reason,'stopped');
    assert.equal(report.demo,true);assert.equal(report.packet_count,0);
    assert.ok(report.decoded_count>=1);assert.ok(report.recognized_presses>=1);
    assert.ok(report.events.some(e=>e.kind==='decision' && e.reason==='recognized'));
    assert.ok(!raw.includes(token));assert.ok(!raw.includes('/api/webhook/demo-only'));
    assert.ok(!raw.includes('"source":"rf-demo"'));
    const after=await readState();
    assert.equal(after.actions_paused,false);
    assert.equal(after.receiver_generation,before.receiver_generation);
    await page.setViewportSize({width:1440,height:1100});
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
  await check('capture progress, diagnostics and retry controls (mocked RF states)', async () => {
    const baseState = await (await fetch(`${base}/api/state`, {headers:{Authorization:`Bearer ${token}`}})).json();
    const button = baseState.remotes[0].buttons[0];
    const session = {session_id:'browser-fixture', button_id:button.id, automatic:true, stage:'waiting', message:'Receiver ready. Press once.', presses:0, required:3, candidate:'', raw:null, identity_fields:['codes'], expires_at:new Date(Date.now()+180000).toISOString(), diagnostics:{packets:0,pulses:0,bytes:0,candidates:0,replays:0}};
    let mock = {...baseState, demo:false, learning:session, learning_notice:null, actions_paused:true};
    const stateRoute = route => route.fulfill({json:mock});
    const learnRoute = async route => {
      if (route.request().url().endsWith('/capture')) {
        await route.fulfill({json:{format:'rf433-button-manager-capture-v1',packets:[{modulation:'FSK',pulse_data:'synthetic fixture'}]}});return;
      }
      if (route.request().url().endsWith('/start')) {
        assert.equal(route.request().postDataJSON().button_id, button.id);
        mock = {...mock, learning:{...session}, learning_notice:null, actions_paused:true};
      } else {
        mock = {...mock, learning:null, learning_notice:null, actions_paused:false};
      }
      await route.fulfill({json:{ok:true}});
    };
    await page.route('**/api/state', stateRoute);
    await page.route('**/api/learn/*', learnRoute);
    try {
      await visible(page.locator('#learning-dialog'));
      await page.waitForFunction(() => document.getElementById('learning-instruction').textContent === 'Press the physical button once');
      mock = {...mock, learning:{...session, stage:'receiving', message:'Transmission detected. Release the button.', diagnostics:{packets:1,pulses:144,bytes:1441,candidates:0,replays:0}}};
      await page.waitForFunction(() => document.getElementById('learning-instruction').textContent === 'Transmission detected');
      await page.locator('#learning-diagnostic-panel summary').click();
      await visible(page.locator('#learning-diagnostics'));
      assert.match(await page.locator('#learning-diagnostics').textContent(), /pulses: 144/);
      mock = {...mock, learning:null, actions_paused:true, learning_notice:{...session, stage:'unsupported', message:'Transmission detected, but replay could not validate a supported fixed code.'}};
      await visible(page.locator('#learning-retry'));
      assert.equal(await page.locator('#learning-retry').isDisabled(), true, 'Retry waits for cleanup');
      await hidden(page.locator('#learning-confirm'));
      mock = {...mock, actions_paused:false, rf_capture_available:true};
      await page.waitForFunction(() => !document.getElementById('learning-export').disabled);
      const captureDownload = page.waitForEvent('download');
      await page.locator('#learning-export').click();
      const captureFile = await captureDownload;
      const exportedCapture = JSON.parse(fs.readFileSync(await captureFile.path(), 'utf8'));
      assert.equal(captureFile.suggestedFilename(), 'rf433-button-manager-capture.json');
      assert.equal(exportedCapture.packets[0].modulation, 'FSK');
      assert.equal(exportedCapture.format, 'rf433-button-manager-capture-v1');
      await page.waitForFunction(() => !document.getElementById('learning-retry').disabled);
      await page.locator('#learning-retry').click();
      await hidden(page.locator('#learning-retry'));
      assert.match(await page.locator('#learning-count').textContent(), /0 of 3/);
      await page.locator('#learning-cancel').click();
      await hidden(page.locator('#learning-dialog'));
    } finally {
      await page.unroute('**/api/state', stateRoute);
      await page.unroute('**/api/learn/*', learnRoute);
      await page.reload();
      await hidden(page.locator('#login-dialog'));
      await page.locator('.rf-button').first().click();
    }
  });
  await check('settings, backup download and restore disable actions', async () => {
    await page.locator('#settings-open').click();
    await visible(page.locator('#settings-dialog'));
    const downloading = page.waitForEvent('download');
    await page.locator('#backup-download').click();
    const download = await downloading;
    const content = JSON.parse(fs.readFileSync(await download.path(), 'utf8'));
    assert.equal(content.schema_version, 2);
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
    await hidden(page.locator('#editor-back'));
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    const screenshots = path.resolve(__dirname, '../../docs/screenshots');
    fs.mkdirSync(screenshots, {recursive:true});
    await hidden(page.locator('#toast'));
    await page.screenshot({path:path.join(screenshots,'dashboard.png'),fullPage:true});
    await page.locator('#button-name').fill('Unsaved layout check');
    await page.locator('#layout-toggle').click();
    assert.equal(await page.locator('html').getAttribute('data-layout'), 'unified');
    await hidden(page.locator('#editor-back'));
    const mainBox=await page.locator('#main-content').boundingBox();
    assert.ok(Math.abs(mainBox.x+mainBox.width-1440)<2,'main scrollbar must reach the viewport edge');
    await page.locator('#view-activity').click();
    await visible(page.locator('#activity-view'));
    await page.locator('#view-remotes').click();
    assert.equal(await page.locator('#button-name').inputValue(), 'Unsaved layout check');
    await page.locator('#button-name').fill('A ON');
    await page.locator('#button-save').click();
    await page.waitForFunction(() => document.getElementById('unsaved-status').textContent === '');
    await page.screenshot({path:path.join(screenshots,'unified.png'),fullPage:true});
    await page.locator('#theme-toggle').click();
    assert.equal(await page.locator('html').getAttribute('data-theme'), 'light');
    await page.setViewportSize({width:820,height:1180});
    await visible(page.locator('#editor-back'));
    await page.locator('#editor-back').click();
    await visible(page.locator('#remote-picker'));
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.screenshot({path:path.join(screenshots,'tablet.png'),fullPage:true});
    await page.locator('#remote-picker').click();
    await visible(page.locator('#remote-picker-dialog'));
    const option=page.locator('#remote-picker-list button').first();
    assert.ok((await option.boundingBox()).height>=48);
    await page.locator('#remote-picker-search').fill('no such remote');
    assert.equal(await page.locator('#remote-picker-list button').count(),0);
    await page.locator('#remote-picker-search').fill('');
    await option.click();
    await hidden(page.locator('#remote-picker-dialog'));
    await page.setViewportSize({width:390,height:844});
    await delay(250);
    await visible(page.locator('#receiver-state'));
    assert.equal(await page.locator('.rf-button').nth(1).evaluate(el => getComputedStyle(el).backgroundColor), 'rgb(255, 255, 255)');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.screenshot({path:path.join(screenshots,'mobile.png'),fullPage:true});
    const keyBox=await page.locator('.rf-button').first().boundingBox();
    assert.ok(keyBox.width>=48 && keyBox.height>=48);
    await page.locator('#settings-open').click();
    await page.locator('#appearance-theme').selectOption('dark');
    assert.equal(await page.locator('html').getAttribute('data-theme'), 'dark');
    await page.locator('#view-remotes').click();
    await page.locator('.rf-button').first().click();
    await visible(page.locator('#editor-back'));
    await hidden(page.locator('.remote-panel'));
  });
  await check('network workflow, certificate download and touch address selection', async () => {
    await page.locator('#settings-open').click();
    await page.locator('#network-heading').scrollIntoViewIfNeeded();
    assert.equal(await page.locator('#network-enabled').isChecked(),false);
    assert.equal(await page.locator('#network-http').isDisabled(),true);
    assert.equal(await page.locator('#network-https').isDisabled(),true);
    assert.equal(await page.locator('#network-keep-http').count(),0);
    assert.equal(await page.locator('#network-address').count(),0);
    assert.equal(await page.locator('#network-advanced summary').textContent(),'Advanced: address and ports');
    assert.equal(await page.locator('#network-advanced #network-address-choice').count(),1);
    assert.equal(await page.locator('#network-stage').isDisabled(),true);
    assert.match(await page.locator('#network-local-urls').textContent(),new RegExp(`127.0.0.1:${port}`));
    await page.locator('#network-enabled').check();
    await visible(page.locator('#network-address-dialog'));
    await page.keyboard.press('Escape');
    await page.locator('#connection-security').evaluate(el=>{el.open=true;});
    await page.locator('#network-https').check();
    assert.equal(await page.locator('#network-stage').isDisabled(),false);
    await page.locator('#network-http').check();
    await page.locator('#network-enabled').uncheck();
    assert.equal(await page.locator('#network-stage').isDisabled(),true);
    await page.locator('#certificate-guide').evaluate(el=>{el.open=true;});
    const downloadEvent=page.waitForEvent('download');
    await page.locator('#certificate-download').click();
    const download=await downloadEvent;
    const certificate=fs.readFileSync(await download.path(),'utf8');
    assert.match(certificate,/BEGIN CERTIFICATE/);assert.doesNotMatch(certificate,/PRIVATE KEY/);
    await page.waitForFunction(()=>document.getElementById('certificate-fingerprint').textContent.length>0);
    assert.match(await page.locator('#certificate-fingerprint').textContent(),/^([0-9A-F]{2}:){31}[0-9A-F]{2}$/);
    for(const text of ['Windows — Chrome and Edge','Linux — Chrome and Chromium','Firefox — desktop','Android — Chrome']){
      const summary=page.locator('#certificate-guide summary').filter({hasText:text});
      await summary.click();
      assert.ok(await summary.locator('..').locator('li').count()>=3);
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
      await summary.click();
    }
    await page.locator('#network-advanced').evaluate(el=>{el.open=true;});
    await page.locator('#network-http-plus').click();
    assert.equal(await page.locator('#network-http-port').inputValue(),String(Number(port)+1));
    await page.locator('#network-http-minus').click();
    assert.equal(await page.locator('#network-http-port').inputValue(),port);
    await page.locator('#network-http-reset').click();
    assert.equal(await page.locator('#network-http-port').inputValue(),'8787');
    await page.locator('#network-https-plus').click();
    await page.locator('#network-https-reset').click();
    assert.equal(await page.locator('#network-https-port').inputValue(),'8788');
    await page.locator('#network-http-port').fill('18789');
    await delay(2200);
    assert.equal(await page.locator('#network-http-port').inputValue(),'18789','poll must retain draft');
    await page.locator('#network-stage').click();
    await visible(page.locator('#network-pending'));
    assert.equal(await page.locator('#network-confirm').isDisabled(),true);
    assert.equal(await page.locator('#network-enabled').isDisabled(),true);
    await page.locator('#network-cancel').click();
    await hidden(page.locator('#network-pending'));
    assert.equal(await page.locator('#network-http-port').inputValue(),port);
    assert.equal(await page.locator('#network-stage').isDisabled(),true);
    await page.locator('#certificate-guide').evaluate(el=>{el.open=false;});
    await page.setViewportSize({width:1440,height:1100});
    for(const size of [{width:1440,height:1100},{width:820,height:1180},{width:390,height:844},{width:320,height:740}]) {
      await page.setViewportSize(size);
      assert.equal(await page.locator('#main-content').evaluate(el=>el.scrollWidth<=el.clientWidth+1),true,`content overflow at ${size.width}`);
      for(const control of await page.locator('.port-stepper button, .port-stepper input').all()) {
        assert.ok((await control.boundingBox()).height>=48,'port controls need touch-sized targets');
        assert.equal(await control.evaluate(el=>el.scrollWidth<=el.clientWidth+1),true,'port controls must fit');
      }
      for(const card of await page.locator('.security-choices .choice-card').all()) {
        assert.equal(await card.evaluate(el=>el.scrollWidth<=el.clientWidth+1),true,'radio labels must fit inside the card');
        assert.ok((await card.locator('input').boundingBox()).width<=24,'native radio must not inherit full text-input width');
      }
    }
    await page.setViewportSize({width:1440,height:1100});
    await page.locator('#network-heading').evaluate(el=>el.scrollIntoView({block:'start'}));
    await hidden(page.locator('#toast'));
    await page.screenshot({path:path.join(qa,'network-desktop.png'),fullPage:true});
    await page.setViewportSize({width:390,height:844});
    await page.locator('#network-heading').evaluate(el=>el.scrollIntoView({block:'start'}));
    await page.screenshot({path:path.join(qa,'network-phone.png'),fullPage:true});
  });
  await check('many remotes remain searchable and scrollable on a touch phone', async () => {
    for(let i=1;i<=30;i++) {
      const response=await fetch(`${base}/api/remotes`,{method:'POST',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify({name:`Test remote ${String(i).padStart(2,'0')}`,columns:2,button_count:1})});
      assert.equal(response.status,200);
    }
    const phone=page;
    {
      await phone.setViewportSize({width:390,height:844});
      await phone.locator('#view-remotes').tap();
      if(await phone.locator('#editor-back').isVisible()) await phone.locator('#editor-back').tap();
      await phone.waitForFunction(()=>document.querySelectorAll('#remote-list .remote-nav').length===32);
      assert.equal(await phone.evaluate(()=>matchMedia('(pointer:coarse)').matches),true);
      await phone.locator('#remote-picker').tap();
      await visible(phone.locator('#remote-picker-dialog'));
      assert.equal(await phone.locator('#remote-picker-list button').count(),32);
      const last=phone.locator('#remote-picker-list button').last();await last.scrollIntoViewIfNeeded();
      const box=await last.boundingBox();assert.ok(box.height>=48&&box.y>=0&&box.y+box.height<=844);
      await phone.locator('#remote-picker-search').fill('Test remote 30');
      assert.equal(await phone.locator('#remote-picker-list button').count(),1);
      assert.ok((await phone.locator('#remote-picker-dialog').boundingBox()).height<600,'filtered picker must shrink instead of leaving a full blank sheet');
      await phone.screenshot({path:path.join(qa,'remote-picker-phone.png')});
      await phone.locator('#remote-picker-list button').tap();await hidden(phone.locator('#remote-picker-dialog'));
      assert.equal(await phone.locator('#remote-title').textContent(),'Test remote 30');
      await phone.locator('.rf-button').tap();await visible(phone.locator('#editor-back'));await hidden(phone.locator('.remote-panel'));
      await phone.locator('#button-name').fill('Touch draft');await phone.locator('#editor-back').tap();
      await phone.locator('.rf-button').tap();assert.equal(await phone.locator('#button-name').inputValue(),'Touch draft');
      await phone.locator('#settings-open').tap();await phone.locator('#network-enabled').check();
      await visible(phone.locator('#network-address-dialog'));
      for(const choice of await phone.locator('#network-address-list button').all()) assert.ok((await choice.boundingBox()).height>=48);
      await phone.locator('[data-close="network-address-dialog"]').tap();await phone.locator('#network-enabled').uncheck();
      assert.equal(await phone.locator('#network-stage').isDisabled(),true,'selecting network draft must not silently apply it');
    }
  });
  await check('lock removes token and clears visible secrets', async () => {
    await page.locator('#logout').click();
    await visible(page.locator('#confirm-dialog'));
    await page.locator('#confirm-yes').click();
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
