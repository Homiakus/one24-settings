'use strict';

(() => {
  const O = window.ONE24 = {
    API: '/api/v1',
    state: {
      connected: false, busy: false, ready: null, error: null,
      activeTab: 'settings', confirmResolve: null, wsAttempt: 0, toastTimer: null
    },
    $: (s) => document.querySelector(s),
    $$: (s) => [...document.querySelectorAll(s)]
  };
  let ws;
  void import('./app-profile.js');

  O.text = (selector, value) => { const element = O.$(selector); if (element) element.textContent = value; };
  O.request = async (path, { method = 'GET', body, timeout = 300000 } = {}) => {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeout);
    try {
      const response = await fetch(O.API + path, {
        method,
        signal: controller.signal,
        headers: { Accept: 'application/json', ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}) },
        body: body === undefined ? undefined : JSON.stringify(body)
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok || !payload?.ok) throw new Error(payload?.error || `HTTP ${response.status}`);
      return payload.data;
    } catch (error) {
      if (error.name === 'AbortError') throw new Error('Превышено время ожидания');
      throw error;
    } finally { clearTimeout(timer); }
  };

  O.announce = (message) => { O.text('#live-region', ''); setTimeout(() => O.text('#live-region', message), 20); };
  O.toast = (message, kind = '') => {
    const toast = O.$('#toast');
    clearTimeout(O.state.toastTimer);
    toast.textContent = message;
    toast.className = `toast${kind ? ` is-${kind}` : ''}`;
    toast.hidden = false;
    O.state.toastTimer = setTimeout(() => { toast.hidden = true; }, kind === 'error' ? 7000 : 4000);
  };
  O.message = (selector, message, kind = '') => {
    const element = O.$(selector); if (!element) return;
    element.textContent = message;
    element.className = `operation-message${kind ? ` is-${kind}` : ''}`;
  };
  O.label = (selector, message, kind = '') => {
    const element = O.$(selector); if (!element) return;
    element.textContent = message;
    element.className = `state-label${kind ? ` is-${kind}` : ''}`;
  };
  O.setBusy = (value, message = '') => {
    O.state.busy = value;
    O.updateAvailability();
    if (message) O.announce(message);
  };
  O.updateAvailability = () => {
    O.$$('.requires-connection').forEach((element) => { element.disabled = !O.state.connected || O.state.busy; });
    O.$('#btn-connect').disabled = O.state.connected || O.state.busy;
    O.$('#btn-disconnect').disabled = !O.state.connected || O.state.busy;
    if (O.updateSettingsAvailability) O.updateSettingsAvailability();
    if (O.updateOperationsAvailability) O.updateOperationsAvailability();
  };

  function selectTab(tab, writeHash = true) {
    O.state.activeTab = tab;
    O.$$('.sb-item').forEach((button) => {
      const active = button.dataset.tab === tab;
      button.classList.toggle('active', active);
      active ? button.setAttribute('aria-current', 'page') : button.removeAttribute('aria-current');
    });
    O.$$('.tab-panel').forEach((panel) => {
      const active = panel.id === `tab-${tab}`;
      panel.hidden = !active;
      if (active) O.text('#tb-page', panel.dataset.title || tab);
    });
    if (writeHash) history.replaceState(null, '', `#${tab}`);
  }
  function initNavigation() {
    O.$$('.sb-item').forEach((button) => button.addEventListener('click', () => selectTab(button.dataset.tab)));
    const hash = location.hash.slice(1);
    selectTab(O.$(`#tab-${hash}`) ? hash : 'settings', false);
  }

  function setDot(kind) {
    ['#device-dot', '#sb-dot'].forEach((selector) => { O.$(selector).className = `status-dot is-${kind}`; });
  }
  O.syncZoneUI = (zone) => {
    if (!zone) return;
    O.state.zone = zone;
    const zStr = String(zone);
    ['#zone-select', '#zone-select-prog'].forEach((sel) => {
      const el = O.$(sel);
      if (el && el.value !== zStr) el.value = zStr;
    });
    O.text('#kpi-zone', zStr);
  };

  O.renderConnection = (errorName = '') => {
    const port = O.$('#conn-port-inp').value.trim() || '—';
    O.text('#sb-status-text', O.state.connected ? 'Подключено' : 'Нет подключения');
    O.text('#sb-device-text', O.state.connected ? `${port} · slave ${O.$('#conn-slave-inp').value}` : 'Контроллер не выбран');
    O.text('#kpi-zone', O.state.zone ? String(O.state.zone) : '3');
    O.text('#kpi-ready', O.state.ready ?? '—');
    O.text('#kpi-error', O.state.error === 0 ? 'Нет' : O.state.error ?? '—');
    O.text('#device-status-text', O.state.connected
      ? `${port}, ${O.$('#conn-baud-inp').value} бод, slave ${O.$('#conn-slave-inp').value}`
      : 'Укажите последовательный порт и подключитесь.');
    const attention = O.$('#system-attention');
    if (!O.state.connected) {
      setDot('offline'); attention.className = 'attention-strip is-warning';
      O.text('#attention-title', 'Контроллер не подключён'); O.text('#attention-text', 'Чтение, запись и запуск программ недоступны.');
    } else if (Number(O.state.error) > 0) {
      setDot('error'); attention.className = 'attention-strip is-error';
      O.text('#attention-title', `Ошибка контроллера ${O.state.error}`); O.text('#attention-text', errorName || 'Откройте диагностику и устраните причину.');
    } else if (Number(O.state.ready) === 1) {
      setDot('busy'); attention.className = 'attention-strip is-neutral';
      O.text('#attention-title', 'Контроллер занят'); O.text('#attention-text', 'Дождитесь завершения текущей операции.');
    } else {
      setDot('online'); attention.className = 'attention-strip is-ok';
      O.text('#attention-title', 'Контроллер готов'); O.text('#attention-text', 'Можно читать настройки или запускать программу.');
    }
    O.updateAvailability();
  };
  O.refreshStatus = async () => {
    try {
      const data = await O.request('/status');
      O.state.connected = Boolean(data.connected);
      O.state.ready = data.ready_status ?? null;
      O.state.error = data.status_error ?? null;
      if (data.zone) O.syncZoneUI(data.zone);
      O.renderConnection(data.error_name || '');
    } catch (error) {
      O.state.connected = false; O.renderConnection(); O.text('#conn-error', error.message);
    }
  };
  async function loadPorts() {
    try {
      const data = await O.request('/ports');
      const list = O.$('#ports-list'); list.replaceChildren();
      (data.ports || []).forEach((port) => { const option = document.createElement('option'); option.value = port; list.append(option); });
    } catch (_) { /* Ручной ввод остаётся доступным. */ }
  }
  async function connect() {
    const port = O.$('#conn-port-inp').value.trim();
    const baudrate = Number(O.$('#conn-baud-inp').value);
    const slave = Number(O.$('#conn-slave-inp').value);
    if (!port || !Number.isInteger(baudrate) || baudrate <= 0 || !Number.isInteger(slave) || slave < 1 || slave > 247) {
      O.text('#conn-error', 'Проверьте порт, скорость и Slave ID.'); return;
    }
    O.text('#conn-error', ''); O.setBusy(true, 'Подключение к контроллеру'); setDot('busy');
    try {
      await O.request('/connect', { method: 'POST', body: { port, baudrate, slave_id: slave } });
      O.state.connected = true; await O.refreshStatus(); O.toast(`Контроллер подключён: ${port}`, 'ok');
    } catch (error) {
      O.state.connected = false; O.text('#conn-error', error.message); O.toast(`Не удалось подключиться: ${error.message}`, 'error'); O.renderConnection();
    } finally { O.setBusy(false); }
  }
  async function disconnect() {
    O.setBusy(true, 'Отключение контроллера');
    try { await O.request('/disconnect', { method: 'POST' }); } catch (error) { O.toast(error.message, 'error'); }
    O.state.connected = false; O.state.ready = null; O.state.error = null; O.setBusy(false); O.renderConnection();
  }
  function initConnection() {
    O.$('#connection-form').addEventListener('submit', (event) => { event.preventDefault(); connect(); });
    O.$('#btn-disconnect').addEventListener('click', disconnect);
    O.$('#attention-action').addEventListener('click', () => { O.$('#conn-port-inp').focus(); scrollTo({ top: 0, behavior: 'smooth' }); });
    loadPorts();
  }

  O.confirm = (title, message, confirmText = 'Подтвердить', danger = false) => {
    if (O.state.confirmResolve) resolveConfirm(false);
    O.text('#confirm-title', title); O.text('#confirm-message', message);
    const button = O.$('#confirm-accept'); button.textContent = confirmText; button.className = danger ? 'btn btn-danger' : 'btn btn-primary';
    O.$('#confirm-modal').hidden = false; setTimeout(() => button.focus(), 0);
    return new Promise((resolve) => { O.state.confirmResolve = resolve; });
  };
  function resolveConfirm(result) {
    if (!O.state.confirmResolve) return;
    const resolve = O.state.confirmResolve; O.state.confirmResolve = null; O.$('#confirm-modal').hidden = true; resolve(result);
  }
  function initModals() {
    O.$('#confirm-cancel').onclick = () => resolveConfirm(false);
    O.$('#confirm-accept').onclick = () => resolveConfirm(true);
    O.$('#confirm-modal').onclick = (event) => { if (event.target === O.$('#confirm-modal')) resolveConfirm(false); };
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape' && !O.$('#confirm-modal').hidden) resolveConfirm(false);
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
        event.preventDefault();
        if (O.state.activeTab === 'settings' && O.writeSettings) O.writeSettings();
        if (O.state.activeTab === 'valves' && O.writeValves) O.writeValves();
      }
    });
  }

  O.updateProgress = (data) => {
    if (data.stopped || data.running === false) { O.hideProgress(); O.refreshStatus(); return; }
    const total = Number(data.total_steps || 0), current = Number(data.current_step || 0);
    if (total <= 0) return;
    const percent = Math.max(0, Math.min(100, Math.round(current / total * 100)));
    const percentStr = `${percent}%`;
    const chipStr = `Шаг ${current}/${total}`;
    const caption = data.step_name || data.op || 'Операция';
    const titles = {
      read: 'Чтение настроек', write: 'Запись настроек в EEPROM',
      read_valves: 'Чтение координат клапанов', write_valves: 'Запись координат клапанов',
      system_check: 'Проверка системы', stain: 'Окраска Папаниколау', wash: 'Промывка системы'
    };

    const gProgress = O.$('#global-progress');
    if (gProgress) {
      gProgress.hidden = false;
      O.text('#gp-title', titles[data.op] || 'Операция выполняется');
      O.text('#gp-text', caption);
      O.text('#gp-chip', chipStr);
      O.text('#gp-percent', percentStr);
      const gpFill = O.$('#gp-fill');
      if (gpFill) gpFill.style.width = `${percent}%`;
    }

    const pWrap = O.$('#progress-wrap');
    if (pWrap) {
      pWrap.hidden = false;
      O.text('#prog-card-title', titles[data.op] || 'Выполнение цикла');
      O.text('#progress-text', caption);
      O.text('#prog-chip', chipStr);
      O.text('#prog-percent', percentStr);
      const pFill = O.$('#progress-fill');
      if (pFill) pFill.style.width = `${percent}%`;
    }
  };
  O.hideProgress = () => {
    ['#global-progress', '#progress-wrap'].forEach((sel) => {
      const el = O.$(sel);
      if (el) el.hidden = true;
    });
    ['#gp-fill', '#progress-fill'].forEach((sel) => {
      const el = O.$(sel);
      if (el) el.style.width = '0%';
    });
  };
  function connectWebSocket() {
    if (ws && [WebSocket.OPEN, WebSocket.CONNECTING].includes(ws.readyState)) return;
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${location.host}/ws/events?topics=status,progress,sensors,reagent,reagent_low,error,log`);
    O.text('#ws-status', 'Телеметрия: подключение…');
    ws.onopen = () => { O.state.wsAttempt = 0; O.text('#ws-status', 'Телеметрия: активна'); };
    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data), data = message.data || {};
        if (message.type === 'connect') { O.state.connected = true; O.refreshStatus(); }
        else if (message.type === 'disconnect') { O.state.connected = false; O.renderConnection(); }
        else if (message.type === 'status') { O.state.connected = data.connected ?? O.state.connected; O.state.ready = data.ready_status ?? O.state.ready; O.state.error = data.status_error ?? O.state.error; O.renderConnection(data.error_name || ''); }
        else if (message.type === 'progress') O.updateProgress(data);
        else if (O.handleEvent) O.handleEvent(message.type, data);
      } catch (_) { if (O.addLog) O.addLog('WARN', 'Некорректное событие телеметрии'); }
    };
    ws.onclose = () => {
      O.text('#ws-status', 'Телеметрия: переподключение');
      setTimeout(connectWebSocket, Math.min(30000, 1000 * (2 ** O.state.wsAttempt++)));
    };
    ws.onerror = () => ws.close();
  }

  document.addEventListener('DOMContentLoaded', async () => {
    initNavigation(); initConnection(); initModals(); connectWebSocket();
    if (O.initSettings) await O.initSettings();
    if (O.initOperations) await O.initOperations();
    await O.refreshStatus();
    setInterval(() => { if (!O.state.busy) O.refreshStatus(); }, 5000);
    O.updateAvailability();
  });
})();
