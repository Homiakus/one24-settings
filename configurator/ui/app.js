// ═══════════════════════════════════════════════════════════════════════════
// ONEPAP.24 — Modbus Configurator  |  Enterprise Dashboard  |  Zero Radius
// ═══════════════════════════════════════════════════════════════════════════

const state = {
  connected: false,
  steps: [],
  detection: { reagent_empty: false, reagent_empty_delta: 10 },
  valves1: [],
  valves2: [],
  log: [],
  activeSelector: 1,
  selectorPos: { 1: 1, 2: 1 }
};

const API = '/api/v1';
const PAGE_NAMES = {
  settings: 'Настройки окраски',
  valves: 'Настройки переключающих клапанов',
  programs: 'Запуск программ',
  testing: 'Датчики',
  diagnostics: 'Диагностика'
};

// ─── API ────────────────────────────────────────────────────────────────────
async function apiGet(p) { const r = await fetch(API + p); return r.json(); }
async function apiPost(p, b) {
  const r = await fetch(API + p, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: b ? JSON.stringify(b) : undefined });
  return r.json();
}
async function apiPut(p, b) {
  const r = await fetch(API + p, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(b) });
  return r.json();
}
async function apiDelete(p) { const r = await fetch(API + p, { method: 'DELETE' }); return r.json(); }

// ─── WebSocket ──────────────────────────────────────────────────────────────
let ws = null;
function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(proto + '//' + location.host + '/ws/events?topics=status,progress,sensors,reagent,error,log');
  ws.onmessage = e => { try { handleWSEvent(JSON.parse(e.data)); } catch(ex){} };
  ws.onclose = () => setTimeout(connectWS, 3000);
  ws.onerror = () => {};
}
function handleWSEvent(evt) {
  switch (evt.type) {
    case 'status': updateStatusFromWS(evt.data); break;
    case 'progress': updateProgress(evt.data); break;
    case 'sensors': renderSensors(evt.data); break;
    case 'log': addLogEntry(evt.data.level, evt.data.message); break;
    case 'error': addLogEntry('ERROR', evt.data.message || evt.data.code); break;
    case 'reagent_low': showReagentDialog(evt.data); break;
    case 'connect':
      state.connected = true;
      updateStatusUI({connected: true});
      updateKPIs({connected: true, ready_status: '—', status_error: 0});
      $('#conn-error').textContent = '';
      break;
    case 'disconnect':
      state.connected = false;
      updateStatusUI({connected: false});
      updateKPIs({connected: false, ready_status: '—', status_error: 0});
      break;
  }
}

// ─── INIT ───────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  connectWS();
  initTabs();
  initButtons();
  refreshStatus();
  renderStepsTable();
  renderValvesTables();
  renderSensorPlaceholders();
});

function $(s) { const el = document.querySelector(s); if (!el) console.warn('null:', s); return el; }

// ─── TABS ───────────────────────────────────────────────────────────────────
function initTabs() {
  document.querySelectorAll('.sb-item').forEach(item => {
    item.addEventListener('click', () => {
      document.querySelectorAll('.sb-item').forEach(i => i.classList.remove('active'));
      document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
      item.classList.add('active');
      const tab = item.dataset.tab;
      const panel = document.getElementById('tab-' + tab);
      if (panel) panel.classList.add('active');
      const tbPage = $('#tb-page');
      if (tbPage) tbPage.textContent = PAGE_NAMES[tab] || tab;
    });
  });
}

// ─── BUTTONS ────────────────────────────────────────────────────────────────
function initButtons() {
  $('#btn-connect').onclick = doConnect;
  $('#btn-disconnect').onclick = doDisconnect;
  $('#btn-read-all').onclick = readAllSettings;
  $('#btn-write-all').onclick = writeAllSettings;
  $('#btn-read-valves').onclick = readAllValves;
  $('#btn-write-valves').onclick = writeAllValves;
  $('#det-delta').onchange = () => { state.detection.reagent_empty_delta = parseInt($('#det-delta').value) || 10; };
  document.querySelectorAll('.prog-btn').forEach(b => { b.onclick = () => runProgram(b.dataset.prog); });
  $('#btn-emergency').onclick = emergencyStop;
  $('#btn-read-sensors').onclick = readSensors;
  $('#btn-clear-error').onclick = clearError;
  $('#btn-clear-log').onclick = clearLog;
  $('#btn-reagent-replace').onclick = reagentReplace;
  $('#btn-reagent-cancel').onclick = reagentCancel;
  initSelectors();
}

