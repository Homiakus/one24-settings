'use strict';

(() => {
  const O = window.ONE24;
  const STEP_DEFAULTS = [
    ['Экспозиция образца', 0, 0],
    ['Спирт 96% (фиксация)', 10, 50],
    ['Гематоксилин Харриса', 120, 30],
    ['Вода дистиллированная', 10, 80],
    ['Вода дистиллированная', 50, 80],
    ['Спирт 87%', 10, 60],
    ['OG-6 (оранжевый G)', 10, 40],
    ['Спирт 96%', 10, 60],
    ['EA-50', 120, 40],
    ['Спирт 87%', 10, 60],
    ['Спирт 87%', 10, 60],
    ['Спирт 96%', 10, 60],
    ['Хлорка', 20, 50],
    ['Хлорка', 20, 50],
    ['Спирт', 10, 60],
    ['Вода', 10, 80]
  ];
  // NB: эталонные значения в configurator/modbus/registers.go
  const HOLES = ['Исходное положение (Home)','Воздух','EA-50','Воздух','Вода дистиллированная','Воздух','Спирт 87%','Воздух','Спирт 96%','Воздух','OG-6 (оранжевый G)','Воздух','Гематоксилин Харриса','Воздух','Хлорка'];
  const COORDS1 = [0,700,1400,2771,4142,5513,6884,8256,9627,10998,12369,13741,15112,16483,17853];
  const COORDS2 = [0,600,1200,2571,3942,5313,6684,8056,9427,10798,12169,13541,14912,16283,17653];

  Object.assign(O.state, {
    steps: [], valves1: [], valves2: [], stepsFromDevice: false, valvesFromDevice: false,
    detection: { reagent_empty: false, reagent_empty_delta: 10, original: 10, dirty: false }
  });

  const defaultSteps = () => STEP_DEFAULTS.map(([name, t, v], i) => ({
    id: i, name, exposure_time: t, fill_volume: v, originalT: t, originalV: v, dirty: false, error: ''
  }));

  const defaultValves = (selector) => HOLES.map((name, hole) => {
    const coords = selector === 1 ? COORDS1 : COORDS2;
    return { selector, hole, name, coord: coords[hole], original: coords[hole], dirty: false, error: '' };
  });

  const normalizeSteps = (items) => {
    let rawMap = new Map();
    if (Array.isArray(items)) {
      items.forEach(item => { if (item && item.id !== undefined) rawMap.set(Number(item.id), item); });
    }
    const list = [];
    for (let i = 0; i <= 15; i++) {
      const def = STEP_DEFAULTS[i];
      const item = rawMap.get(i) || {};
      const t = Number(item.exposure_time ?? def[1]);
      const v = Number(item.fill_volume ?? def[2]);
      const name = String(item.name || def[0]);
      list.push({ id: i, name, exposure_time: t, fill_volume: v, originalT: t, originalV: v, dirty: false, error: '' });
    }
    return list;
  };

  const normalizeValves = (items, selector) => (items || []).map((item, i) => {
    const coords = selector === 1 ? COORDS1 : COORDS2;
    const coord = Number(item.coord ?? coords[i]);
    return { selector, hole: Number(item.hole ?? i), name: String(item.name || HOLES[i]), coord, original: coord, dirty: false, error: '' };
  });

  const settingsCount = () => O.state.steps.filter((item) => item.dirty).length + (O.state.detection.dirty ? 1 : 0);
  const valvesCount = () => [...O.state.valves1, ...O.state.valves2].filter((item) => item.dirty).length;

  function updateCounters() {
    const sCount = settingsCount(), vCount = valvesCount(), total = sCount + vCount;
    O.text('#settings-dirty-count', total ? `Изменено параметров: ${total}` : 'Изменений нет');
    O.$('#settings-dirty-count').classList.toggle('has-changes', total > 0);
    O.updateAvailability();
  }

  O.updateSettingsAvailability = () => {
    const total = settingsCount() + valvesCount();
    O.$('#btn-write-all').disabled = !O.state.connected || O.state.busy || total === 0;
  };

  function numberInput(value, min, max, label) {
    const input = document.createElement('input');
    input.type = 'number';
    input.className = 'inp';
    input.value = String(value);
    input.min = String(min);
    input.max = String(max);
    input.inputMode = 'numeric';
    input.setAttribute('aria-label', label);
    return input;
  }

  function cell(child) {
    const td = document.createElement('td');
    td.append(child);
    return td;
  }

  function showSkeleton(id, rows){const body=O.$(id);if(!body)return;body.replaceChildren();for(let i=0;i<rows;i++){const row=document.createElement('div');row.className='skeleton-row';for(let j=0;j<5;j++){const span=document.createElement('span');row.append(span);}body.append(row);}}

  function renderSteps() {
    const body = O.$('#steps-tbody');
    body.replaceChildren();
    O.state.steps.forEach((step, index) => {
      const row = document.createElement('tr');
      row.classList.toggle('is-dirty', step.dirty);
      row.classList.toggle('is-error', Boolean(step.error));

      const number = document.createElement('td');
      number.className = 'mono';
      number.textContent = String(step.id);

      const name = document.createElement('td');
      name.textContent = step.name;

      const minVal = step.id === 0 ? 0 : 1;
      const time = numberInput(step.exposure_time, minVal, 600, `Шаг ${step.id}: экспозиция`);
      const volume = numberInput(step.fill_volume, minVal, 6000, `Шаг ${step.id}: налив`);

      time.classList.toggle('dirty', step.exposure_time !== step.originalT);
      volume.classList.toggle('dirty', step.fill_volume !== step.originalV);

      time.addEventListener('input', () => changeStep(index, 'exposure_time', time));
      volume.addEventListener('input', () => changeStep(index, 'fill_volume', volume));

      const status = document.createElement('td');
      status.className = `row-state${step.dirty ? ' is-dirty' : step.error ? ' is-error' : ' is-ok'}`;
      status.textContent = step.error || (step.dirty ? 'Изменено' : O.state.stepsFromDevice ? 'Прочитано' : 'По умолчанию');

      row.append(number, name, cell(time), cell(volume), status);
      body.append(row);
    });
  }

  function changeStep(index, key, input) {
    const step = O.state.steps[index];
    const value = Number(input.value);
    const minVal = step.id === 0 ? 0 : 1;
    const max = key === 'exposure_time' ? 600 : 6000;
    step[key] = value;
    step.error = Number.isInteger(value) && value >= minVal && value <= max ? '' : `Допустимо ${minVal}–${max}`;
    step.dirty = step.exposure_time !== step.originalT || step.fill_volume !== step.originalV;

    input.classList.toggle('dirty', key === 'exposure_time' ? step.exposure_time !== step.originalT : step.fill_volume !== step.originalV);
    const row = input.closest('tr');
    if (row) {
      row.classList.toggle('is-dirty', step.dirty);
      row.classList.toggle('is-error', Boolean(step.error));
      const status = row.querySelector('.row-state');
      if (status) {
        status.className = `row-state${step.dirty ? ' is-dirty' : step.error ? ' is-error' : ' is-ok'}`;
        status.textContent = step.error || (step.dirty ? 'Изменено' : O.state.stepsFromDevice ? 'Прочитано' : 'По умолчанию');
      }
    }
    updateCounters();
  }

  function changeDelta() {
    const value = Number(O.$('#det-delta').value);
    O.state.detection.reagent_empty_delta = value;
    O.state.detection.dirty = value !== O.state.detection.original;
    O.$('#det-delta').classList.toggle('dirty', O.state.detection.dirty);
    updateCounters();
  }

  async function readSettings() {
    O.setBusy(true, 'Чтение всех настроек');
    O.message('#settings-log', 'Чтение параметров и координат…');
    try {
      const [data, vData] = await Promise.all([
        O.request('/settings/read-all', { method: 'POST', timeout: 120000 }),
        O.request('/settings/valves/read-all', { method: 'POST', timeout: 180000 }).catch(() => null)
      ]);
      O.state.steps = normalizeSteps(data.steps);
      const delta = Number(data.detection?.reagent_empty_delta ?? 10);
      O.state.detection = { reagent_empty: Boolean(data.detection?.reagent_empty), reagent_empty_delta: delta, original: delta, dirty: false };
      O.state.stepsFromDevice = true;
      O.$('#det-delta').value = String(delta);
      O.$('#det-delta').classList.remove('dirty');

      if (vData) {
        O.state.valves1 = normalizeValves(vData.selector1, 1);
        O.state.valves2 = normalizeValves(vData.selector2, 2);
        O.state.valvesFromDevice = true;
      }

      O.label('#settings-state', 'Прочитано', 'ok');
      O.label('#det-status', O.state.detection.reagent_empty ? 'Реагент пуст' : 'Реагент в норме', O.state.detection.reagent_empty ? 'error' : 'ok');
      const timeStr = new Date().toLocaleTimeString();
      O.text('#settings-source', `Данные прочитаны ${timeStr}.`);
      O.text('#data-freshness', `Настройки и клапаны: ${timeStr}`);
      O.message('#settings-log', 'Все параметры и координаты прочитаны.', 'ok');
      renderSteps();
      renderValves();
      updateCounters();
    } catch (error) {
      O.message('#settings-log', error.message, 'error');
      O.toast(`Чтение не выполнено: ${error.message}`, 'error');
    } finally {
      O.setBusy(false);
    }
  }

  O.writeSettings = async () => {
    const sChanged = O.state.steps.filter((item) => item.dirty);
    const vChanged = [...O.state.valves1, ...O.state.valves2].filter((item) => item.dirty);
    const sInvalid = sChanged.find((item) => item.error);
    const vInvalid = vChanged.find((item) => item.error);

    if (sInvalid) { O.toast(`Исправьте шаг ${sInvalid.id}: ${sInvalid.error}`, 'error'); return; }
    if (vInvalid) { O.toast(`Исправьте селектор ${vInvalid.selector}, позиция ${vInvalid.hole}.`, 'error'); return; }

    const totalCount = sChanged.length + (O.state.detection.dirty ? 1 : 0) + vChanged.length;
    if (!totalCount) return;

    if (!await O.confirm('Записать изменения?', `В контроллер будут записаны изменённые параметры: ${totalCount} (шагов: ${sChanged.length}, клапанов: ${vChanged.length}).`, 'Записать')) return;

    O.setBusy(true, 'Запись настроек');
    O.message('#settings-log', 'Передача изменений…');
    try {
      for (const step of sChanged) {
        await O.request(`/settings/steps/${step.id}`, { method: 'PUT', body: { exposure_time: step.exposure_time, fill_volume: step.fill_volume } });
      }
      if (O.state.detection.dirty) {
        await O.request('/settings/detection', { method: 'PUT', body: { delta: O.state.detection.reagent_empty_delta } });
      }
      if (sChanged.length > 0 || O.state.detection.dirty) {
        await O.request('/settings/write-all', { method: 'POST', timeout: 120000 });
      }

      for (const item of vChanged) {
        await O.request(`/settings/valves/${item.selector}/${item.hole}`, { method: 'PUT', body: { coord: item.coord } });
      }
      if (vChanged.length > 0) {
        await O.request('/settings/valves/write-all', { method: 'POST', timeout: 180000 });
      }

      sChanged.forEach((step) => { step.originalT = step.exposure_time; step.originalV = step.fill_volume; step.dirty = false; });
      vChanged.forEach((item) => { item.original = item.coord; item.dirty = false; });
      O.state.detection.original = O.state.detection.reagent_empty_delta;
      O.state.detection.dirty = false;
      O.$('#det-delta').classList.remove('dirty');

      O.message('#settings-log', `Записано параметров: ${totalCount}.`, 'ok');
      O.toast('Все изменения записаны и подтверждены контроллером.', 'ok');
      renderSteps();
      renderValves();
      updateCounters();
    } catch (error) {
      O.message('#settings-log', `Ошибка записи: ${error.message}`, 'error');
      O.toast(`Запись не завершена: ${error.message}`, 'error');
    } finally {
      O.setBusy(false);
    }
  };

  function renderValves() {
    renderValveBody('#valves1-tbody', O.state.valves1);
    renderValveBody('#valves2-tbody', O.state.valves2);
  }

  function renderValveBody(selector, items) {
    const body = O.$(selector);
    if (!body) return;
    body.replaceChildren();
    items.forEach((item) => {
      const row = document.createElement('tr');
      row.classList.toggle('is-dirty', item.dirty);
      row.classList.toggle('is-error', Boolean(item.error));

      const number = document.createElement('td');
      number.className = 'mono';
      number.textContent = String(item.hole);

      const name = document.createElement('td');
      name.textContent = item.name;

      const input = numberInput(item.coord, 0, 65535, `Селектор ${item.selector}, позиция ${item.hole}`);
      input.classList.toggle('dirty', item.dirty);
      input.addEventListener('input', () => {
        const value = Number(input.value);
        item.coord = value;
        item.error = Number.isInteger(value) && value >= 0 && value <= 65535 ? '' : 'Допустимо 0–65535';
        item.dirty = value !== item.original;
        input.classList.toggle('dirty', item.dirty);
        const row = input.closest('tr');
        if (row) {
          row.classList.toggle('is-dirty', item.dirty);
          row.classList.toggle('is-error', Boolean(item.error));
          const status = row.querySelector('.row-state');
          if (status) {
            status.className = `row-state${item.dirty ? ' is-dirty' : item.error ? ' is-error' : ' is-ok'}`;
            status.textContent = item.error || (item.dirty ? 'Изменено' : O.state.valvesFromDevice ? 'Прочитано' : 'По умолчанию');
          }
        }
        updateCounters();
      });

      const status = document.createElement('td');
      status.className = `row-state${item.dirty ? ' is-dirty' : item.error ? ' is-error' : ' is-ok'}`;
      status.textContent = item.error || (item.dirty ? 'Изменено' : O.state.valvesFromDevice ? 'Прочитано' : 'По умолчанию');

      row.append(number, name, cell(input), status);
      body.append(row);
    });
  }

  async function readValves() {
    return readSettings();
  }

  O.writeValves = async () => {
    return O.writeSettings();
  };

  O.initSettings = async () => {
    showSkeleton('#steps-tbody', 16);
    showSkeleton('#valves1-tbody', 15);
    showSkeleton('#valves2-tbody', 15);
    const [settings, valves] = await Promise.all([
      O.request('/settings/steps').catch(() => null),
      O.request('/settings/valves').catch(() => null)
    ]);
    O.state.steps = settings?.steps ? normalizeSteps(settings.steps) : defaultSteps();
    if (settings?.detection) {
      const delta = Number(settings.detection.reagent_empty_delta ?? 10);
      O.state.detection = { reagent_empty: Boolean(settings.detection.reagent_empty), reagent_empty_delta: delta, original: delta, dirty: false };
      O.$('#det-delta').value = String(delta);
    }
    O.state.valves1 = valves?.selector1 ? normalizeValves(valves.selector1, 1) : defaultValves(1);
    O.state.valves2 = valves?.selector2 ? normalizeValves(valves.selector2, 2) : defaultValves(2);

    O.sendZone = async (zoneVal) => {
      const zone = Number(zoneVal);
      O.setBusy(true, 'Отправка зоны в контроллер');
      try {
        await O.request('/settings/zone', { method: 'PUT', body: { zone } });
        O.syncZoneUI(zone);
        O.toast(`Зона ${zone} успешно отправлена в контроллер.`, 'ok');
      } catch (error) {
        O.toast(`Ошибка отправки зоны: ${error.message}`, 'error');
        try {
          const current = await O.request('/settings/zone');
          if (current?.zone) O.syncZoneUI(current.zone);
        } catch (_) {}
      } finally {
        O.setBusy(false);
      }
    };

    // Инициализация селектора зоны
    try {
      const zoneData = await O.request('/settings/zone').catch(() => ({ zone: 3 }));
      O.syncZoneUI(zoneData.zone ?? 3);
    } catch (_) { /* по умолчанию 3 */ }

    const onZoneSelectChange = (event) => {
      O.syncZoneUI(Number(event.target.value));
    };

    if (O.$('#zone-select')) O.$('#zone-select').onchange = onZoneSelectChange;
    if (O.$('#zone-select-prog')) O.$('#zone-select-prog').onchange = onZoneSelectChange;

    const onSendZoneClick = () => {
      const val = O.$('#zone-select')?.value || O.$('#zone-select-prog')?.value || 3;
      O.sendZone(val);
    };

    if (O.$('#btn-send-zone')) O.$('#btn-send-zone').onclick = onSendZoneClick;
    if (O.$('#btn-send-zone-prog')) O.$('#btn-send-zone-prog').onclick = onSendZoneClick;

    renderSteps();
    renderValves();
    updateCounters();

    O.$('#btn-read-all').onclick = readSettings;
    O.$('#btn-write-all').onclick = O.writeSettings;
    O.$('#det-delta').oninput = changeDelta;
  };
})();
