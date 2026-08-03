'use strict';

(() => {
  const O = window.ONE24;
  const FORMAT = 'onepap24-settings';
  const VERSION = 1;
  const MAX_FILE_SIZE = 1024 * 1024;

  function buildProfile() {
    return {
      format: FORMAT,
      version: VERSION,
      exported_at: new Date().toISOString(),
      application: 'ONEPAP.24 Modbus Configurator',
      connection: {
        port: String(O.$('#conn-port-inp').value || '').trim(),
        baudrate: Number(O.$('#conn-baud-inp').value),
        slave_id: Number(O.$('#conn-slave-inp').value)
      },
      steps: O.state.steps.map((step) => ({
        id: Number(step.id),
        name: String(step.name || ''),
        exposure_time: Number(step.exposure_time),
        fill_volume: Number(step.fill_volume)
      })),
      detection: {
        reagent_empty_delta: Number(O.state.detection.reagent_empty_delta)
      },
      selectors: {
        selector_1: O.state.valves1.map((item) => ({ hole: Number(item.hole), name: String(item.name || ''), coord: Number(item.coord) })),
        selector_2: O.state.valves2.map((item) => ({ hole: Number(item.hole), name: String(item.name || ''), coord: Number(item.coord) }))
      }
    };
  }

  function assertInteger(value, min, max, path) {
    if (!Number.isInteger(value) || value < min || value > max) {
      throw new Error(`${path}: ожидается целое число ${min}–${max}`);
    }
  }

  function validateUnique(items, key, min, max, path) {
    const seen = new Set();
    items.forEach((item, index) => {
      const value = Number(item?.[key]);
      assertInteger(value, min, max, `${path}[${index}].${key}`);
      if (seen.has(value)) throw new Error(`${path}: повторяется ${key}=${value}`);
      seen.add(value);
    });
  }

  function validateProfile(profile) {
    if (!profile || typeof profile !== 'object' || Array.isArray(profile)) throw new Error('Корневой элемент должен быть JSON-объектом');
    if (profile.format !== FORMAT) throw new Error(`Неверный format: ожидается "${FORMAT}"`);
    if (profile.version !== VERSION) throw new Error(`Версия ${profile.version} не поддерживается; ожидается ${VERSION}`);

    const connection = profile.connection;
    if (!connection || typeof connection.port !== 'string' || !connection.port.trim()) throw new Error('connection.port не должен быть пустым');
    assertInteger(Number(connection.baudrate), 1200, 4000000, 'connection.baudrate');
    assertInteger(Number(connection.slave_id), 1, 247, 'connection.slave_id');

    if (!Array.isArray(profile.steps) || profile.steps.length !== 11) throw new Error('steps должен содержать ровно 11 шагов');
    validateUnique(profile.steps, 'id', 1, 11, 'steps');
    profile.steps.forEach((step, index) => {
      assertInteger(Number(step.exposure_time), 1, 600, `steps[${index}].exposure_time`);
      assertInteger(Number(step.fill_volume), 1, 6000, `steps[${index}].fill_volume`);
    });

    assertInteger(Number(profile.detection?.reagent_empty_delta), 0, 255, 'detection.reagent_empty_delta');
    validateSelector(profile.selectors?.selector_1, 'selectors.selector_1');
    validateSelector(profile.selectors?.selector_2, 'selectors.selector_2');
    return profile;
  }

  function validateSelector(items, path) {
    if (!Array.isArray(items) || items.length !== 15) throw new Error(`${path} должен содержать ровно 15 позиций`);
    validateUnique(items, 'hole', 0, 14, path);
    items.forEach((item, index) => assertInteger(Number(item.coord), 0, 65535, `${path}[${index}].coord`));
  }

  function countChanges(profile) {
    const currentSteps = new Map(O.state.steps.map((item) => [Number(item.id), item]));
    const current1 = new Map(O.state.valves1.map((item) => [Number(item.hole), item]));
    const current2 = new Map(O.state.valves2.map((item) => [Number(item.hole), item]));
    const steps = profile.steps.filter((item) => {
      const current = currentSteps.get(Number(item.id));
      return !current || current.exposure_time !== Number(item.exposure_time) || current.fill_volume !== Number(item.fill_volume);
    }).length;
    const selector1 = profile.selectors.selector_1.filter((item) => current1.get(Number(item.hole))?.coord !== Number(item.coord)).length;
    const selector2 = profile.selectors.selector_2.filter((item) => current2.get(Number(item.hole))?.coord !== Number(item.coord)).length;
    const detection = Number(profile.detection.reagent_empty_delta) !== Number(O.state.detection.reagent_empty_delta);
    const connection = String(profile.connection.port).trim() !== String(O.$('#conn-port-inp').value).trim() ||
      Number(profile.connection.baudrate) !== Number(O.$('#conn-baud-inp').value) ||
      Number(profile.connection.slave_id) !== Number(O.$('#conn-slave-inp').value);
    return { steps, valves: selector1 + selector2, detection, connection };
  }

  function setInput(input, value) {
    input.value = String(value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  }

  function applyProfile(profile) {
    O.$('#conn-port-inp').value = String(profile.connection.port).trim();
    O.$('#conn-baud-inp').value = String(profile.connection.baudrate);
    O.$('#conn-slave-inp').value = String(profile.connection.slave_id);

    const steps = new Map(profile.steps.map((item) => [Number(item.id), item]));
    O.$$('#steps-tbody tr').forEach((row) => {
      const id = Number(row.cells[0]?.textContent);
      const item = steps.get(id);
      const inputs = row.querySelectorAll('input');
      if (item && inputs.length >= 2) {
        setInput(inputs[0], item.exposure_time);
        setInput(inputs[1], item.fill_volume);
      }
    });
    setInput(O.$('#det-delta'), profile.detection.reagent_empty_delta);

    applySelectorRows('#valves1-tbody', profile.selectors.selector_1);
    applySelectorRows('#valves2-tbody', profile.selectors.selector_2);
    O.text('#settings-source', `Загружен JSON-профиль ${new Date().toLocaleTimeString()}. Значения ещё не записаны в контроллер.`);
  }

  function applySelectorRows(selector, positions) {
    const byHole = new Map(positions.map((item) => [Number(item.hole), item]));
    O.$$(`${selector} tr`).forEach((row) => {
      const hole = Number(row.cells[0]?.textContent);
      const item = byHole.get(hole);
      const input = row.querySelector('input');
      if (item && input) setInput(input, item.coord);
    });
  }

  function downloadProfile() {
    try {
      const json = JSON.stringify(buildProfile(), null, 2) + '\n';
      const blob = new Blob([json], { type: 'application/json;charset=utf-8' });
      const link = document.createElement('a');
      const stamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\..+/, '').replace('T', '-');
      link.href = URL.createObjectURL(blob);
      link.download = `onepap24-settings-${stamp}.json`;
      document.body.append(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(link.href);
      O.toast('JSON-профиль экспортирован.', 'ok');
      O.message('#profile-log', 'Экспортирован текущий профиль интерфейса.', 'ok');
    } catch (error) {
      O.toast(`Экспорт не выполнен: ${error.message}`, 'error');
    }
  }

  async function importFile(file) {
    if (!file) return;
    if (file.size > MAX_FILE_SIZE) throw new Error('Файл больше 1 МБ');
    const text = await file.text();
    let profile;
    try { profile = JSON.parse(text); }
    catch (error) { throw new Error(`Ошибка синтаксиса JSON: ${error.message}`); }
    validateProfile(profile);
    const changes = countChanges(profile);
    const total = changes.steps + changes.valves + Number(changes.detection) + Number(changes.connection);
    const description = total
      ? `Шаги: ${changes.steps}; координаты: ${changes.valves}; порог: ${changes.detection ? 'изменится' : 'без изменений'}; подключение: ${changes.connection ? 'изменится' : 'без изменений'}.`
      : 'Файл совпадает с текущими значениями.';
    if (!await O.confirm('Импортировать JSON-профиль?', `${description} Импорт только подготовит изменения и ничего не запишет в контроллер.`, 'Импортировать')) return;
    applyProfile(profile);
    O.message('#profile-log', total ? `Профиль импортирован. Подготовлено изменений: ${total}.` : 'Профиль импортирован, отличий нет.', total ? 'ok' : '');
    O.toast(total ? 'Профиль загружен. Проверьте и запишите изменения.' : 'Профиль совпадает с текущими настройками.', 'ok');
  }

  function mount() {
    const actions = O.$('#tab-settings .page-actions');
    if (!actions || O.$('#btn-profile-export')) return;

    const exportButton = document.createElement('button');
    exportButton.id = 'btn-profile-export';
    exportButton.className = 'btn btn-secondary';
    exportButton.type = 'button';
    exportButton.textContent = 'Экспорт JSON';
    exportButton.onclick = downloadProfile;

    const importButton = document.createElement('button');
    importButton.id = 'btn-profile-import';
    importButton.className = 'btn btn-secondary';
    importButton.type = 'button';
    importButton.textContent = 'Импорт JSON';

    const input = document.createElement('input');
    input.id = 'profile-file-input';
    input.type = 'file';
    input.accept = 'application/json,.json';
    input.hidden = true;
    importButton.onclick = () => input.click();
    input.onchange = async () => {
      try { await importFile(input.files?.[0]); }
      catch (error) { O.message('#profile-log', error.message, 'error'); O.toast(`Импорт не выполнен: ${error.message}`, 'error'); }
      finally { input.value = ''; }
    };

    const log = document.createElement('span');
    log.id = 'profile-log';
    log.className = 'operation-message';
    log.setAttribute('role', 'status');

    actions.prepend(importButton, exportButton, input);
    const panel = O.$('#tab-settings .work-panel');
    panel?.insertAdjacentElement('beforebegin', log);
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mount, { once: true });
  else mount();
})();