// ─── STATUS ─────────────────────────────────────────────────────────────────
async function refreshStatus() {
  try {
    const r = await apiGet('/status');
    if (r.ok) { state.connected = r.data.connected; updateStatusUI(r.data); updateKPIs(r.data); }
  } catch(e) {}
}
function updateStatusUI(d) {
  const dot = $('#sb-dot'); const txt = $('#sb-status-text');
  if (d.connected) { dot.className = 'sb-status-dot ok'; txt.textContent = 'Подключено'; }
  else { dot.className = 'sb-status-dot err'; txt.textContent = 'Не подключено'; }
  const err = $('#conn-error');
  if (d.status_error && d.status_error !== 0) {
    err.textContent = 'ERR: ' + (d.error_name || '0x' + d.status_error.toString(16));
  } else { err.textContent = ''; }
}
function updateKPIs(d) {
  $('#kpi-status').textContent = d.connected ? 'ONLINE' : 'OFFLINE';
  $('#kpi-status').style.color = d.connected ? 'var(--success)' : 'var(--text-muted)';
  $('#kpi-ready').textContent = d.ready_status !== undefined ? d.ready_status : '—';
  const err = d.status_error;
  if (err !== undefined && err !== 0) {
    $('#kpi-error').textContent = '0x' + err.toString(16).toUpperCase();
    $('#kpi-error').style.color = 'var(--danger)';
  } else {
    $('#kpi-error').textContent = err === 0 ? 'OK' : '—';
    $('#kpi-error').style.color = err === 0 ? 'var(--success)' : 'var(--text-muted)';
  }
}
function updateStatusFromWS(d) {
  updateStatusUI(d);
  updateKPIs(d);
}

// ─── CONNECT ────────────────────────────────────────────────────────────────
async function doConnect() {
  const btn = $('#btn-connect');
  const port = $('#conn-port-inp').value.trim() || 'COM4';
  const baud = parseInt($('#conn-baud-inp').value) || 115200;
  const slave = parseInt($('#conn-slave-inp').value) || 1;

  btn.disabled = true;
  btn.textContent = '...';
  $('#conn-error').textContent = 'Подключение...';

  const r = await apiPost('/connect', { port, baudrate: baud, slave_id: slave });
  btn.disabled = false;
  btn.textContent = 'Подключить';

  if (r.ok) {
    state.connected = true;
    await refreshStatus();
    $('#conn-error').textContent = '';
  } else {
    state.connected = false;
    updateStatusUI({connected: false});
    updateKPIs({connected: false, ready_status: '—', status_error: 0});
    $('#conn-error').textContent = r.error;
  }
}
async function doDisconnect() {
  await apiPost('/disconnect');
  state.connected = false;
  updateStatusUI({connected: false});
  updateKPIs({connected: false, ready_status: '—', status_error: 0});
  refreshStatus();
}

// ─── SETTINGS ───────────────────────────────────────────────────────────────
async function readAllSettings() {
  $('#settings-log').textContent = 'Чтение...';
  try {
    const r = await apiPost('/settings/read-all');
    if (r.ok) {
      state.steps = r.data.steps; state.detection = r.data.detection;
      renderStepsTable(); $('#det-delta').value = state.detection.reagent_empty_delta;
      const ok = state.detection.reagent_empty;
      const ds = $('#det-status');
      ds.textContent = ok ? 'ПУСТО' : 'OK';
      ds.className = 'tag ' + (ok ? 'tag-err' : 'tag-ok');
      $('#settings-log').textContent = 'OK — 11 шагов';
      $('#kpi-steps').textContent = '11';
    } else { $('#settings-log').textContent = 'ERR: ' + r.error; }
  } catch(e) { $('#settings-log').textContent = 'Сеть'; }
}

