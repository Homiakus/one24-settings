'use strict';

(() => {
  const O = window.ONE24;
  const SCHEMA = 'onepap24-settings';
  const VERSION = 1;
  const MAX_FILE_SIZE = 1024 * 1024;
  let initialized = false;

  const isObject = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);
  const ownKeys = (value) => Object.keys(value || {});

  function assertAllowedKeys(value, allowed, path) {
    const unknown = ownKeys(value).filter((key) => !allowed.includes(key));
    if (unknown.length) throw new Error(`${path}: неизвестные поля: ${unknown.join(', ')}`);
  }

  function integer(value, min, max, path) {
    if (!Number.isInteger(value) || value < min || value > max) {
      throw new Error(`${path}: требуется целое число ${min}–${max}`);
    }
    return value;
  }

  function text(value, path, maxLength = 120) {
    if (typeof value !== 'string' || !value.trim() || value.length > maxLength) {
      throw new Error(`${path}: требуется непустая строка длиной до ${maxLength} символов`);
    }
    return value.trim();
  }

  function normalizeSteps(items) {
    if (!Array.isArray(items) || items.length !== 11) {
      throw new Error('staining.steps: требуется ровно 11 шагов');
    }
    const seen = new Set();
    const result = items.map((item, index) => {
      if (!isObject(item)) throw new Error(`staining.steps[${index}]: требуется объект`);
      assertAllowedKeys(item, ['id', 'name', 'exposure_time', 'fill_volume'], `staining.steps[${index}]`);
      const id = integer(item.id, 1, 11, `staining.steps[${index}].id`);
      if (seen.has(id)) throw new Error(`staining.steps: повторяется id ${id}`);
      seen.add(id);
      return {
        id,
        name: text(item.name, `staining.steps[${index}].name`),
        exposure_time: integer(item.exposure_time, 1, 600, `staining.steps[${index}].exposure_time`),
        fill_volume: integer(item.fill_volume, 1, 6000, `staining.steps[${index}].fill_volume`)
      };
    });
    return result.sort((a, b) => a.id - b.id);
  }

  function normalizeSelector(items, selector) {
    if (!Array.isArray(items) || items.length !== 15) {
      throw new Error(`selectors.selector${selector}: требуется ровно 15 позиций`);
    }
    const seen = new Set();
    const result = items.map((item, index) => {
      const path = `selectors.selector${selector}[${index}]`;
      if (!isObject(item)) throw new Error(`${path}: требуется объект`);
      assertAllowedKeys(item, ['selector', 'hole', 'name', 'coord'], path);
      if (integer(item.selector, 1, 2, `${path}.selector`) !== selector) {
        throw new Error(`${path}.selector: ожидалось значение ${selector}`);
      }
      const hole = integer(item.hole, 0, 14, `${path}.hole`);
      if (seen.has(hole)) throw new Error(`selectors.selector${selector}: повторяется позиция ${hole}`);
      seen.add(hole);
      return {
        selector,
        hole,
        name: text(item.name, `${path}.name`),
        coord: integer(item.coord, 0, 65535, `${path}.coord`)
      };
    });
    return result.sort((a, b) => a.hole - b.hole);
  }

  function normalizeProfile(raw) {
    if (!isObject(raw)) throw new Error('Корень JSON должен быть объектом');
    assertAllowedKeys(raw, ['schema', 'schema_version', 'exported_at', 'profile_name', 'description', 'application', 'device', 'staining', 'selectors'], 'profile');
    if (raw.schema !== SCHEMA) throw new Error(`schema: ожидалось "${SCHEMA}"`);
    if (raw.schema_version !== VERSION) throw new Error(`schema_version: поддерживается только версия ${VERSION}`);

    if (raw.exported_at !== undefined && (typeof raw.exported_at !== 'string' || Number.isNaN(Date.parse(raw.exported_at)))) {
      throw new Error('exported_at: некорректная дата');
    }
    if (raw.application !== undefined) {
      if (!isObject(raw.application)) throw new Error('application: требуется объект');
      assertAllowedKeys(raw.application, ['name', 'version'], 'application');
      if (raw.application.name !== undefined) text(raw.application.name, 'application.name', 120);
      if (raw.application.version !== undefined) text(raw.application.version, 'application.version', 40);
    }

    if (!isObject(raw.device)) throw new Error('device: требуется объект');
    assertAllowedKeys(raw.device, ['port', 'baudrate', 'slave_id'], 'device');
    const device = {
      port: text(raw.device.port, 'device.port', 128),
      baudrate: integer(raw.device.baudrate, 1200, 4000000, 'device.baudrate'),
      slave_id: integer(raw.device.slave_id, 1, 247, 'device.slave_id')
    };

    if (!isObject(raw.staining)) throw new Error('staining: требуется объект');
    assertAllowedKeys(raw.staining, ['protocol', 'reagent_empty_delta', 'steps'], 'staining');
    if (raw.staining.protocol !== 'pap_stain') throw new Error('staining.protocol: поддерживается только "pap_stain"');
    const staining = {
      protocol: 'pap_stain',
      reagent_empty_delta: integer(raw.staining.reagent_empty_delta, 0, 255, 'staining.reagent_empty_delta'),
      steps: normalizeSteps(raw.staining.steps)
    };

    if (!isObject(raw.selectors)) throw new Error('selectors: требуется объект');
    assertAllowedKeys(raw.selectors, ['selector1', 'selector2'], 'selectors');
    const selectors = {
      selector1: normalizeSelector(raw.selectors.selector1, 1),
      selector2: normalizeSelector(raw.selectors.selector2, 2)
    };

    return {
      schema: SCHEMA,
      schema_version: VERSION,
      exported_at: raw.exported_at || new Date().toISOString(),
      profile_name: raw.profile_name ? String(raw.profile_name) : undefined,
      description: raw.description ? String(raw.description) : undefined,
      application: { name: 'ONEPAP.24 Modbus Configurator', version: '1' },
      device,
      staining,
      selectors
    };
  }

  const SEL1_COORDS = [0, 700, 1400, 2771, 4142, 5513, 6884, 8256, 9627, 10998, 12369, 13741, 15112, 16483, 17853];
  const SEL2_COORDS = [0, 600, 1200, 2571, 3942, 5313, 6684, 8056, 9427, 10798, 12169, 13541, 14912, 16283, 17653];
  const HOLE_NAMES = ["Исходное положение (Home)","Воздух","EA-50","Воздух","Вода дистиллированная","Воздух","Спирт 87%","Воздух","Спирт 96%","Воздух","OG-6 (оранжевый G)","Воздух","Гематоксилин Харриса","Воздух","Хлорка"];

  const buildSelectorList = (sel, coords) => coords.map((coord, hole) => ({
    selector: sel,
    hole,
    name: HOLE_NAMES[hole] || (hole % 2 === 1 ? 'Воздух' : `Позиция ${hole}`),
    coord
  }));

  const STAIN_NAMES = [
    "Спирт 96% (фиксация)", "Гематоксилин Харриса", "Вода дистиллированная", "Вода дистиллированная",
    "Спирт 87%", "OG-6", "Спирт 96%", "EA-50", "Спирт 87%", "Спирт 87%", "Спирт 96%"
  ];
  const FULL_EXP = [10, 120, 10, 50, 10, 10, 10, 120, 10, 10, 10];
  const FILL_VOLS = [50, 30, 80, 80, 60, 40, 60, 40, 60, 60, 60];

  const buildStepsList = (expTime) => STAIN_NAMES.map((name, i) => ({
    id: i + 1,
    name,
    exposure_time: typeof expTime === 'number' ? expTime : FULL_EXP[i],
    fill_volume: FILL_VOLS[i]
  }));

  const PRESETS = {
    full: normalizeProfile({
      schema: SCHEMA,
      schema_version: VERSION,
      profile_name: 'Полноценный рабочий профиль',
      device: { port: 'COM4', baudrate: 115200, slave_id: 1 },
      staining: { protocol: 'pap_stain', reagent_empty_delta: 10, steps: buildStepsList(null) },
      selectors: { selector1: buildSelectorList(1, SEL1_COORDS), selector2: buildSelectorList(2, SEL2_COORDS) }
    }),
    test: normalizeProfile({
      schema: SCHEMA,
      schema_version: VERSION,
      profile_name: 'Тестовый профиль (2 сек)',
      device: { port: 'COM4', baudrate: 115200, slave_id: 1 },
      staining: { protocol: 'pap_stain', reagent_empty_delta: 10, steps: buildStepsList(2) },
      selectors: { selector1: buildSelectorList(1, SEL1_COORDS), selector2: buildSelectorList(2, SEL2_COORDS) }
    })
  };

  function buildProfile() {
    return normalizeProfile({
      schema: SCHEMA,
      schema_version: VERSION,
      exported_at: new Date().toISOString(),
      application: { name: 'ONEPAP.24 Modbus Configurator', version: '1' },
      device: {
        port: O.$('#conn-port-inp').value.trim() || 'COM4',
        baudrate: Number(O.$('#conn-baud-inp').value),
        slave_id: Number(O.$('#conn-slave-inp').value)
      },
      staining: {
        protocol: 'pap_stain',
        reagent_empty_delta: Number(O.state.detection.reagent_empty_delta),
        steps: O.state.steps.map((step) => ({
          id: Number(step.id),
          name: String(step.name),
          exposure_time: Number(step.exposure_time),
          fill_volume: Number(step.fill_volume)
        }))
      },
      selectors: {
        selector1: O.state.valves1.map((item) => ({ selector: 1, hole: Number(item.hole), name: String(item.name), coord: Number(item.coord) })),
        selector2: O.state.valves2.map((item) => ({ selector: 2, hole: Number(item.hole), name: String(item.name), coord: Number(item.coord) }))
      }
    });
  }

  function countChanges(profile) {
    const currentSteps = new Map(O.state.steps.map((item) => [Number(item.id), item]));
    const currentValves = new Map([...O.state.valves1, ...O.state.valves2].map((item) => [`${item.selector}:${item.hole}`, item]));
    const steps = profile.staining.steps.filter((item) => {
      const current = currentSteps.get(item.id);
      return !current || Number(current.exposure_time) !== item.exposure_time || Number(current.fill_volume) !== item.fill_volume;
    }).length;
    const valves = [...profile.selectors.selector1, ...profile.selectors.selector2].filter((item) => {
      const current = currentValves.get(`${item.selector}:${item.hole}`);
      return !current || Number(current.coord) !== item.coord;
    }).length;
    const detection = Number(O.state.detection.reagent_empty_delta) !== profile.staining.reagent_empty_delta;
    const device = O.$('#conn-port-inp').value.trim() !== profile.device.port
      || Number(O.$('#conn-baud-inp').value) !== profile.device.baudrate
      || Number(O.$('#conn-slave-inp').value) !== profile.device.slave_id;
    return { steps, valves, detection, device };
  }

  function dispatchNumberInput(selector, rowIndex, inputIndex, value) {
    const row = O.$$(selector)[rowIndex];
    if (!row) throw new Error(`Не найдена строка ${rowIndex + 1} в интерфейсе`);
    const input = row.querySelectorAll('input')[inputIndex];
    if (!input) throw new Error(`Не найдено поле ${inputIndex + 1} в строке ${rowIndex + 1}`);
    if (Number(input.value) === value) return;
    input.value = String(value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  }

  function applyProfile(profile) {
    profile.staining.steps.forEach((step, index) => {
      dispatchNumberInput('#steps-tbody tr', index, 0, step.exposure_time);
      dispatchNumberInput('#steps-tbody tr', index, 1, step.fill_volume);
    });

    const delta = O.$('#det-delta');
    if (Number(delta.value) !== profile.staining.reagent_empty_delta) {
      delta.value = String(profile.staining.reagent_empty_delta);
      delta.dispatchEvent(new Event('input', { bubbles: true }));
    }

    profile.selectors.selector1.forEach((item, index) => dispatchNumberInput('#valves1-tbody tr', index, 0, item.coord));
    profile.selectors.selector2.forEach((item, index) => dispatchNumberInput('#valves2-tbody tr', index, 0, item.coord));

    if (!O.state.connected) {
      O.$('#conn-port-inp').value = profile.device.port;
      O.$('#conn-baud-inp').value = String(profile.device.baudrate);
      O.$('#conn-slave-inp').value = String(profile.device.slave_id);
      O.renderConnection();
    }

    const importedAt = new Date().toLocaleTimeString();
    const name = profile.profile_name || 'JSON-профиль';
    O.text('#settings-source', `Применён профиль "${name}" (${importedAt}). Перед записью проверьте изменённые строки.`);
    O.text('#data-freshness', `Профиль: ${importedAt}`);
  }

  function downloadProfile(profile) {
    const data = `${JSON.stringify(profile, null, 2)}\n`;
    const blob = new Blob([data], { type: 'application/json;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const timestamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d{3}Z$/, 'Z');
    const link = document.createElement('a');
    link.href = url;
    link.download = `onepap24-settings-${timestamp}.json`;
    document.body.append(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }

  async function exportProfile() {
    try {
      const profile = buildProfile();
      downloadProfile(profile);
      O.message('#profile-log', 'JSON-профиль экспортирован. В файл вошли текущие значения интерфейса.', 'ok');
      O.toast('Настройки экспортированы в JSON.', 'ok');
    } catch (error) {
      O.message('#profile-log', error.message, 'error');
      O.toast(`Экспорт не выполнен: ${error.message}`, 'error');
    }
  }

  async function importProfile(file) {
    if (!file) return;
    if (file.size > MAX_FILE_SIZE) throw new Error('Файл больше 1 МБ');
    const raw = JSON.parse(await file.text());
    const profile = normalizeProfile(raw);
    const changes = countChanges(profile);
    const total = changes.steps + changes.valves + (changes.detection ? 1 : 0);
    const deviceNote = changes.device
      ? (O.state.connected ? ' Параметры подключения не будут менять активное соединение.' : ' Параметры подключения будут подставлены в форму.')
      : '';
    const accepted = await O.confirm(
      'Импортировать JSON-профиль?',
      `Файл корректен: 11 шагов, 30 координат. Изменится параметров: ${total}.${deviceNote} Данные не будут записаны в PLC автоматически.`,
      'Импортировать'
    );
    if (!accepted) return;
    applyProfile(profile);
    O.message('#profile-log', `Профиль импортирован: шагов ${changes.steps}, координат ${changes.valves}${changes.detection ? ', порог детекции изменён' : ''}.`, 'ok');
    O.toast('Профиль импортирован. Проверьте изменения и запишите их в контроллер.', 'ok');
  }

  function createUI() {
    if (initialized) return;
    initialized = true;
    const actions = O.$('#tab-settings .page-actions');
    const intro = O.$('#tab-settings .page-intro');
    if (!actions || !intro) return;

    const presetSelect = document.createElement('select');
    presetSelect.id = 'preset-profile-select';
    presetSelect.className = 'inp';
    presetSelect.ariaLabel = 'Выбор профиля настроек';
    presetSelect.innerHTML = `
      <option value="full">Полноценный рабочий профиль (Папаниколау)</option>
      <option value="test">Тестовый профиль (2 сек у всех экспозиций)</option>
    `;

    const applyButton = document.createElement('button');
    applyButton.id = 'btn-apply-profile';
    applyButton.type = 'button';
    applyButton.className = 'btn btn-primary';
    applyButton.textContent = 'Применить профиль';

    const importButton = document.createElement('button');
    importButton.id = 'btn-import-profile';
    importButton.type = 'button';
    importButton.className = 'btn btn-secondary';
    importButton.textContent = 'Импорт JSON';

    const exportButton = document.createElement('button');
    exportButton.id = 'btn-export-profile';
    exportButton.type = 'button';
    exportButton.className = 'btn btn-secondary';
    exportButton.textContent = 'Экспорт JSON';

    const input = document.createElement('input');
    input.id = 'profile-file';
    input.type = 'file';
    input.accept = 'application/json,.json';
    input.hidden = true;

    const status = document.createElement('p');
    status.id = 'profile-log';
    status.className = 'operation-message standalone-message';
    status.setAttribute('role', 'status');

    actions.prepend(exportButton);
    actions.prepend(importButton);
    actions.prepend(applyButton);
    actions.prepend(presetSelect);
    intro.after(status);
    document.body.append(input);

    applyButton.onclick = async () => {
      const key = presetSelect.value;
      const profile = PRESETS[key] || PRESETS.full;
      const name = key === 'test' ? 'Тестовый профиль (2 сек)' : 'Полноценный рабочий профиль';
      const accepted = await O.confirm(
        `Применить профиль "${name}"?`,
        `Профиль полностью обновит экспозиции всех 11 шагов (${key === 'test' ? '2 секунды' : 'штатные времена'}) и все 30 координат для первого и второго селекторов.`,
        'Применить полностью'
      );
      if (!accepted) return;
      applyProfile(profile);
      O.message('#profile-log', `Профиль "${name}" применён полностью к настройкам экспозиции и селекторам.`, 'ok');
      O.toast(`Профиль "${name}" полностью применён.`, 'ok');
    };

    importButton.onclick = () => input.click();
    exportButton.onclick = exportProfile;
    input.onchange = async () => {
      const file = input.files?.[0];
      input.value = '';
      try {
        await importProfile(file);
      } catch (error) {
        const message = error instanceof SyntaxError ? 'Файл содержит некорректный JSON' : error.message;
        O.message('#profile-log', message, 'error');
        O.toast(`Импорт не выполнен: ${message}`, 'error');
      }
    };
  }

  async function boot() {
    for (let attempt = 0; attempt < 100; attempt += 1) {
      if (Array.isArray(O.state.steps) && O.state.steps.length === 11
        && Array.isArray(O.state.valves1) && O.state.valves1.length === 15
        && Array.isArray(O.state.valves2) && O.state.valves2.length === 15) {
        createUI();
        return;
      }
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    O.toast('Модуль JSON-профилей не инициализирован.', 'error');
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot, { once: true });
  } else {
    boot();
  }
})();
