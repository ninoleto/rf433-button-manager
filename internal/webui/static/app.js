'use strict';
(() => {
  const $ = (id) => document.getElementById(id);
  const DEFAULT_FIELDS = ['id', 'channel', 'command', 'state', 'button', 'code', 'codes'];
  let state = { remotes: [], events: [], settings: {}, receiver: {}, learning: null };
  let token = '';
  try { token = sessionStorage.getItem('rfbm-token') || ''; } catch (_) { /* Storage may be disabled. */ }
  let selectedRemote = null;
  let selectedButton = null;
  let editorDirty = false;
  let pollingTimer = null;
  let pollDelay = 1000;
  let pollingGeneration = 0;
  let remoteMode = 'create';
  let toastTimer = null;
  let navKey = '';
  let gridKey = '';
  let eventsKey = '';
  let learningKey = '';
  let learningWasActive = false;
  let confirmResolve = null;
  let mutationCount = 0;
  let receiverDevices = [];
  let receiverListReady = false;
  let receiverScanGeneration = 0;
  let activeView = 'remotes';
  let settingsLoaded = false;
  let networkLoaded = false;
  let networkState = null;
  let networkAddress = '';
  let networkTimer = null;
  let networkURLKey = '';
  let networkSavedKey = '';
  let networkPendingID = null;
  let layout = 'classic';
  try { layout = localStorage.getItem('rf433-layout') || 'classic'; } catch (_) {}
  function applyLayout(value) {
    layout = value === 'unified' ? 'unified' : 'classic';
    document.documentElement.dataset.layout = layout;
    $('layout-toggle').textContent = `Layout: ${layout === 'unified' ? 'Unified' : 'Classic'}`;
    $('layout-toggle').setAttribute('aria-label', `Switch to ${layout === 'unified' ? 'Classic' : 'Unified'} desktop layout`);
    try { localStorage.setItem('rf433-layout', layout); } catch (_) {}
  }
  function setView(value) {
    activeView = value;
    document.documentElement.dataset.view = value;
    $('settings-dialog').open = value === 'settings';
    for (const [id,view] of [['view-remotes','remotes'],['view-activity','activity'],['settings-open','settings']]) $(id).setAttribute('aria-current', value === view ? 'page' : 'false');
  }
  applyLayout(layout); setView('remotes');

  function node(tag, className, text) {
    const el = document.createElement(tag);
    if (className) el.className = className;
    if (text !== undefined) el.textContent = text;
    return el;
  }
  function fmtTime(value) {
    if (!value || String(value).startsWith('0001-')) return 'Never';
    const d = new Date(value);
    return Number.isNaN(d.getTime()) ? 'Unknown' : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }
  function fmtDate(value) {
    if (!value || String(value).startsWith('0001-')) return 'No signal received';
    const d = new Date(value);
    return Number.isNaN(d.getTime()) ? 'No signal received' : `Last received ${d.toLocaleString()}`;
  }
  function currentRemote() { return (state.remotes || []).find((r) => r.id === selectedRemote); }
  function currentButton() { return (currentRemote()?.buttons || []).find((b) => b.id === selectedButton); }
  function findButton(id) {
    for (const remote of state.remotes || []) {
      const button = (remote.buttons || []).find((b) => b.id === id);
      if (button) return { remote, button };
    }
    return null;
  }
  function setError(id, message) {
    $(id).textContent = message || '';
    $(id).hidden = !message;
  }
  function toast(message, error = false) {
    clearTimeout(toastTimer);
    $('toast').textContent = message;
    $('toast').className = `toast${error ? ' error' : ''}`;
    $('toast').hidden = false;
    toastTimer = setTimeout(() => { $('toast').hidden = true; }, error ? 8500 : 4500);
  }
  function show(dialog) { if (dialog === 'settings-dialog') { setView('settings'); return; } if (!$(dialog).open) $(dialog).showModal(); }
  function close(dialog) { if (dialog === 'settings-dialog') { settingsLoaded = false; setView('remotes'); return; } if ($(dialog).open) $(dialog).close(); }
  async function ask(title, message, label = 'Continue') {
    if (confirmResolve) return false;
    $('confirm-title').textContent = title;
    $('confirm-message').textContent = message;
    $('confirm-yes').textContent = label;
    show('confirm-dialog');
    $('confirm-no').focus();
    return new Promise((resolve) => { confirmResolve = resolve; });
  }
  function answer(value) {
    close('confirm-dialog');
    const resolve = confirmResolve;
    confirmResolve = null;
    if (resolve) resolve(value);
  }
  $('confirm-no').addEventListener('click', () => answer(false));
  $('confirm-yes').addEventListener('click', () => answer(true));
  $('confirm-dialog').addEventListener('cancel', (e) => { e.preventDefault(); answer(false); });

  function lock(message = '') {
    token = '';
    clearTimeout(networkTimer); settingsLoaded = false; networkLoaded = false; networkState = null; networkAddress = ''; networkURLKey = ''; networkSavedKey = ''; networkPendingID = null;
    $('new-management-token').value = ''; $('new-token-panel').hidden = true;
    $('management-token').type = 'password'; $('token-visibility').textContent = 'Show';
    setView('remotes'); document.documentElement.dataset.editor = 'closed';
    pollingGeneration++;
    clearTimeout(pollingTimer);
    try { sessionStorage.removeItem('rfbm-token'); } catch (_) { /* No persistent token fallback. */ }
    for (const dialog of document.querySelectorAll('dialog[open]')) dialog.close();
    if (confirmResolve) { confirmResolve(false); confirmResolve = null; }
    state = { remotes: [], events: [], settings: {}, receiver: {}, learning: null };
    selectedRemote = null; selectedButton = null; editorDirty = false;
    navKey = ''; gridKey = ''; eventsKey = ''; learningKey = ''; learningWasActive = false;
    receiverDevices = []; receiverListReady = false; receiverScanGeneration++;
    $('receiver-serial').replaceChildren();
    $('button-editor').reset(); $('settings-form').reset(); $('ha-id').value = '';
    render();
    $('management-token').value = '';
    setError('login-error', message);
    show('login-dialog');
  }
  async function api(path, method = 'GET', body) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 45000);
    try {
      const requestToken = token;
      const headers = { Authorization: `Bearer ${requestToken}` };
      if (body !== undefined) headers['Content-Type'] = 'application/json';
      const response = await fetch(`/api${path}`, {
        method, headers, body: body === undefined ? undefined : JSON.stringify(body),
        credentials: 'same-origin', cache: 'no-store', redirect: 'error', signal: controller.signal,
      });
      let result;
      try { result = await response.json(); } catch (_) { result = {}; }
      if (!response.ok) {
        if (response.status === 401 && token === requestToken) lock('The management token was rejected. Enter the current token.');
        throw new Error(result.error || `Request failed (HTTP ${response.status}).`);
      }
      return result;
    } catch (error) {
      if (error.name === 'AbortError') throw new Error('The request timed out. Check the event monitor before repeating an action.');
      throw error;
    } finally { clearTimeout(timeout); }
  }
  async function refresh() {
    const requestedToken = token;
    const next = await api('/state');
    if (!token || requestedToken !== token) return;
    state = next;
    state.remotes = state.remotes || [];
    state.events = state.events || [];
    state.settings = state.settings || {};
    state.receiver = state.receiver || {};
    if (!state.remotes.some((r) => r.id === selectedRemote)) {
      selectedRemote = state.remotes[0]?.id || null;
      selectedButton = null; editorDirty = false;
    }
    if (selectedButton && !currentButton()) { selectedButton = null; editorDirty = false; }
    $('connection-banner').hidden = true;
    render();
  }
  async function poll(generation) {
    if (!token || generation !== pollingGeneration) return;
    try {
      await refresh(); pollDelay = 1000;
    } catch (error) {
      if (token) {
        $('connection-banner').textContent = `Connection interrupted. Retrying automatically. ${error.message}`;
        $('connection-banner').hidden = false;
        pollDelay = Math.min(pollDelay * 2, 30000);
      }
    }
    if (token && generation === pollingGeneration) pollingTimer = setTimeout(() => poll(generation), pollDelay);
  }
  function startPolling() {
    clearTimeout(pollingTimer);
    const generation = ++pollingGeneration;
    pollDelay = 1000;
    poll(generation);
  }
  async function mutate(path, method = 'POST', body = {}, message = '') {
    mutationCount++;
    updateActionAvailability();
    try {
      const result = await api(path, method, body);
      await refresh();
      if (message) toast(message);
      return result;
    } finally { mutationCount--; updateActionAvailability(); }
  }
  function handleError(error, target) {
    if (target) setError(target, error.message);
    else if ($('remote-dialog').open) setError('remote-error', error.message);
    else if ($('name-dialog').open) setError('name-error', error.message);
    else toast(error.message || 'The operation failed.', true);
  }
  function bindAction(id, action, errorTarget) {
    $(id).addEventListener('click', async () => {
      try { await action(); } catch (error) { handleError(error, errorTarget); }
    });
  }
  function fields(value) { return [...new Set(value.split(',').map((s) => s.trim()).filter(Boolean))]; }

  function renderNavigation() {
    const search = $('remote-search').value.trim().toLowerCase();
    const key = JSON.stringify([search, selectedRemote, state.remotes.map((r) => [r.id, r.name, (r.buttons || []).length])]);
    if (key === navKey) return;
    navKey = key;
    const fragment = document.createDocumentFragment();
    if (!state.remotes.length) fragment.append(node('p', 'sidebar-empty', 'No remotes yet. Add your first remote to get started.'));
    const selected = state.remotes.find(r => r.id === selectedRemote);
    $('remote-picker-name').textContent = selected?.name || 'Choose a remote';
    $('remote-picker-count').textContent = selected ? `${selected.buttons.length} buttons · Change remote` : 'Select or create a remote';
    renderRemotePicker();
    for (const remote of state.remotes.filter(r => r.name.toLowerCase().includes(search))) {
      const button = node('button', `remote-nav${remote.id === selectedRemote ? ' selected' : ''}`);
      button.type = 'button';
      if (remote.id === selectedRemote) button.setAttribute('aria-current', 'true');
      button.append(node('span', 'nav-icon', '▦'));
      const label = node('span');
      label.append(node('span', 'nav-label', remote.name), node('span', 'nav-count', `${(remote.buttons || []).length} buttons`));
      button.append(label);
      button.addEventListener('click', async () => {
        if (remote.id === selectedRemote) { setView('remotes'); return; }
        if (!(await mayDiscard())) return;
        selectedRemote = remote.id; selectedButton = null; editorDirty = false;
        setView('remotes');
        render();
      });
      fragment.append(button);
    }
    $('remote-list').replaceChildren(fragment);
  }
  function renderRemote() {
    const remote = currentRemote();
    $('remote-title').textContent = remote?.name || 'Your first remote';
    $('remote-description').textContent = remote ? 'Learn a signal. Assign an action. Keep it running.' : 'Create a remote, learn its buttons, and connect your actions.';
    $('remote-count').textContent = `${(remote?.buttons || []).length} BUTTONS`;
    $('remote-empty').hidden = !!remote;
    $('remote-device').hidden = !remote;
    $('remote-caption').hidden = !remote;
    $('remote-edit-open').disabled = !remote;
    $('button-create').disabled = !remote;
    const key = JSON.stringify([selectedButton, remote?.id, remote?.columns, remote?.buttons]);
    if (key === gridKey) return;
    gridKey = key;
    const fragment = document.createDocumentFragment();
    for (const button of remote?.buttons || []) {
      const receivedAt = new Date(button.last_received).getTime();
      const recent = Number.isFinite(receivedAt) && Date.now() - receivedAt < 1800;
      const item = node('button', `rf-button${button.id === selectedButton ? ' selected' : ''}${button.signature ? ' mapped' : ''}${button.enabled ? ' enabled' : ''}${recent ? ' recent' : ''}`);
      item.type = 'button';
      item.setAttribute('aria-pressed', String(button.id === selectedButton));
      item.setAttribute('aria-label', `${button.name}, ${button.signature ? 'signal learned' : 'not learned'}, ${button.enabled ? 'enabled' : 'disabled'}. Edit button.`);
      const indicator = node('span', 'button-indicator'); indicator.setAttribute('aria-hidden', 'true');
      item.append(indicator, node('span', 'rf-name', button.name), node('span', 'rf-sub', button.signature ? (button.enabled ? 'Ready' : 'Disabled') : 'Not learned'));
      item.addEventListener('click', async () => {
        document.documentElement.dataset.editor = 'open';
        if (button.id === selectedButton) return;
        if (!(await mayDiscard())) return;
        selectedButton = button.id; editorDirty = false;
        fillEditor(); render();
      });
      fragment.append(item);
    }
    $('button-grid').className = `button-grid cols-${Math.min(6, Math.max(1, Number(remote?.columns) || 2))}`;
    $('button-grid').replaceChildren(fragment);
  }
  function fillEditor() {
    const button = currentButton();
    if (!button) return;
    $('button-name').value = button.name || '';
    $('button-enabled').checked = !!button.enabled;
    $('button-debounce').value = button.debounce_ms || 700;
    $('learning-automatic-fields').checked = true;
    $('button-fields').value = (button.identity_fields?.length ? button.identity_fields : state.settings.identity_fields?.length ? state.settings.identity_fields : DEFAULT_FIELDS).join(', ');
    const hook = button.webhook || {};
    $('webhook-url').value = hook.url || '';
    $('webhook-method').value = hook.method || 'POST';
    $('webhook-timeout').value = hook.timeout_ms || 5000;
    $('webhook-payload').value = hook.payload || '';
    $('webhook-headers').value = Object.keys(hook.headers || {}).length ? JSON.stringify(hook.headers, null, 2) : '';
    $('ha-id').value = '';
    try {
      const saved = new URL(hook.url);
      if (saved.pathname.startsWith('/api/webhook/')) $('ha-base').value = saved.origin;
    } catch (_) { /* No saved Home Assistant URL yet. */ }
    editorDirty = false;
    setError('editor-error', '');
  }
  function renderEditor() {
    const button = currentButton();
    $('editor-empty').hidden = !!button;
    $('button-editor').hidden = !button;
    if (!button) return;
    $('editor-heading').textContent = button.name;
    $('button-signature').textContent = button.signature || 'No signal assigned';
    $('mapping-status').textContent = button.signature ? 'Signal learned' : 'Not learned';
    $('mapping-status').className = `pill${button.signature ? ' success' : ''}`;
    $('learn-start').textContent = button.signature ? 'Relearn signal' : 'Learn signal';
    $('button-last-received').textContent = fmtDate(button.last_received);
    $('unsaved-status').textContent = editorDirty ? 'Unsaved changes' : '';
    renderWebhookDestination();
    renderReception();
    updateActionAvailability();
  }
  function levelText(value, unit) {
    return typeof value === 'number' && Number.isFinite(value) ? `${value.toFixed(1)} ${unit}` : 'Not available';
  }
  function readingTime(at) {
    const age = Date.now() - new Date(at || '').getTime();
    return `${fmtTime(at)}${age > 30000 ? ' · older reading' : ''}`;
  }
  function renderReception() {
    const button = currentButton();
    if (!button) return;
    const d = state.reception?.buttons?.[button.id];
    $('reception-mode').textContent = `${state.demo ? 'Synthetic demo · ' : ''}${state.receiver.adaptive_sensitivity ? 'Adaptive' : 'Standard'} sensitivity · ${state.actions_paused ? 'normal reception paused' : (state.receiver.state || 'not connected')}`;
    $('reception-count').textContent = String(d?.presses || 0);
    $('reception-repeats').textContent = String(d?.repeats || 0);
    $('reception-signal').textContent = levelText(d?.levels?.rssi_dbfs, 'dBFS');
    $('reception-snr').textContent = levelText(d?.levels?.snr_db, 'dB');
    $('reception-noise').textContent = levelText(d?.levels?.noise_dbfs, 'dBFS');
    $('reception-last').textContent = d?.last_matched_at ? `Last matched signal: ${readingTime(d.last_matched_at)}` : 'Waiting for this saved button. Background RF traffic does not update these measurements.';
    const since = d?.since || state.reception?.since;
    $('reception-since').textContent = since ? `Since ${fmtTime(since)} · resets when reception restarts` : 'Counts begin with normal reception.';
    const action = d?.action ? `Latest action: ${d.action} (${fmtTime(d.action_at)})` : 'No action result in this counting session.';
    const stateNote = state.actions_paused ? 'Actions are paused.' : state.storage_error ? 'Actions are paused because storage failed.' : !button.enabled ? 'This button is disabled; matched presses are still counted.' : !button.webhook?.url ? 'No webhook is configured.' : 'This button is enabled; recognized presses may run its action.';
    $('reception-action').textContent = `${stateNote} ${action}`;
    const recording = state.reception_recording;
    $('recording-panel').hidden = !state.reception_recording_supported && !recording;
    $('recording-stop').hidden = !recording?.active;
    $('recording-download').hidden = !recording || recording.active;
    $('recording-start').hidden = !!recording?.active;
    const reasons = {stopped:'Recording stopped', time_limit:'60-second recording complete', data_limit:'Recording stopped at the data limit', record_too_large:'Recording stopped: one record exceeded the size limit', receiver_stream_closed:'Recording stopped: receiver stream closed', receiver_unavailable:'Recording stopped: receiver became unavailable', receiver_settings_changed:'Recording stopped: receiver settings changed', configuration_changed:'Recording stopped: configuration changed', inspection_or_learning_started:'Recording stopped: inspection or learning started', service_stopping:'Recording stopped: app shutting down'};
    $('recording-status').textContent = recording
      ? `${state.demo ? 'Synthetic demo · ' : ''}${recording.active ? `Recording ${recording.button_name} · ${recording.seconds_remaining}s remaining` : `${reasons[recording.stop_reason] || 'Recording complete'} · ${recording.button_name}`} · ${recording.recognized_presses} recognized · ${recording.packet_count} RF packets · ${recording.decoded_count} decoded reports`
      : state.demo ? 'Demo records synthetic events only; no radio capture or webhook is performed.' : 'Starting and stopping a recording keeps the receiver running. Existing action settings stay in effect.';
  }
  function webhookDestination(raw) {
    if (!raw) return null;
    let url;
    try { url = new URL(raw); } catch (_) { throw new Error('Paste a complete HTTP(S) webhook URL, including the server address.'); }
    if (!/^https?:\/\//i.test(raw) || !['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.hash || /[\r\n\t]/.test(raw)) {
      throw new Error('Use an HTTP(S) webhook URL without a username, password or # fragment.');
    }
    // Preserve explicit ports, matching the server's exact origin check.
    const origin = raw.match(/^https?:\/\/[^/?#]+/i)[0].toLowerCase();
    return { origin, needsApproval: !(state.settings.allowed_origins || []).includes(origin) };
  }
  function renderWebhookDestination() {
    let destination;
    try { destination = webhookDestination($('webhook-url').value.trim()); } catch (_) { /* Explain invalid input on Save. */ }
    $('button-save').textContent = destination?.needsApproval ? 'Save & allow destination' : 'Save button';
    $('webhook-destination-note').textContent = destination?.needsApproval
      ? `Saving will allow webhook requests to ${destination.origin}. It will not send a test request.`
      : 'Saving does not send a test request. Requests run from this server.';
  }
  function updateActionAvailability() {
    const learning = !!state.learning || !!state.actions_paused;
    const busy = mutationCount > 0;
    $('inspect-open').disabled = learning || busy || !token;
    $('settings-save').disabled = learning || busy || !receiverListReady;
    $('receiver-serial').disabled = learning || busy || !receiverListReady;
    $('sensitivity-standard').disabled = learning || busy; $('sensitivity-adaptive').disabled = learning || busy;
    $('reception-reset').disabled = learning || busy || !currentButton()?.signature;
    $('recording-start').disabled = learning || busy || editorDirty || !state.reception_recording_supported || !!state.reception_recording?.active || !currentButton()?.signature || (!state.demo && state.receiver.state !== 'connected');
    $('recording-stop').disabled = busy || !state.reception_recording?.active;
    $('recording-download').disabled = busy || !state.reception_recording;
    $('webhook-test').disabled = !!state.demo || learning || busy || editorDirty || !currentButton()?.webhook?.url;
    $('learn-start').disabled = learning || busy || editorDirty;
    $('button-save').disabled = busy || learning;
    $('button-clear').disabled = busy || learning;
    $('button-delete').disabled = busy || learning;
    $('learning-arm').disabled = busy;
    $('learning-confirm').disabled = busy;
    $('learning-cancel').disabled = busy;
    $('learning-retry').disabled = busy || !!state.actions_paused;
  }
  async function mayDiscard() {
    if (!editorDirty) return true;
    return ask('Discard unsaved changes?', 'This button has changes that have not been saved.', 'Discard changes');
  }
  function renderEvents() {
    const filter = $('event-filter').value;
    const items = state.events.filter((event) => {
      if (filter === 'button') return selectedButton && event.button_id === selectedButton;
      if (filter === 'all') return true;
      return String(event.kind).toLowerCase().includes(filter);
    }).slice().sort((a, b) => new Date(b.time) - new Date(a.time)).slice(0, 150);
    const key = JSON.stringify([filter, selectedButton, items]);
    if (key === eventsKey) return;
    eventsKey = key;
    const fragment = document.createDocumentFragment();
    if (!items.length) fragment.append(node('p', 'events-empty', filter === 'button' && !selectedButton ? 'Select a button to view its history.' : 'No matching events yet.'));
    for (const event of items) {
      const row = node('div', 'event-row');
      const time = node('time', 'event-time', fmtTime(event.time));
      time.dateTime = event.time || '';
      time.title = event.time || '';
      const kind = String(event.kind || 'event');
      const isError = /error|fail|unknown/.test(kind);
      const type = node('span', `event-kind${isError ? ' error' : ''}`, kind.replaceAll('_', ' '));
      const content = node('div', 'event-message');
      content.append(node('span', '', event.message || 'Received event'));
      if (event.receiver && (event.receiver.serial || event.receiver.usb_port || event.receiver.name)) content.append(node('p', 'event-source', `Receiver: ${receiverLabel(event.receiver)}`));
      if (event.levels) content.append(node('p', 'event-source', `Signal ${levelText(event.levels.rssi_dbfs, 'dBFS')} · SNR ${levelText(event.levels.snr_db, 'dB')} · noise ${levelText(event.levels.noise_dbfs, 'dBFS')}`));
      if (event.data !== undefined && event.data !== null && event.data !== '') {
        const details = node('details');
        details.append(node('summary', '', event.data.diagnostics || event.kind === 'learning_capture' ? 'Capture diagnostics' : 'Decoded data'), node('pre', '', typeof event.data === 'string' ? event.data : JSON.stringify(event.data, null, 2)));
        content.append(details);
      }
      row.append(time, type, content); fragment.append(row);
    }
    $('event-list').replaceChildren(fragment);
  }
  function renderLearning() {
    $('learning-export').disabled = !state.rf_capture_available;
    const learning = state.learning || state.learning_notice;
    const active = !!state.learning;
    const inspecting = !!learning?.inspect_only;
    $('learning-banner').hidden = !state.actions_paused && !active;
    $('learning-reopen').hidden = !learning;
    $('learning-banner-text').textContent = active ? `${inspecting ? 'Signal inspection' : 'Learning mode'} · all webhook actions are paused.` : 'Restoring reception · all webhook actions remain paused.';
    if (!learning) {
      close('learning-dialog');
      learningKey = '';
      if (learningWasActive) {
        learningWasActive = false;
        if (!editorDirty) fillEditor();
      }
      return;
    }
    $('learning-version').textContent = state.version ? `RF433 Button Manager ${state.version}` : '';
    $('learning-title').textContent = inspecting ? 'Inspect signal' : 'Learn a button';
    $('learning-eyebrow').textContent = inspecting ? 'SIGNAL INSPECTION' : 'SIGNAL LEARNING';
    $('inspection-result').hidden = !inspecting;
    const target = findButton(learning.button_id);
    $('learning-target').textContent = inspecting ? '433.92 MHz · inspect without creating a remote or webhook' : target ? `${target.remote.name} / ${target.button.name}` : `Button ${learning.button_id}`;
    const key = JSON.stringify([learning, state.receiver, state.receiver_mode, state.actions_paused]);
    if (key !== learningKey) {
      learningKey = key;
      const presses = Number(learning.presses) || 0;
      const required = Number(learning.required) || 3;
      const progress = document.createDocumentFragment();
      for (let i = 1; i <= required; i++) {
        const step = node('span', `learning-step${i <= presses ? ' complete' : learning.stage === 'waiting' && i === presses + 1 ? ' active' : ''}`, i <= presses ? '✓' : String(i));
        step.setAttribute('aria-label', `Press ${i}: ${i <= presses ? 'verified' : 'pending'}`);
        progress.append(step);
      }
      $('learning-steps').replaceChildren(progress);
      $('learning-count').textContent = `${presses} of ${required} matching presses`;
      $('learning-raw').textContent = learning.raw ? JSON.stringify(learning.raw, null, 2) : 'No decoded candidate yet.';
      $('learning-candidate').textContent = learning.candidate || '—';
      const ready = active && learning.stage === 'ready';
      const captured = active && learning.stage === 'captured';
      if (inspecting) {
        let identifier = learning.candidate || 'No decoded candidate yet.';
        try { identifier = JSON.stringify(JSON.parse(learning.candidate), null, 2); } catch (_) { /* Not decoded yet. */ }
        $('inspection-code').textContent = identifier;
        $('inspection-verification').textContent = presses >= required ? 'The same identifier was observed on three separately armed presses. Verify it belongs to the intended button.' : `${presses} of ${required} matching presses. A first candidate is not a compatibility guarantee.`;
        $('inspection-decoder').textContent = learning.profile?.flex_decoder || (learning.candidate ? 'Decoded by an existing rtl_433 or configured decoder.' : 'No validated decoder yet. Download the RF capture if this attempt fails.');
      }
      $('learning-arm').hidden = !captured;
      $('learning-confirm').hidden = !ready;
      $('learning-confirm').textContent = inspecting ? 'Finish inspection' : 'Confirm and save';
      $('learning-retry').hidden = active;
      $('learning-cancel').textContent = active ? (inspecting ? 'Stop inspection' : 'Cancel learning') : 'Close';
      const titles = {preparing:'Preparing receiver', waiting:'Press the physical button once', receiving:'Transmission detected', analyzing:'Checking the captured signal', captured:'Press verified — release the button', ready:'Ready to save', receiver_unavailable:'Receiver unavailable', timeout:'Learning session timed out', no_capture:'No pulse capture', malformed_capture:'Malformed pulse capture', unsupported_modulation:'No supported modulation captured', capture_error:'Capture stream failed', unsupported:'Capture has no validated decoder', inconsistent:'Captures did not agree', capture_limit:'Capture limit reached', analysis_error:'Analysis could not finish'};
      titles.press_timeout = 'Press window timed out';
      if (inspecting) { titles.ready = 'Three matching presses observed'; titles.captured = 'Candidate decoded — release the button'; }
      const budgetUsed = learning.automatic && learning.stage === 'waiting' && learning.window_analysis_limit > 0 && learning.window_analyses >= learning.window_analysis_limit;
      $('learning-instruction').textContent = budgetUsed ? 'Analysis budget reached' : titles[learning.stage] || 'Learning stopped';
      $('learning-description').textContent = learning.message || (ready ? 'Review the decoded data, then confirm this is the intended button.' : captured ? 'Release the button, then select Ready for next press.' : 'Keep other remotes idle. Release the button fully after each press.');
      if (inspecting && ready) $('learning-description').textContent = 'Your code is shown below. Download the inspection report before finishing. Nothing will be saved as a button action.';
      $('learning-pause-text').textContent = active || state.actions_paused ? 'All webhook actions are paused' : 'Learning ended; previous mappings are unchanged';
      const diagnostic = learning.diagnostics || {};
      const stream = diagnostic.stream || {};
      const capturedReceiver = learning.capture_receiver || {};
      const currentMode = !active && state.actions_paused ? 'restoring normal reception' : state.receiver_mode === 'learning_capture' ? 'learning capture' : 'normal reception';
      $('learning-diagnostics').textContent = [
        `Capture receiver ${active ? 'during this attempt' : 'at failure'}: ${capturedReceiver.state || 'not recorded'} — ${capturedReceiver.message || ''}`,
        `Capture plan: ${learning.capture_generation ?? 'not recorded'}; status time: ${learning.capture_receiver_at || 'not recorded'}`,
        `Current reception: ${currentMode} — ${state.receiver.state || 'unknown'} — ${state.receiver.message || ''}`,
        `Mode: ${learning.automatic ? 'Automatic learning: decoded events and pulse discovery' : 'Decoded signal / demonstration'}`,
        `Packet records seen: ${diagnostic.observed_packets || 0}; accepted for replay: ${diagnostic.packets || 0}; rejected: ${diagnostic.rejected_packets || 0}; outside capture window / during cleanup: ${diagnostic.discarded_packets || 0}`,
        `Observed pulses: ${diagnostic.pulses || 0}; packet bytes: ${diagnostic.bytes || 0}; accepted bytes: ${diagnostic.accepted_bytes || 0}`,
        `OOK packets: ${diagnostic.ook_packets || 0}; FSK packets: ${diagnostic.fsk_packets || 0}; malformed: ${diagnostic.malformed_packets || 0}; empty: ${diagnostic.empty_packets || 0}`,
        `Stdout read: ${stream.bytes || 0} bytes / ${stream.lines || 0} lines; JSON records: ${stream.json_records || 0}; pulse headers: ${stream.headers || 0}; end boundaries: ${stream.completed || 0}`,
        `Pulse stream: ${stream.closed ? 'closed' : 'open'}${stream.packet_incomplete ? `; incomplete packet (${stream.current_rows || 0}/${stream.expected_pulses || 0} rows)` : ''}`,
        diagnostic.failure_code ? `Failure: ${diagnostic.failure_code} — ${diagnostic.failure_reason || ''}` : '',
        diagnostic.rejection_code ? `Last rejection: ${diagnostic.rejection_code} — ${diagnostic.rejection_reason || ''}` : '',
        `Analyzer candidates: ${diagnostic.candidates || 0}; offline replays: ${diagnostic.replays || 0}`,
        `Captures analyzed: ${diagnostic.analysis_attempts || 0}; captures rejected: ${diagnostic.rejected_captures || 0}; this window: ${learning.window_analyses || 0}/${learning.window_analysis_limit || 0} analysis attempts`,
        ...(diagnostic.recent_rejections || []).map(r => `Capture ${r.attempt}, press window ${r.press_window}: ${r.code} — ${r.reason}; packets: ${r.packets}; bytes: ${r.bytes}; candidates: ${r.candidates}; replays: ${r.replays}`),
        `Decoder: ${learning.profile?.name || diagnostic.source || 'No validated profile yet'}`,
        diagnostic.detail || '',
      ].filter(Boolean).join('\n');
      setError('learning-error', '');
    }
    const expiry = new Date(learning.expires_at).getTime();
    const seconds = Math.max(0, Math.ceil((expiry - Date.now()) / 1000));
    const windowExpiry = new Date(learning.window_expires_at).getTime();
    const windowSeconds = Math.max(0, Math.ceil((windowExpiry - Date.now()) / 1000));
    const waiting = ['waiting', 'receiving', 'analyzing'].includes(learning.stage);
    $('learning-expiry').textContent = active && Number.isFinite(expiry) ? `${waiting && Number.isFinite(windowExpiry) ? `Press window: ${windowSeconds}s · ` : ''}Session limit: ${seconds}s` : '';
    learningWasActive = true;
    if (!$('login-dialog').open && !$('confirm-dialog').open) show('learning-dialog');
  }
  function receiverLabel(device) {
    return [device.name || 'RTL-SDR receiver', device.serial ? `Serial ${device.serial}` : 'No serial', device.usb_port ? `USB ${device.usb_port}` : ''].filter(Boolean).join(' · ');
  }
  function renderReceiverIdentity() {
    const active = ['connected','recovering'].includes(state.receiver.state) && (state.receiver.name || state.receiver.usb_port || state.receiver.serial);
    const label = active ? receiverLabel(state.receiver) : (state.settings.receiver_serial ? `Selected serial ${state.settings.receiver_serial} · not receiving` : 'No active receiver');
    $('receiver-device').textContent = label;
    $('receiver-sensitivity-active').textContent = `Running mode: ${state.receiver.adaptive_sensitivity ? 'Adaptive' : 'Standard'}${state.receiver.state !== 'connected' ? ' · receiver not yet connected' : ''}`;
    const noise = state.receiver.noise;
    $('receiver-noise').textContent = noise && state.receiver.state === 'connected' ? `Estimated background noise: ${levelText(noise.dbfs, 'dBFS')} · ${readingTime(noise.at)}. This is receiver-wide, not specific to a button.` : 'Background noise: waiting for a measurement. Measurements normally update every 10 seconds.';
    $('receiver-settings-active').textContent = `Current receiver: ${label}`;
    const at = state.receiver_last_decoded_at;
    const recent = state.receiver.state === 'connected' && at && Date.now() - new Date(at).getTime() >= 0 && Date.now() - new Date(at).getTime() < 2500;
    $('receiver-activity').textContent = recent ? 'Decoded RF signal received' : at ? `Last decoded RF: ${fmtTime(at)}` : 'No decoded RF activity yet';
    $('receiver-activity').className = recent ? 'rf-activity recent' : 'rf-activity';
  }
  async function refreshReceiverDevices(wanted = $('receiver-serial').value) {
    const generation = ++receiverScanGeneration;
    receiverListReady = false;
    $('receiver-refresh').disabled = true;
    $('receiver-list-note').textContent = 'Scanning connected RTL-SDR receivers…';
    updateActionAvailability();
    try {
      const result = await api('/receivers');
      if (generation !== receiverScanGeneration) return;
      receiverDevices = result.devices || [];
      const options = [];
      const automatic = node('option', '', `Automatic — ${receiverDevices.length === 1 ? 'one receiver connected' : `${receiverDevices.length} receivers; choose a unique serial`}`);
      automatic.value = ''; automatic.disabled = receiverDevices.length !== 1; options.push(automatic);
      for (const [index, d] of receiverDevices.entries()) {
        const option = node('option', '', `${receiverLabel(d)}${d.selectable ? '' : ' — '+d.reason}`);
        option.value = d.selectable ? d.serial : `!unavailable-${index}`;
        option.disabled = !d.selectable; options.push(option);
      }
      if (wanted && !receiverDevices.some(d => d.selectable && d.serial === wanted)) {
        const saved = node('option', '', `Saved serial ${wanted} — missing or ambiguous`);
        saved.value = wanted; saved.disabled = true; options.push(saved);
      }
      $('receiver-serial').replaceChildren(...options);
      $('receiver-serial').value = wanted || '';
      receiverListReady = true;
      const problems = receiverDevices.filter(d => !d.selectable).map(d => `${d.usb_port ? 'USB '+d.usb_port+': ' : ''}${d.reason}`);
      $('receiver-list-note').textContent = `${receiverDevices.length} receiver(s) detected. ${problems.join('. ') || 'Choose a receiver, then Save settings. Detection does not prove USB access or sample acquisition.'}`;
    } catch (error) {
      if (generation !== receiverScanGeneration) return;
      $('receiver-list-note').textContent = 'Device scan failed. Your saved receiver selection has not changed.';
      handleError(error, 'settings-error');
    } finally {
      if (generation === receiverScanGeneration) { $('receiver-refresh').disabled = false; updateActionAvailability(); }
    }
  }
  function render() {
    $('demo-banner').hidden = !state.demo;
    const status = state.receiver.state || (token ? 'connecting' : 'locked');
    $('receiver-state').textContent = status;
    $('receiver-message').textContent = state.receiver.message || (token ? 'Waiting for receiver status' : 'Enter a token to connect');
    $('receiver-dot').className = `status-dot ${['connected', 'disconnected', 'recovering', 'error'].includes(status) ? status : ''}`;
    renderReceiverIdentity();
    $('login-version').textContent = state.version ? `Version ${state.version}` : '';
    $('version-label').textContent = state.version ? `RF433 Button Manager · ${state.version}` : 'RF433 Button Manager';
    renderNavigation(); renderRemote(); renderEditor(); renderEvents(); renderLearning(); updateActionAvailability();
  }

  $('login-form').addEventListener('submit', async (event) => {
    event.preventDefault();
    $('login-submit').disabled = true; setError('login-error', '');
    token = $('management-token').value.trim();
    try {
      await refresh();
      try { sessionStorage.setItem('rfbm-token', token); } catch (_) { /* Token remains in memory only. */ }
      close('login-dialog'); $('management-token').value = '';
      startPolling();
    } catch (error) { token = ''; setError('login-error', error.message); }
    finally { $('login-submit').disabled = false; }
  });
  $('login-dialog').addEventListener('cancel', (e) => e.preventDefault());
  bindAction('logout', async () => { if (await mayDiscard()) lock(); });
  function applyTheme(theme) {
    document.documentElement.dataset.theme = theme;
    $('theme-toggle').setAttribute('aria-label', `Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`);
    $('appearance-theme').value = theme;
    try { localStorage.setItem('rfbm-theme', theme); } catch (_) { /* Nonessential preference. */ }
  }
  let theme = 'dark';
  try { theme = localStorage.getItem('rfbm-theme') || 'dark'; } catch (_) { /* Default dark. */ }
  applyTheme(theme === 'light' ? 'light' : 'dark');
  bindAction('theme-toggle', async () => applyTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark'));
  $('appearance-theme').addEventListener('change', () => applyTheme($('appearance-theme').value));
  document.querySelectorAll('[data-close]').forEach((button) => button.addEventListener('click', () => close(button.dataset.close)));

  function openRemote(mode) {
    remoteMode = mode;
    setError('remote-error', '');
    const remote = currentRemote();
    $('remote-dialog-title').textContent = mode === 'edit' ? 'Edit remote' : 'Create a remote';
    $('remote-submit').textContent = mode === 'edit' ? 'Save remote' : 'Create remote';
    $('remote-name').value = mode === 'edit' ? remote.name : 'Six-button remote';
    $('remote-columns').value = String(mode === 'edit' ? remote.columns : 2);
    $('remote-button-count').value = '6';
    $('button-count-field').hidden = mode === 'edit';
    $('remote-extra-actions').hidden = mode !== 'edit';
    $('remote-dialog-hint').textContent = mode === 'edit' ? 'Changing columns rearranges the same buttons. Duplicating creates an unlearned, disabled copy.' : 'Six buttons use A ON / A OFF, B ON / B OFF, and C ON / C OFF.';
    show('remote-dialog');
  }
  bindAction('remote-create-open', async () => openRemote('create'));
  bindAction('remote-create-empty', async () => openRemote('create'));
  bindAction('remote-edit-open', async () => openRemote('edit'));
  $('remote-form').addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      if (remoteMode === 'create' && !(await mayDiscard())) return;
      const payload = { name: $('remote-name').value.trim(), columns: Number($('remote-columns').value) };
      if (!payload.name) throw new Error('Enter a remote name.');
      if (remoteMode === 'create') payload.button_count = Number($('remote-button-count').value);
      const result = await mutate(remoteMode === 'create' ? '/remotes' : `/remotes/${encodeURIComponent(selectedRemote)}`, remoteMode === 'create' ? 'POST' : 'PUT', payload);
      if (remoteMode === 'create') { selectedRemote = result.id; selectedButton = null; editorDirty = false; }
      close('remote-dialog'); render(); toast(remoteMode === 'create' ? 'Remote created.' : 'Remote updated.');
    } catch (error) { handleError(error); }
  });
  bindAction('remote-duplicate', async () => {
    if (!(await mayDiscard())) return;
    const result = await mutate(`/remotes/${encodeURIComponent(selectedRemote)}/duplicate`);
    selectedRemote = result.id; selectedButton = null; editorDirty = false;
    close('remote-dialog'); render(); toast('Remote duplicated. Learn its buttons and review actions before enabling.');
  });
  bindAction('remote-delete', async () => {
    if (!(await ask('Delete this remote?', `Delete ${currentRemote()?.name || 'this remote'} and all of its buttons and mappings?`, 'Delete remote'))) return;
    await mutate(`/remotes/${encodeURIComponent(selectedRemote)}`, 'DELETE');
    selectedButton = null; editorDirty = false; close('remote-dialog'); render(); toast('Remote deleted.');
  });
  bindAction('button-create', async () => {
    setError('name-error', '');
    $('new-button-name').value = `Button ${(currentRemote()?.buttons || []).length + 1}`;
    show('name-dialog');
  });
  $('name-form').addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      if (!(await mayDiscard())) return;
      const result = await mutate(`/remotes/${encodeURIComponent(selectedRemote)}/buttons`, 'POST', { name: $('new-button-name').value.trim() });
      selectedButton = result.id; editorDirty = false; close('name-dialog'); fillEditor(); render(); toast('Button added.');
    } catch (error) { handleError(error); }
  });
  $('button-editor').addEventListener('input', (event) => {
    if (event.target.id === 'ha-id' || event.target.id === 'ha-base' || event.target.id === 'button-fields') return;
    editorDirty = true; renderEditor();
  });
  $('button-editor').addEventListener('change', (event) => {
    if (event.target.id === 'ha-id' || event.target.id === 'ha-base' || event.target.id === 'button-fields') return;
    editorDirty = true; renderEditor();
  });
  function buttonPayload() {
    const existing = currentButton();
    if (!existing) throw new Error('Select a button first.');
    let headers = {};
    const rawHeaders = $('webhook-headers').value.trim();
    if (rawHeaders) {
      try { headers = JSON.parse(rawHeaders); } catch (_) { throw new Error('Headers must be valid JSON.'); }
      if (headers === null || Array.isArray(headers) || typeof headers !== 'object' || Object.values(headers).some((v) => typeof v !== 'string')) throw new Error('Headers must be a JSON object with string values.');
    }
    const payload = $('webhook-payload').value.trim();
    if (payload) { try { JSON.parse(payload); } catch (_) { throw new Error('Payload must be valid JSON, or empty.'); } }
    const name = $('button-name').value.trim();
    if (!name) throw new Error('Enter a button name.');
    const identityFields = fields($('button-fields').value);
    const oldFields = existing.identity_fields || [];
    const destination = webhookDestination($('webhook-url').value.trim());
    return {
      ...(destination?.needsApproval ? { allow_webhook_origin: destination.origin } : {}),
      id: existing.id, name, enabled: $('button-enabled').checked,
      debounce_ms: Number($('button-debounce').value), identity_fields: existing.signature ? oldFields : identityFields,
      webhook: { url: $('webhook-url').value.trim(), method: $('webhook-method').value,
        timeout_ms: Number($('webhook-timeout').value), payload, headers },
    };
  }
  $('button-editor').addEventListener('submit', async (event) => {
    event.preventDefault(); setError('editor-error', '');
    try {
      await mutate(`/buttons/${encodeURIComponent(selectedButton)}`, 'PUT', buttonPayload());
      editorDirty = false; fillEditor(); render(); toast('Button saved.');
    } catch (error) { handleError(error, 'editor-error'); }
  });
  bindAction('ha-apply', async () => {
    const value = $('ha-id').value.trim();
    if (!value) throw new Error('Paste a Home Assistant webhook ID or complete URL.');
    if (/^[a-z][a-z0-9+.-]*:/i.test(value) || value.startsWith('//')) {
      webhookDestination(value);
      $('webhook-url').value = value;
      $('ha-base').value = new URL(value).origin;
    } else {
      const base = new URL($('ha-base').value.trim());
      if (!['http:', 'https:'].includes(base.protocol) || base.username || base.password || base.search || base.hash || !['', '/'].includes(base.pathname)) throw new Error('Use a plain HTTP(S) Home Assistant origin, without credentials or a path.');
      $('webhook-url').value = `${base.origin}/api/webhook/${encodeURIComponent(value)}`;
    }
    $('webhook-method').value = 'POST'; $('ha-id').value = '';
    editorDirty = true; renderEditor(); toast('Home Assistant URL filled in. Save the button to apply it.');
  }, 'editor-error');
  bindAction('webhook-test', async () => {
    if (editorDirty) throw new Error('Save changes before testing.');
    if (!(await ask('Send a real webhook?', `This runs the saved action for ${currentButton()?.name || 'this button'}.`, 'Send test'))) return;
    await mutate(`/buttons/${encodeURIComponent(selectedButton)}/test`, 'POST', {}, 'Test submitted. Check the event monitor for its result.');
  }, 'editor-error');
  bindAction('button-clear', async () => {
    if (!(await ask('Clear button configuration?', 'Remove the learned signal and webhook action, and disable this button?', 'Clear configuration'))) return;
    await mutate(`/buttons/${encodeURIComponent(selectedButton)}/clear`);
    editorDirty = false; fillEditor(); render(); toast('Button configuration cleared.');
  }, 'editor-error');
  bindAction('button-delete', async () => {
    if (!(await ask('Delete this button?', `Delete ${currentButton()?.name || 'this button'} and its mapping?`, 'Delete button'))) return;
    await mutate(`/buttons/${encodeURIComponent(selectedButton)}`, 'DELETE');
    selectedButton = null; editorDirty = false; render(); toast('Button deleted.');
  }, 'editor-error');
  bindAction('learn-start', async () => {
    if (editorDirty) throw new Error('Save button changes before learning.');
    setError('learning-error', '');
    await mutate('/learn/start', 'POST', { button_id: selectedButton, identity_fields: $('learning-automatic-fields').checked ? DEFAULT_FIELDS : fields($('button-fields').value) });
  }, 'editor-error');
  bindAction('inspect-open', async () => {
    setError('learning-error', '');
    await mutate('/inspect/start');
  });
  bindAction('inspection-export', async () => {
    const report = await api('/inspect/result');
    const url = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2) + '\n'], {type: 'application/json'}));
    const link = node('a'); link.href = url; link.download = 'rf433-button-manager-inspection.json';
    document.body.append(link); link.click(); link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    toast('Inspection report downloaded. RF identifiers are included; application credentials are excluded.');
  }, 'learning-error');
  bindAction('learning-export', async () => {
    const capture = await api('/learn/capture');
    const blob = new Blob([JSON.stringify(capture, null, 2) + '\n'], {type: 'application/json'});
    const url = URL.createObjectURL(blob);
    const link = node('a'); link.href = url; link.download = 'rf433-button-manager-capture.json';
    document.body.append(link); link.click(); link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    toast('RF capture downloaded. It contains radio identifiers, without application credentials.');
  }, 'learning-error');
  bindAction('learning-arm', async () => { await mutate('/learn/arm'); }, 'learning-error');
  bindAction('learning-retry', async () => {
    const previous = state.learning_notice;
    if (!previous) return;
    if (previous.inspect_only) await mutate('/inspect/start');
    else await mutate('/learn/start', 'POST', {button_id: previous.button_id, identity_fields: previous.identity_fields});
  }, 'learning-error');
  bindAction('learning-confirm', async () => {
    if (state.learning?.inspect_only) {
      await mutate('/inspect/finish'); render(); toast('Inspection finished. No mapping or webhook was changed.'); return;
    }
    await mutate('/learn/confirm'); fillEditor(); render(); toast('RF signal saved. Review the webhook, then enable this button.');
  }, 'learning-error');
  async function cancelLearning() { await mutate('/learn/cancel'); }
  bindAction('learning-cancel', cancelLearning, 'learning-error');
  $('learning-dialog').addEventListener('cancel', async (event) => {
    event.preventDefault();
    try { await cancelLearning(); } catch (error) { handleError(error, 'learning-error'); }
  });
  bindAction('learning-reopen', async () => show('learning-dialog'));
  $('event-filter').addEventListener('change', renderEvents);

  bindAction('settings-open', async () => {
    if (settingsLoaded) { show('settings-dialog'); await loadManagement(); return; }
    const settings = state.settings;
    receiverListReady = false;
    $('receiver-flex').value = settings.flex_decoder || '';
    $('receiver-protocols').value = (settings.protocols || []).join(', ');
    $('sensitivity-adaptive').checked = !!settings.adaptive_sensitivity;
    $('sensitivity-standard').checked = !settings.adaptive_sensitivity;
    $('settings-identity-fields').value = (settings.identity_fields?.length ? settings.identity_fields : DEFAULT_FIELDS).join(', ');
    $('allowed-origins').value = (settings.allowed_origins || []).join('\n');
    setError('settings-error', ''); show('settings-dialog');
    await refreshReceiverDevices(settings.receiver_serial || '');
    settingsLoaded = true;
    await loadManagement();
  });
  bindAction('reception-reset', async () => {
    const id = currentButton()?.id;
    if (id) await mutate(`/buttons/${id}/reception/reset`, 'POST', {}, 'Reception counts reset. Action settings are unchanged.');
  }, 'editor-error');
  bindAction('receiver-refresh', () => refreshReceiverDevices(), 'settings-error');
  $('settings-form').addEventListener('submit', async (event) => {
    event.preventDefault(); setError('settings-error', '');
    try {
      if (!receiverListReady) throw new Error('Refresh devices before saving settings.');
      const serial = $('receiver-serial').value;
      if (serial !== (state.settings.receiver_serial || '') && !(serial === '' ? receiverDevices.length === 1 : receiverDevices.some(d => d.selectable && d.serial === serial))) throw new Error('Choose an available receiver with a unique serial.');
      const protocolParts = fields($('receiver-protocols').value);
      if (protocolParts.some((v) => !/^\d+$/.test(v) || Number(v) < 1 || Number(v) > 10000)) throw new Error('Protocol IDs must be comma-separated integers between 1 and 10000.');
      const origins = [...new Set($('allowed-origins').value.split(/\n/).map((v) => v.trim()).filter(Boolean))];
      for (const origin of origins) {
        const url = new URL(origin);
        if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || !['', '/'].includes(url.pathname)) throw new Error('Each allowlist entry must be an HTTP(S) origin without a path or credentials.');
      }
      await mutate('/settings', 'PUT', {
        adaptive_sensitivity: $('sensitivity-adaptive').checked,
        receiver_serial: $('receiver-serial').value.trim(), flex_decoder: $('receiver-flex').value.trim(),
        protocols: protocolParts.map(Number), identity_fields: fields($('settings-identity-fields').value),
        allowed_origins: origins.map((value) => new URL(value).origin),
      });
      close('settings-dialog'); toast('Settings saved. Receiver changes restart the decoding process.');
    } catch (error) { handleError(error, 'settings-error'); }
  });
  async function downloadRecording() {
    const report = await api('/reception/recording');
    const url = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2) + '\n'], {type: 'application/json'}));
    const link = node('a'); link.href = url; link.download = 'rf433-button-manager-reception.json';
    document.body.append(link); link.click(); link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    toast('Recording downloaded. Include how many physical presses you made and how many actions worked.');
  }
  bindAction('recording-start', async () => {
    if (editorDirty) throw new Error('Save button changes before recording.');
    if (selectedButton) await mutate(`/buttons/${selectedButton}/reception/record`, 'POST', {});
  }, 'recording-error');
  bindAction('recording-stop', async () => { await mutate('/reception/recording/stop', 'POST', {}); await downloadRecording(); }, 'recording-error');
  bindAction('recording-download', downloadRecording, 'recording-error');
  bindAction('backup-download', async () => {
    const backup = await api('/backup');
    const blob = new Blob([JSON.stringify(backup, null, 2) + '\n'], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = node('a'); link.href = url; link.download = `rf433-button-manager-backup-${new Date().toISOString().slice(0, 10)}.json`;
    document.body.append(link); link.click(); link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    toast('Backup downloaded. It contains secrets; store it privately.');
  }, 'settings-error');
  bindAction('backup-restore', async () => { $('backup-file').value = ''; $('backup-file').click(); });
  $('backup-file').addEventListener('change', async () => {
    const file = $('backup-file').files[0];
    if (!file) return;
    try {
      if (file.size > 2 * 1024 * 1024) throw new Error('Backup is too large (maximum 2 MiB).');
      let backup;
      try { backup = JSON.parse(await file.text()); } catch (_) { throw new Error('The selected file is not valid JSON.'); }
      if (![1, 2].includes(backup.schema_version) || !Array.isArray(backup.remotes) || !backup.settings) throw new Error('This file is not a supported RF433 Button Manager backup.');
      if (!(await ask('Replace current configuration?', `Restore ${file.name}? All current remotes and settings will be replaced. Restored actions remain disabled until you review and enable them.`, 'Restore backup'))) return;
      await mutate('/restore', 'POST', backup);
      settingsLoaded = false;
      editorDirty = false; selectedButton = null; close('settings-dialog'); render();
      toast('Configuration restored. Review every action before enabling buttons.');
    } catch (error) { handleError(error, 'settings-error'); }
  });
  async function copyText(text) {
    try { if (navigator.clipboard && window.isSecureContext) { await navigator.clipboard.writeText(text); return; } } catch (_) {}
    const previous = document.activeElement;
    const field = node('textarea'); field.value = text; field.readOnly = true;
    field.style.position = 'fixed'; field.style.opacity = '0'; document.body.append(field); field.select();
    try { if (!document.execCommand('copy')) throw new Error('Select the text and copy it manually.'); }
    finally { field.remove(); previous?.focus(); }
  }
  function renderRemotePicker() {
    const query = $('remote-picker-search').value.trim().toLowerCase();
    const list = document.createDocumentFragment();
    for (const remote of state.remotes.filter(r => r.name.toLowerCase().includes(query))) {
      const button = node('button', 'picker-option'); button.type = 'button';
      button.setAttribute('aria-current', remote.id === selectedRemote ? 'true' : 'false');
      const label = node('span'); label.append(node('strong', '', remote.name), node('small', '', `${remote.buttons.length} buttons`));
      button.append(label, node('span', 'picker-mark', remote.id === selectedRemote ? '✓' : '›'));
      button.addEventListener('click', async () => {
        if (remote.id !== selectedRemote && !(await mayDiscard())) return;
        if (remote.id !== selectedRemote) { selectedRemote = remote.id; selectedButton = null; editorDirty = false; }
        document.documentElement.dataset.editor = 'closed'; close('remote-picker-dialog'); render();
      });
      list.append(button);
    }
    $('remote-picker-list').replaceChildren(list);
    if (!$('remote-picker-list').children.length) $('remote-picker-list').append(node('p', 'hint', 'No remotes match this search.'));
  }
  function isLoopback(address) { return address === '::1' || /^127\./.test(address); }
  function configURLs(c) {
    const host = c.address.includes(':') ? `[${c.address}]` : c.address;
    return [...(c.http ? [`http://${host}:${c.http_port}`] : []), ...(c.https ? [`https://${host}:${c.https_port}`] : [])];
  }
  function networkDraft() {
    const enabled = $('network-enabled').checked;
    return {address: enabled ? networkAddress : '127.0.0.1', http: !enabled || $('network-http').checked, https: enabled && $('network-https').checked, http_port: Number($('network-http-port').value), https_port: Number($('network-https-port').value)};
  }
  function networkKey(c) { return JSON.stringify([c.address,c.http,c.http_port,c.https,c.https_port]); }
  function fillNetwork(c) {
    $('network-enabled').checked = !isLoopback(c.address);
    networkAddress = c.address;
    $('network-http').checked = !c.https; $('network-https').checked = c.https;
    $('network-http-port').value = c.http_port; $('network-https-port').value = c.https_port;
  }
  function renderNetworkDraft() {
    if (!networkState) return;
    const enabled = $('network-enabled').checked, address = networkAddress;
    const info = (networkState.interfaces || []).find(i => i.address === address);
    $('network-address-choice').hidden = !enabled;
    $('network-address-label').textContent = address && !isLoopback(address) ? address : 'Choose a LAN or VPN address';
    $('network-interface-label').textContent = info && !isLoopback(address) ? info.interface : 'An address assigned to the Linux server';
    $('network-http').disabled = !enabled; $('network-https').disabled = !enabled;
    $('network-https-help').hidden = !enabled || !$('network-https').checked;
    $('network-legacy-both').hidden = !(networkState.config.http && networkState.config.https);
    const pending = !!networkState.pending;
    const changed = networkKey(networkDraft()) !== networkKey(networkState.config);
    $('network-fields').disabled = pending;
    $('network-stage').disabled = pending || !changed;
    $('network-draft-status').textContent = pending ? 'Waiting for confirmation. Current access stays available until then.' : changed ? 'Unsaved changes. Apply, open the proposed address, then confirm there within 3 minutes. Without confirmation, your current settings stay in place.' : 'Current settings saved. Changes here affect this app only.';
  }
  function adjustNetworkPort(protocol, change) {
    if (networkState?.pending) return;
    const input = $(`network-${protocol}-port`);
    const current = Number(input.value);
    if (change !== 'reset' && (!String(input.value).trim() || !Number.isInteger(current) || current < 1024 || current > 65535)) {
      setError('network-error', 'Enter a whole port number from 1024 to 65535, or use Reset.'); return;
    }
    input.value = String(change === 'reset' ? (protocol === 'http' ? 8787 : 8788) : Math.min(65535, Math.max(1024, current + change)));
    setError('network-error', ''); renderNetworkDraft();
  }
  function renderAddressPicker() {
    const list = document.createDocumentFragment();
    const items = (networkState?.interfaces || []).filter(i => !isLoopback(i.address));
    for (const item of items) {
      const button = node('button', 'picker-option'); button.type = 'button';
      const vpn = /^(tailscale|wg|tun|tap)/i.test(item.interface), virtual = /^(docker|br-|veth|virbr)/i.test(item.interface);
      const label = node('span'); label.append(node('strong', '', item.address), node('small', '', `${item.interface} · ${vpn ? 'VPN connection' : virtual ? 'Virtual network — advanced' : 'Network interface'}`));
      const selected = item.address === networkAddress;
      button.setAttribute('aria-current', selected ? 'true' : 'false');
      button.append(label, node('span', 'picker-mark', selected ? '✓' : '›'));
      button.addEventListener('click', () => { networkAddress = item.address; close('network-address-dialog'); renderNetworkDraft(); });
      list.append(button);
    }
    $('network-address-list').replaceChildren(list);
    if (!items.length) $('network-address-list').append(node('p', 'hint', 'No LAN or VPN address is currently available. Connect the server to your network, then reopen this picker. Local access still works.'));
  }
  function showURLCards() {
    const c = networkState.config, remoteURLs = isLoopback(c.address) ? [] : configURLs(c);
    const localURLs = networkState.local_urls || configURLs({...c,address:'127.0.0.1',http:true,https:false});
    const key = JSON.stringify([remoteURLs,localURLs]);
    if (key === networkURLKey) return;
    networkURLKey = key;
    for (const [id,urls] of [['network-current-urls',remoteURLs],['network-local-urls',localURLs]]) {
      $(id).replaceChildren();
      for (const url of urls) {
        const row = node('div', 'url-row'), code = node('code', '', url), button = node('button', 'secondary compact', 'Copy URL'); button.type = 'button';
        button.setAttribute('aria-label', `Copy ${url}`);
        button.addEventListener('click', async () => { try { await copyText(url); toast('URL copied.'); } catch (e) { setError('network-error', e.message); } });
        row.append(code,button); $(id).append(row);
      }
      if (!urls.length) $(id).append(node('p', 'hint', 'Network access is off. Only the Linux server can connect.'));
    }
  }
  function showNetworkStatus() {
    if (!networkState) return;
    const pending = networkState.pending;
    $('network-pending').hidden = !pending;
    const oldLink = $('network-links').children[0]?.href;
    if (pending) {
      const seconds = Math.max(0, Math.ceil((new Date(pending.expires_at).getTime() - Date.now()) / 1000));
      const c = pending.config, urls = configURLs(c), url = c.https ? urls.find(u => u.startsWith('https:')) : urls[0];
      const here = typeof location !== 'undefined' && location.origin === url;
      $('network-pending-message').textContent = `${seconds} seconds left to confirm. If you do not confirm, the previous settings are kept automatically.`;
      if (oldLink !== url) { const a = node('a', '', url); a.href = url; a.target = '_blank'; a.rel = 'noopener noreferrer'; $('network-links').replaceChildren(a); }
      $('network-confirm').disabled = !here;
      const localOnly = isLoopback(c.address);
      $('network-confirm-help').textContent = here ? 'This tab is using the proposed connection. Confirm to save it, or cancel to keep the previous settings.' : localOnly ? 'Open this localhost address in a browser ON THE LINUX SERVER (or through an SSH tunnel). Log in, open Settings and confirm there. Opening it on your phone or Windows PC will not reach the server.' : 'Open the proposed address in a new tab, log in with your existing token and return to Settings to confirm. Keep this tab open until you finish. The token is never included in the link.';
    } else { $('network-links').replaceChildren(); }
    const cert = networkState.certificate || {};
    $('certificate-status').textContent = cert.expires_at ? `Server certificate expires ${new Date(cert.expires_at).toLocaleDateString()}. Trust certificate SHA-256:` : 'Download to create this server’s local trust certificate.';
    $('certificate-fingerprint').textContent = cert.ca_sha256 ? cert.ca_sha256.toUpperCase().match(/.{2}/g).join(':') : '';
    $('certificate-renew').disabled = !!pending;
    showURLCards(); renderNetworkDraft();
  }
  async function loadManagement(fill = false) {
    try {
      const next = await api('/manage/network');
      const changedElsewhere = networkLoaded && networkSavedKey !== networkKey(next.config);
      const endedPending = networkPendingID && !next.pending;
      networkState = next;
      if (!networkLoaded || fill || changedElsewhere || endedPending) {
        fillNetwork(networkState.pending?.config || networkState.config);
        networkLoaded = true;
      }
      networkSavedKey = networkKey(networkState.config); networkPendingID = networkState.pending?.id || null;
      showNetworkStatus();
    } catch (error) { setError('network-error', error.message); }
    clearTimeout(networkTimer);
    if (token && activeView === 'settings') networkTimer = setTimeout(() => loadManagement(), 2000);
  }
  function initializeNavigation() {
    bindAction('layout-toggle', () => applyLayout(layout === 'classic' ? 'unified' : 'classic'));
    bindAction('view-remotes', () => setView('remotes'));
    bindAction('view-activity', () => setView('activity'));
    bindAction('editor-back', () => { document.documentElement.dataset.editor = 'closed'; });
    bindAction('compact-add-remote', () => { close('remote-picker-dialog'); $('remote-create-open').click(); });
    $('remote-search').addEventListener('input', renderNavigation);
    $('remote-picker-search').addEventListener('input', renderRemotePicker);
    bindAction('remote-picker', () => { $('remote-picker-search').value = ''; renderRemotePicker(); show('remote-picker-dialog'); });
    bindAction('network-address-picker', () => { renderAddressPicker(); show('network-address-dialog'); });
    $('network-form').addEventListener('input', renderNetworkDraft);
    for (const protocol of ['http', 'https']) {
      bindAction(`network-${protocol}-minus`, () => adjustNetworkPort(protocol, -1), 'network-error');
      bindAction(`network-${protocol}-plus`, () => adjustNetworkPort(protocol, 1), 'network-error');
      bindAction(`network-${protocol}-reset`, () => adjustNetworkPort(protocol, 'reset'), 'network-error');
    }
    $('network-enabled').addEventListener('change', () => {
      renderNetworkDraft();
      if ($('network-enabled').checked && isLoopback(networkAddress)) { renderAddressPicker(); show('network-address-dialog'); }
    });
    bindAction('token-visibility', () => {
      const show = $('management-token').type === 'password'; $('management-token').type = show ? 'text' : 'password';
      $('token-visibility').textContent = show ? 'Hide' : 'Show'; $('token-visibility').setAttribute('aria-label', `${show ? 'Hide' : 'Show'} management token`);
    });
    bindAction('token-command-copy', async () => { await copyText($('token-command').textContent); $('copy-command-status').textContent = 'Command copied. Run it on your Linux server.'; }, 'copy-command-status');
    $('login-transport').textContent = typeof location !== 'undefined' && location.protocol === 'https:' ? 'HTTPS connection. Verify that your browser trusts this server’s certificate.' : 'HTTP does not encrypt your token. Use HTTPS or a trusted VPN; keep this interface private.';
    $('network-form').addEventListener('submit', async e => {
      e.preventDefault(); setError('network-error', '');
      try {
        const draft = networkDraft();
        if ($('network-enabled').checked && (!draft.address || isLoopback(draft.address))) throw new Error('Choose the server’s LAN or VPN address before allowing network access.');
        if (isLoopback(draft.address) && !isLoopback(networkState.config.address) && !(await ask('Turn off network access?', 'To finish this change, you must open the localhost URL on the Linux server, or through an SSH tunnel, and confirm there. Other devices will then lose access. Without confirmation, your current network access is kept.', 'Test local-only access'))) return;
        await api('/manage/network/stage', 'POST', draft);
        await loadManagement();
      } catch (error) { setError('network-error', error.message); }
    });
    bindAction('network-confirm', async () => {
      if (!networkState?.pending) throw new Error('No pending network change.');
      await api('/manage/network/confirm', 'POST', {id: networkState.pending.id}); await loadManagement(true); toast('Connection confirmed and saved.');
    }, 'network-error');
    bindAction('network-cancel', async () => { await api('/manage/network/cancel', 'POST', {}); await loadManagement(true); toast('Previous network settings retained.'); }, 'network-error');
    bindAction('certificate-download', async () => {
      await api('/manage/certificate/prepare', 'POST', {});
      const response = await fetch('/api/manage/certificate', {headers:{Authorization:`Bearer ${token}`},cache:'no-store',redirect:'error',credentials:'same-origin'});
      if (!response.ok) throw new Error('Certificate download failed.');
      const url = URL.createObjectURL(await response.blob()), a = node('a'); a.href = url; a.download = 'rf433-button-manager-ca.crt'; document.body.append(a); a.click(); a.remove(); setTimeout(()=>URL.revokeObjectURL(url),1000);
      await loadManagement();
    }, 'network-error');
    bindAction('certificate-renew', async () => {
      if (!(await ask('Renew HTTPS certificate?', 'Generate a new server certificate using the existing trust authority. Your installed trust certificate remains valid.', 'Renew certificate'))) return;
      await api('/manage/certificate/renew', 'POST', {}); await loadManagement(); toast('HTTPS certificate renewed.');
    }, 'network-error');
    bindAction('token-rotate', async () => {
      if (!(await ask('Replace management token?', 'The current token will stop working in all browser tabs. This tab will receive the new token. Buttons and webhooks keep working.', 'Replace token'))) return;
      pollingGeneration++; clearTimeout(pollingTimer); clearTimeout(networkTimer);
      try {
        const result = await api('/manage/token/rotate', 'POST', {confirm:true});
        token = result.token;
        try { sessionStorage.setItem('rfbm-token', token); } catch (_) {}
        $('new-management-token').type = 'password'; $('new-management-token').value = token; $('new-token-panel').hidden = false;
        toast('Token replaced. Other tabs must reconnect.');
      } finally { if (token) startPolling(); }
    }, 'token-error');
    bindAction('new-token-copy', async () => { await copyText($('new-management-token').value); toast('Token copied. Keep it private.'); }, 'token-error');
    bindAction('new-token-show', () => { const show = $('new-management-token').type === 'password'; $('new-management-token').type = show ? 'text' : 'password'; $('new-token-show').textContent = show ? 'Hide' : 'Show'; });
    bindAction('new-token-dismiss', () => { $('new-management-token').value = ''; $('new-token-panel').hidden = true; });
  }
  window.addEventListener('beforeunload', (event) => { if (editorDirty) { event.preventDefault(); event.returnValue = ''; } });
  initializeNavigation();
  render();
  if (token) startPolling(); else show('login-dialog');
})();