function renderStepsTable() {
  if (state.steps.length === 0) {
    const n = ['Спирт 96% (фиксация)','Гематоксилин Харриса','Вода дистилл.','Вода дистилл.','Спирт 87%','OG-6','Спирт 96%','EA-50','Спирт 87%','Спирт 87%','Спирт 96%'];
    const t = [10,120,10,50,10,10,10,120,10,10,10];
    const v = [50,30,80,80,60,40,60,40,60,60,60];
    for (let i=0;i<11;i++) state.steps.push({id:i+1,name:n[i],exposure_time:t[i],fill_volume:v[i],dirty:false});
  }
  const tb = $('#steps-tbody');
  tb.innerHTML = state.steps.map((s,i) =>
    '<tr><td class="mono">' + s.id + '</td><td>' + s.name + '</td>' +
    '<td><input class="inp inp-sm' + (s.dirty ? ' dirty' : '') + '" type="number" min="1" max="600" value="' + s.exposure_time + '" data-step="' + i + '" data-field="exposure_time"></td>' +
    '<td><input class="inp inp-sm' + (s.dirty ? ' dirty' : '') + '" type="number" min="1" max="6000" value="' + s.fill_volume + '" data-step="' + i + '" data-field="fill_volume"></td></tr>'
  ).join('');
  tb.querySelectorAll('input').forEach(inp => {
    inp.addEventListener('change', () => {
      const i = parseInt(inp.dataset.step);
      state.steps[i][inp.dataset.field] = parseInt(inp.value) || 0;
      state.steps[i].dirty = true; inp.classList.add('dirty');
    });
  });
}

async function writeAllSettings() {
  // PUT все шаги на сервер (не только dirty — пишем всё)
  for (const s of state.steps) {
    await apiPut('/settings/steps/' + s.id, { exposure_time: s.exposure_time, fill_volume: s.fill_volume });
  }
  await apiPut('/settings/detection', { delta: state.detection.reagent_empty_delta });

  $('#settings-log').textContent = 'Сохранение...';
  try {
    const r = await apiPost('/settings/write-all');
    if (r.ok) {
      state.steps.forEach(s => { s.dirty = false; });
      renderStepsTable();
      $('#settings-log').textContent = 'OK: ' + r.data.written + ' параметров';
    } else {
      $('#settings-log').textContent = 'ERR: ' + r.error;
    }
  } catch(e) { $('#settings-log').textContent = 'Сеть'; }
}

// ─── VALVES ─────────────────────────────────────────────────────────────────
const DEFAULT_HOLE_NAMES = [
  'Исходное положение (Home)',
  'Воздух',
  'Гематоксилин Харриса',
  'Воздух',
  'Вода дистилл.',
  'Воздух',
  '87% спирт',
  'Воздух',
  '96% спирт',
  'Воздух',
  'OG-6 (оранжевый G)',
  'Воздух',
  'EA-50',
  'Воздух',
  'Хлорка'
];
const DEFAULT_VALVE_COORDS = [0, 0, 1371, 2742, 4114, 5485, 6856, 8227, 9599, 10970, 12341, 13712, 15084, 16455, 17826];

function renderValvesTables() {
  if (state.valves1.length === 0) {
    for (let i = 0; i <= 14; i++) {
      state.valves1.push({ selector: 1, hole: i, name: DEFAULT_HOLE_NAMES[i] || '', coord: DEFAULT_VALVE_COORDS[i] || 0, dirty: false });
    }
  }
  if (state.valves2.length === 0) {
    for (let i = 0; i <= 14; i++) {
      state.valves2.push({ selector: 2, hole: i, name: DEFAULT_HOLE_NAMES[i] || '', coord: DEFAULT_VALVE_COORDS[i] || 0, dirty: false });
    }
  }

  renderValveTbody('#valves1-tbody', state.valves1, 1);
  renderValveTbody('#valves2-tbody', state.valves2, 2);
}

function renderValveTbody(selector, data, selNum) {
  const tb = $(selector);
  if (!tb) return;
  tb.innerHTML = data.map((v, i) =>
    '<tr><td class="mono">' + v.hole + '</td><td>' + (v.name || 'Отверстие ' + v.hole) + '</td>' +
    '<td><input class="inp inp-sm' + (v.dirty ? ' dirty' : '') + '" type="number" min="0" max="65535" value="' + v.coord + '" data-sel="' + selNum + '" data-hole="' + i + '"></td></tr>'
  ).join('');

  tb.querySelectorAll('input').forEach(inp => {
    inp.addEventListener('change', () => {
      const h = parseInt(inp.dataset.hole);
      const arr = selNum === 1 ? state.valves1 : state.valves2;
      arr[h].coord = parseInt(inp.value) || 0;
      arr[h].dirty = true;
      inp.classList.add('dirty');
    });
  });
}

async function readAllValves() {
  $('#valves-log').textContent = 'Чтение положений клапанов (30 операций)...';
  try {
    const r = await apiPost('/settings/valves/read-all');
    if (r.ok) {
      state.valves1 = r.data.selector1;
      state.valves2 = r.data.selector2;
      renderValvesTables();
      $('#valves-log').textContent = 'OK — 2 клапана прочитаны (15+15 позиций)';
    } else { $('#valves-log').textContent = 'ERR: ' + r.error; }
  } catch(e) { $('#valves-log').textContent = 'Ошибка сети'; }
}

async function writeAllValves() {
  $('#valves-log').textContent = 'Отправка данных на сервер...';
  for (const v of state.valves1) {
    await apiPut('/settings/valves/1/' + v.hole, { coord: v.coord });
  }
  for (const v of state.valves2) {
    await apiPut('/settings/valves/2/' + v.hole, { coord: v.coord });
  }

  $('#valves-log').textContent = 'Запись положений в контроллер (30 операций)...';
  try {
    const r = await apiPost('/settings/valves/write-all');
    if (r.ok) {
      state.valves1.forEach(v => { v.dirty = false; });
      state.valves2.forEach(v => { v.dirty = false; });
      renderValvesTables();
      $('#valves-log').textContent = 'OK: сохранено ' + r.data.written + ' положений клапанов';
    } else {
      $('#valves-log').textContent = 'ERR: ' + r.error;
    }
  } catch(e) { $('#valves-log').textContent = 'Ошибка сети'; }
}

// ─── PROGRAMS ───────────────────────────────────────────────────────────────
async function runProgram(prog) {
  const urls = {
    'system-check':'/programs/system-check','load':'/programs/load','sedimentation':'/programs/sedimentation',
    'stain-start':'/programs/stain/start','stain-pause':'/programs/stain/pause','stain-resume':'/programs/stain/resume',
    'stain-stop':'/programs/stain/stop','wash-start':'/programs/wash/start','full-start':'/programs/full/start',
  };
  const url = urls[prog]; if (!url) return;
  try {
    const r = await apiPost(url);
    if (r.ok) { if (prog === 'stain-start') $('#progress-wrap').style.display = 'block'; }
    else { alert(r.error); }
  } catch(e) { alert('Ошибка сети'); }
}
function updateProgress(d) {
  // Глобальный прогресс-бар (для всех операций)
  const gp = $('#global-progress');
  const gpf = $('#gp-fill');
  const gpt = $('#gp-text');

  if (d.stopped || !d.running) {
    // Операция завершена — скрыть оба бара
    gp.style.display = 'none';
    const pw = $('#progress-wrap'); if (pw) pw.style.display = 'none';
    return;
  }

  if (d.total_steps > 0) {
    const pct = Math.round((d.current_step / d.total_steps) * 100);
    const label = (d.op ? '[' + d.op + '] ' : '') + (d.step_name || '') + ' ' + d.current_step + '/' + d.total_steps;

    // Глобальный бар
    gp.style.display = 'flex';
    gpf.style.width = pct + '%';
    gpt.textContent = label;

    // Также обновить бар программ (если на вкладке)
    const pw = $('#progress-wrap');
    if (pw) { pw.style.display = 'block'; $('#progress-fill').style.width = pct + '%'; $('#progress-text').textContent = label; }
  }
}
async function emergencyStop() {
  if (!confirm('Аварийный останов?')) return;
  try { await apiPost('/programs/emergency-stop'); $('#progress-wrap').style.display = 'none'; } catch(e) {}
}

// ─── TESTING ────────────────────────────────────────────────────────────────
async function readSensors() {
  try { const r = await apiGet('/testing/sensors'); if (r.ok) renderSensors(r.data); } catch(e) {}
}
function renderSensorPlaceholders() {
  const g = $('#sensor-grid');
  const names = ['Вход 0 (жидкость)','Вход 1 (ротор)','Вход 2 (стекло 1-1)','Вход 3 (стекло 1-2)','Вход 4 (стекло 2-1)','Вход 5 (стекло 2-2)','Вход 6','Вход 7','Вход 8','Вход 9 (селектор 1)','Вход 10 (селектор 2)','Вход 11','Вход 12'];
  g.innerHTML = names.map((n,i) => '<div class="sensor off"><span>' + n + '</span><strong>—</strong></div>').join('');
}
function renderSensors(d) {
  const names = ['Вход 0 (жидкость)','Вход 1 (ротор)','Вход 2 (стекло 1-1)','Вход 3 (стекло 1-2)','Вход 4 (стекло 2-1)','Вход 5 (стекло 2-2)','Вход 6','Вход 7','Вход 8','Вход 9 (селектор 1)','Вход 10 (селектор 2)','Вход 11','Вход 12'];
  $('#sensor-grid').innerHTML = (d.inputs || []).map((v,i) =>
    '<div class="sensor ' + (v ? 'on' : 'off') + '"><span>' + (names[i]||'Вход '+i) + '</span><strong>' + (v ? '1' : '0') + '</strong></div>'
  ).join('');
  if (d.mcu_temp !== undefined) $('#s-mcu').textContent = d.mcu_temp.toFixed(1);
  if (d.ntc_temp !== undefined) $('#s-ntc').textContent = d.ntc_temp.toFixed(1);
  if (d.hx711_weight !== undefined) $('#s-hx711').textContent = d.hx711_weight;
}

// ─── DIAGNOSTICS ────────────────────────────────────────────────────────────
async function clearError() { try { await apiPost('/diagnostics/error/clear'); $('#diag-error').textContent = 'OK'; } catch(e) {} }
async function clearLog() { try { await apiDelete('/diagnostics/log'); $('#log-container').innerHTML = ''; } catch(e) {} }
function addLogEntry(level, msg) {
  const c = $('#log-container'); if (!c) return;
  const div = document.createElement('div');
  div.className = 'log-entry ' + level;
  div.textContent = new Date().toLocaleTimeString() + ' [' + level + '] ' + msg;
  c.prepend(div);
  while (c.children.length > 100) c.removeChild(c.lastChild);
}

// ─── RESET PLC ───────────────────────────────────────────────────────────────
async function resetPLC() {
  if (!confirm('Сбросить PLC? Контроллер будет перезагружен.')) return;
  const st = $('#sel-status');
  if (st) { st.textContent = 'Сброс PLC...'; st.style.color = 'var(--warning)'; }
  try {
    const r = await apiPost('/programs/reset-plc');
    if (r.ok) {
      if (st) { st.textContent = 'PLC сброшен. Ready=' + r.data.ready_status; st.style.color = 'var(--success)'; }
      await refreshStatus();
    } else {
      if (st) { st.textContent = 'Ошибка сброса: ' + r.error; st.style.color = 'var(--danger)'; }
    }
  } catch(e) {
    if (st) { st.textContent = 'Ошибка сети'; st.style.color = 'var(--danger)'; }
  }
}

// ─── SELECTOR ────────────────────────────────────────────────────────────────
function initSelectors() {
  $('#sel-btn-1').onclick = () => switchSelector(1);
  $('#sel-btn-2').onclick = () => switchSelector(2);
  $('#btn-calibrate-sel').onclick = calibrateSelector;
  $('#btn-reset-plc').onclick = resetPLC;
  document.querySelectorAll('.sel-hole-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const hole = parseInt(btn.dataset.hole);
      if (hole) selectHole(hole);
    });
  });
  updateSelectorUI();
}

function switchSelector(num) {
  state.activeSelector = num;
  const calBtn = $('#btn-calibrate-sel');
  if (calBtn) calBtn.textContent = '⟲ Обнулить селектор ' + num;
  updateSelectorUI();
}

function updateSelectorUI() {
  const n = state.activeSelector;
  // Toggle buttons
  const b1 = $('#sel-btn-1'), b2 = $('#sel-btn-2');
  if (b1) { b1.className = n === 1 ? 'btn btn-accent btn-sm sel-toggle active' : 'btn btn-outline btn-sm sel-toggle'; }
  if (b2) { b2.className = n === 2 ? 'btn btn-accent btn-sm sel-toggle active' : 'btn btn-outline btn-sm sel-toggle'; }

  // Label
  const lbl = $('#sel-active-label');
  if (lbl) lbl.textContent = 'Селектор ' + n;

  // Highlight current position
  document.querySelectorAll('.sel-hole-btn').forEach(btn => {
    const hole = parseInt(btn.dataset.hole);
    if (hole === state.selectorPos[n]) {
      btn.classList.add('sel-active');
    } else {
      btn.classList.remove('sel-active');
    }
  });

  // Status text
  const st = $('#sel-status');
  const sensorId = n === 1 ? 9 : 10;
  if (st) st.textContent = 'Текущая позиция: отверстие ' + state.selectorPos[n] + ' | датчик: вход ' + sensorId;
}

async function selectHole(hole) {
  if (!state.connected) {
    alert('Нет подключения к Modbus. Нажмите «Подключить».');
    return;
  }

  const n = state.activeSelector;
  // Disable buttons during operation
  document.querySelectorAll('.sel-hole-btn').forEach(b => b.disabled = true);

  const st = $('#sel-status');
  if (st) { st.textContent = 'Переключение селектора ' + n + ' → отверстие ' + hole + '...'; st.style.color = 'var(--warning)'; }

  try {
    const r = await apiPost('/testing/selector', { num: n, hole: hole });
    if (r.ok) {
      state.selectorPos[n] = hole;
      if (st) { st.textContent = 'OK: селектор ' + n + ' → отверстие ' + hole; st.style.color = 'var(--success)'; }
      updateSelectorUI();
    } else {
      if (st) { st.textContent = 'Ошибка: ' + r.error; st.style.color = 'var(--danger)'; }
    }
  } catch(e) {
    if (st) { st.textContent = 'Ошибка сети'; st.style.color = 'var(--danger)'; }
  }

  document.querySelectorAll('.sel-hole-btn').forEach(b => b.disabled = false);
}

// ─── REAGENT DIALOG ─────────────────────────────────────────────────────────
function showReagentDialog(data) {
  const modal = $('#reagent-modal');
  const msg = $('#reagent-modal-msg');
  if (msg) msg.textContent = data.message || ('Закончился реагент на шаге ' + (data.step || '?') + ': ' + (data.step_name || ''));
  if (modal) modal.style.display = 'flex';
}
function hideReagentDialog() {
  const modal = $('#reagent-modal');
  if (modal) modal.style.display = 'none';
}
async function reagentReplace() {
  hideReagentDialog();
  await apiPost('/programs/stain/reagent-replaced');
}
async function reagentCancel() {
  hideReagentDialog();
  await apiPost('/programs/stain/reagent-cancel');
}

async function calibrateSelector() {
  if (!state.connected) {
    alert('Нет подключения к Modbus. Нажмите «Подключить».');
    return;
  }

  const n = state.activeSelector;
  if (!confirm('Обнулить селектор ' + n + '? Механизм вернётся в исходную позицию.')) return;

  const calBtn = $('#btn-calibrate-sel');
  const st = $('#sel-status');
  if (calBtn) calBtn.disabled = true;
  if (st) { st.textContent = 'Калибровка селектора ' + n + '...'; st.style.color = 'var(--warning)'; }

  try {
    const r = await apiPost('/testing/selector/calibrate', { num: n });
    if (r.ok) {
      state.selectorPos[n] = 1;
      if (st) { st.textContent = 'OK: селектор ' + n + ' обнулён, позиция 1'; st.style.color = 'var(--success)'; }
      updateSelectorUI();
    } else {
      if (st) { st.textContent = 'Ошибка: ' + r.error; st.style.color = 'var(--danger)'; }
    }
  } catch(e) {
    if (st) { st.textContent = 'Ошибка сети'; st.style.color = 'var(--danger)'; }
  }

  if (calBtn) calBtn.disabled = false;
}
