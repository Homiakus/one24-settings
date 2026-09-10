'use strict';

/**
 * AutoTrace Interactive Flow & Button Editor for Modbus ONEPAP.24
 * Inspired by AutoTrace Lab (https://github.com/Homiakus/autotraceLab.git)
 */

window.AutoTraceEditor = (() => {
  function getO() {
    return window.ONE24 || {
      state: {},
      toast: (msg) => console.log('[Toast]', msg),
      confirm: async () => true,
      setBusy: () => {},
      request: async () => ({})
    };
  }

  // ─── Constants & Metadata ───────────────────────────────────────────────────

  const AUTOMATION_TYPES = {
    automatic: { id: 'automatic', label: 'АВТО', color: '#10B981', bg: 'rgba(16, 185, 129, 0.12)', line: 'rgba(16, 185, 129, 0.4)' },
    manual:    { id: 'manual',    label: 'РУЧНО', color: '#F59E0B', bg: 'rgba(245, 158, 11, 0.12)', line: 'rgba(245, 158, 11, 0.4)' },
    mixed:     { id: 'mixed',     label: 'СМЕШАННО', color: '#8B5CF6', bg: 'rgba(139, 92, 246, 0.12)', line: 'rgba(139, 92, 246, 0.4)' },
    wait:      { id: 'wait',      label: 'ОЖИДАНИЕ', color: '#3B82F6', bg: 'rgba(59, 130, 246, 0.12)', line: 'rgba(59, 130, 246, 0.4)' },
    qc:        { id: 'qc',        label: 'QC / ТЕСТ', color: '#EF4444', bg: 'rgba(239, 68, 68, 0.12)', line: 'rgba(239, 68, 68, 0.4)' }
  };

  const MODBUS_COMMANDS = [
    { cmd: 120, name: '120 · Калибровка клапана', category: 'automatic', defaultSec: 15 },
    { cmd: 100, name: '100 · Калибровка селектора 1', category: 'automatic', defaultSec: 20 },
    { cmd: 105, name: '105 · Калибровка селектора 2', category: 'automatic', defaultSec: 20 },
    { cmd: 200, name: '200 · Хомирование ротора', category: 'automatic', defaultSec: 12 },
    { cmd: 130, name: '130 · Слив промежуточной ёмкости', category: 'automatic', defaultSec: 10 },
    { cmd: 140, name: '140 · Загрузка материала', category: 'manual', defaultSec: 30 },
    { cmd: 160, name: '160 · Осаждение клеток (300с)', category: 'wait', defaultSec: 300 },
    { cmd: 150, name: '150 · Цикл окраски (11 шагов)', category: 'mixed', defaultSec: 660 },
    { cmd: 170, name: '170 · Промывка системы (4 шага)', category: 'automatic', defaultSec: 180 },
    { cmd: 222, name: '222 · Сохранить настройки в EEPROM', category: 'qc', defaultSec: 5 },
    { cmd: 999, name: '999 · Аварийный сброс / стоп', category: 'qc', defaultSec: 2 }
  ];

  const BLOCK_TYPES = [
    { id: 'command', label: 'Команда Modbus' },
    { id: 'condition', label: 'Условие / ветвление' },
    { id: 'variant', label: 'Вариант алгоритма' },
    { id: 'subgraph', label: 'Подграф' },
    { id: 'checkpoint', label: 'Контрольная точка' }
  ];

  function normalizeStep(step, index) {
    return {
      id: step.id || `step_${index + 1}`,
      name: step.name || `Шаг ${index + 1}`,
      cmd: Number(step.cmd || 120),
      zone: Number(step.zone ?? 3),
      delay_sec: Math.max(0, Number(step.delay_sec || 0)),
      timeout_sec: Math.max(1, Number(step.timeout_sec || 300)),
      retry_count: Math.max(0, Number(step.retry_count || 0)),
      block_type: step.block_type || 'command',
      variant: step.variant || 'default',
      condition: step.condition || '',
      automation: step.automation || 'automatic',
      formula: step.formula || ''
    };
  }

  const PRESET_SEQUENCES = {
    sys_check: {
      name: 'Проверка системы',
      description: 'Калибровка клапанов, ротора и начальный слив',
      steps: [
        { id: 'step_1', name: 'Калибровка клапана', cmd: 120, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'automatic', formula: '15' },
        { id: 'step_2', name: 'Калибровка селектора 1', cmd: 100, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'automatic', formula: 'step_1.time + 5' },
        { id: 'step_3', name: 'Хомирование ротора', cmd: 200, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'automatic', formula: '12' },
        { id: 'step_4', name: 'Промежуточный слив', cmd: 130, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'automatic', formula: '10' }
      ]
    },
    stain_cycle: {
      name: 'Полный цикл Папаниколау',
      description: 'Подготовка, загрузка, экспозиция, 11 этапов окраски и промывка',
      steps: [
        { id: 'step_1', name: 'Калибровка системы', cmd: 120, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'automatic', formula: '15' },
        { id: 'step_2', name: 'Загрузка образцов', cmd: 140, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'manual', formula: '45' },
        { id: 'step_3', name: 'Осаждение клеток', cmd: 160, zone: 3, delay_sec: 2, timeout_sec: 600, automation: 'wait', formula: '300' },
        { id: 'step_4', name: 'Цикл окраски (11 шагов)', cmd: 150, zone: 3, delay_sec: 0, timeout_sec: 900, automation: 'mixed', formula: '660' },
        { id: 'step_5', name: 'Промывка системы', cmd: 170, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'automatic', formula: '180' }
      ]
    },
    wash_system: {
      name: 'Глубокая дезинфекция и промывка',
      description: 'Двукратная обработка хлоркой, спиртом и финишный слив',
      steps: [
        { id: 'step_1', name: 'Промывка: Хлорка (цикл 1)', cmd: 170, zone: 3, delay_sec: 5, timeout_sec: 300, automation: 'automatic', formula: '90' },
        { id: 'step_2', name: 'Промывка: Спирт (цикл 2)', cmd: 170, zone: 3, delay_sec: 5, timeout_sec: 300, automation: 'automatic', formula: '90' },
        { id: 'step_3', name: 'Промывка: Вода очищенная', cmd: 170, zone: 3, delay_sec: 2, timeout_sec: 300, automation: 'automatic', formula: '60' },
        { id: 'step_4', name: 'Финишный слив', cmd: 130, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'automatic', formula: '10' }
      ]
    },
    calib_full: {
      name: 'Полная калибровка узлов',
      description: 'Калибровка всех приводов: клапана, обоих селекторов и ротора',
      steps: [
        { id: 'step_1', name: 'Калибровка клапана', cmd: 120, zone: 3, delay_sec: 1, timeout_sec: 300, automation: 'automatic', formula: '15' },
        { id: 'step_2', name: 'Калибровка селектора 1', cmd: 100, zone: 3, delay_sec: 1, timeout_sec: 300, automation: 'automatic', formula: '20' },
        { id: 'step_3', name: 'Калибровка селектора 2', cmd: 105, zone: 3, delay_sec: 1, timeout_sec: 300, automation: 'automatic', formula: '20' },
        { id: 'step_4', name: 'Хомирование ротора', cmd: 200, zone: 3, delay_sec: 0, timeout_sec: 300, automation: 'automatic', formula: '15' }
      ]
    }
  };

  const DEFAULT_BUTTON_MATRIX = [
    { id: 'btn_sys_check', title: 'Проверка системы', subtitle: 'Калибровка + слив', cmd: 120, zone: 3, color: 'blue', confirm: true, prompt: 'Запустить проверку системы?' },
    { id: 'btn_load_mat', title: 'Загрузка образцов', subtitle: 'Подача в ротор', cmd: 140, zone: 3, color: 'emerald', confirm: true, prompt: 'Запустить загрузку образцов?' },
    { id: 'btn_sediment', title: 'Осаждение (300с)', subtitle: 'Цикл осаждения', cmd: 160, zone: 3, color: 'purple', confirm: true, prompt: 'Запустить цикл осаждения?' },
    { id: 'btn_stain_start', title: 'Цикл окраски', subtitle: 'Папаниколау 11 шагов', cmd: 150, zone: 3, color: 'indigo', confirm: true, prompt: 'Запустить цикл окраски?' },
    { id: 'btn_wash_start', title: 'Промывка системы', subtitle: 'Дезинфекция 4 шага', cmd: 170, zone: 3, color: 'cyan', confirm: true, prompt: 'Запустить промывку системы?' },
    { id: 'btn_drain_inter', title: 'Слив ёмкости', subtitle: 'Промежуточный слив', cmd: 130, zone: 3, color: 'amber', confirm: false },
    { id: 'btn_rotor_home', title: 'Хомирование ротора', subtitle: 'Позиция 0', cmd: 200, zone: 3, color: 'slate', confirm: false },
    { id: 'btn_calib_sel1', title: 'Калибровка Сел. 1', subtitle: 'Обнуление селектора 1', cmd: 100, zone: 3, color: 'teal', confirm: true, prompt: 'Обнулить селектор 1?' },
    { id: 'btn_calib_sel2', title: 'Калибровка Сел. 2', subtitle: 'Обнуление селектора 2', cmd: 105, zone: 3, color: 'teal', confirm: true, prompt: 'Обнулить селектор 2?' },
    { id: 'btn_save_eeprom', title: 'Сохранить EEPROM', subtitle: 'Регистр 1 = 222', cmd: 222, zone: 3, color: 'violet', confirm: true, prompt: 'Записать текущие параметры в энергонезависимую память EEPROM?' },
    { id: 'btn_emerg_stop', title: 'АВАРИЙНЫЙ СТОП', subtitle: 'Немедленный сброс', cmd: 999, zone: 3, color: 'danger', confirm: true, prompt: 'ВНИМАНИЕ! Немедленно остановить все исполнительные механизмы?' }
  ];

  // ─── State ──────────────────────────────────────────────────────────────────

  const state = {
    activeView: 'canvas',
    sequence: [],
    customButtons: [],
    selectedStepId: null,
    activeExecutingStepIdx: null,
    zoom: 1.0,
    panX: 40,
    panY: 30,
    isPanning: false,
    dragStart: { x: 0, y: 0 },
    nodePositions: {},
    formulaResults: {},
    criticalPathIds: new Set()
    ,routedEdges: {}, subgraphs: [], parameters: [], connections: [], portSelection: null
  };

  function routedPath(edge, fallback) {
    const points = state.routedEdges[edge.id];
    if (!Array.isArray(points) || points.length < 2) return fallback;
    return `M ${points.map(point => `${Number(point.x) || 0} ${Number(point.y) || 0}`).join(' L ')}`;
  }

  function loadState() {
    try {
      const seqRaw = localStorage.getItem('onepap_autotrace_seq');
      if (seqRaw) {
        state.sequence = JSON.parse(seqRaw).map(normalizeStep);
      } else {
        state.sequence = PRESET_SEQUENCES.sys_check.steps.map(normalizeStep);
      }

      const btnRaw = localStorage.getItem('onepap_autotrace_buttons');
      if (btnRaw) {
        state.customButtons = JSON.parse(btnRaw);
      } else {
        state.customButtons = DEFAULT_BUTTON_MATRIX.map(b => ({ ...b }));
      }

      const posRaw = localStorage.getItem('onepap_autotrace_positions');
      if (posRaw) {
        state.nodePositions = JSON.parse(posRaw);
      }

      const viewRaw = localStorage.getItem('onepap_autotrace_view');
      if (viewRaw) {
        state.activeView = viewRaw;
      }
      const subgraphsRaw = localStorage.getItem('onepap_autotrace_subgraphs');
      if (subgraphsRaw) state.subgraphs = JSON.parse(subgraphsRaw);
      const paramsRaw = localStorage.getItem('onepap_autotrace_parameters');
      if (paramsRaw) state.parameters = JSON.parse(paramsRaw);
      const connectionsRaw = localStorage.getItem('onepap_autotrace_connections');
      if (connectionsRaw) state.connections = JSON.parse(connectionsRaw);
    } catch (e) {
      console.warn('[AutoTrace] State load fallback:', e);
      state.sequence = PRESET_SEQUENCES.sys_check.steps.map(normalizeStep);
      state.customButtons = DEFAULT_BUTTON_MATRIX.map(b => ({ ...b }));
    }
  }

  function saveState() {
    try {
      localStorage.setItem('onepap_autotrace_seq', JSON.stringify(state.sequence));
      localStorage.setItem('onepap_autotrace_buttons', JSON.stringify(state.customButtons));
      localStorage.setItem('onepap_autotrace_positions', JSON.stringify(state.nodePositions));
      localStorage.setItem('onepap_autotrace_view', state.activeView);
      localStorage.setItem('onepap_autotrace_subgraphs', JSON.stringify(state.subgraphs));
      localStorage.setItem('onepap_autotrace_parameters', JSON.stringify(state.parameters));
      localStorage.setItem('onepap_autotrace_connections', JSON.stringify(state.connections));
      const O = getO();
      if (O.state) {
        O.state.customSequence = state.sequence;
        localStorage.setItem('onepap_custom_seq', JSON.stringify(state.sequence));
      }
    } catch (e) {
      console.warn('[AutoTrace] State save error:', e);
    }
  }

  function evaluateFormulas() {
    const times = {};
    state.criticalPathIds.clear();

    state.sequence.forEach((step, idx) => {
      const key = step.id || `step_${idx + 1}`;
      let calculatedTime = 15;

      if (step.formula && step.formula.trim()) {
        try {
          let expr = step.formula.trim();
          Object.keys(times).forEach(prevKey => {
            const regex = new RegExp(`\\b${prevKey}\\.time\\b`, 'g');
            expr = expr.replace(regex, String(times[prevKey]));
          });
          if (idx > 0) {
            const prevId = state.sequence[idx - 1].id || `step_${idx}`;
            expr = expr.replace(/\bprev\.time\b/g, String(times[prevId] || 15));
          }
          if (/^[0-9+\-*/().\s]+$/.test(expr)) {
            // eslint-disable-next-line no-new-func
            const res = Function(`"use strict"; return (${expr})`)();
            if (typeof res === 'number' && !isNaN(res) && isFinite(res)) {
              calculatedTime = Math.max(1, Math.round(res));
            }
          }
        } catch (e) {
          // fallback
        }
      } else {
        const cmdMeta = MODBUS_COMMANDS.find(c => Number(c.cmd) === Number(step.cmd));
        calculatedTime = (cmdMeta ? cmdMeta.defaultSec : 15) + (Number(step.delay_sec) || 0);
      }

      times[key] = calculatedTime;
    });

    state.formulaResults = times;

    let totalSec = 0;
    let autoSec = 0;
    let manualSec = 0;
    let waitSec = 0;
    let maxStepSec = 0;
    let bottleneckStep = null;

    state.sequence.forEach(step => {
      const sId = step.id;
      const t = times[sId] || 15;
      totalSec += t;
      state.criticalPathIds.add(sId);

      const autoKind = step.automation || (MODBUS_COMMANDS.find(c => Number(c.cmd) === Number(step.cmd))?.category || 'automatic');
      if (autoKind === 'automatic') autoSec += t;
      else if (autoKind === 'manual') manualSec += t;
      else if (autoKind === 'wait') waitSec += t;
      else autoSec += Math.round(t * 0.7);

      if (t > maxStepSec) {
        maxStepSec = t;
        bottleneckStep = step;
      }
    });

    return {
      totalSec,
      totalMin: (totalSec / 60).toFixed(1),
      autoRatio: totalSec > 0 ? Math.round((autoSec / totalSec) * 100) : 100,
      autoSec,
      manualSec,
      waitSec,
      bottleneck: bottleneckStep ? `${bottleneckStep.name} (${maxStepSec}с)` : '—',
      stepCount: state.sequence.length
    };
  }

  function autoLayoutNodes() {
    const startX = 60;
    const startY = 80;
    const stepWidth = 320;
    const stepHeight = 220;
    const maxCols = 3;

    state.sequence.forEach((step, idx) => {
      const col = idx % maxCols;
      const row = Math.floor(idx / maxCols);
      const x = startX + col * stepWidth;
      const y = startY + row * stepHeight;
      state.nodePositions[step.id] = { x, y };
    });

    saveState();
    renderFlowCanvas();
  }

  function graphConnections() {
    if (state.connections.length > 0) return state.connections;
    return state.sequence.slice(0, -1).map((step, index) => ({
      id: `edge-${step.id}-${state.sequence[index + 1].id}`,
      from: step.id,
      to: state.sequence[index + 1].id,
      condition: state.sequence[index + 1].condition || '',
      priority: index
    }));
  }

  function pruneGraphConnections() {
    const nodeIDs = new Set(state.sequence.map(step => step.id));
    state.connections = state.connections.filter(edge => nodeIDs.has(edge.from) && nodeIDs.has(edge.to) && edge.from !== edge.to);
  }

  function handlePortClick(event, nodeId, direction) {
    event.stopPropagation();
    if (direction === 'out') {
      state.portSelection = { nodeId, direction };
      getO().toast('Выход выбран. Теперь нажмите вход другого блока.', '');
      renderFlowCanvas();
      return;
    }
    if (!state.portSelection || state.portSelection.direction !== 'out') {
      getO().toast('Сначала выберите выходной порт блока.', 'error');
      return;
    }
    const from = state.portSelection.nodeId;
    if (from === nodeId) {
      getO().toast('Нельзя соединить блок с самим собой.', 'error');
      return;
    }
    const id = `edge-${from}-${nodeId}`;
    if (!state.connections.some(edge => edge.id === id)) {
      state.connections.push({ id, from, to: nodeId, condition: '', priority: state.connections.length });
      state.routedEdges = {};
      saveState();
      getO().toast('Связь добавлена. Для ветвления повторите с другим входом.', 'ok');
    }
    state.portSelection = null;
    renderFlowCanvas();
  }

  function removeSelectedConnection() {
    if (!state.portSelection?.edgeId) return;
    state.connections = state.connections.filter(edge => edge.id !== state.portSelection.edgeId);
    state.portSelection = null;
    state.routedEdges = {};
    saveState();
    renderFlowCanvas();
  }

  function switchView(viewName) {
    state.activeView = viewName;
    saveState();

    document.querySelectorAll('.at-view-tab').forEach(t => t.classList.toggle('active', t.dataset.view === viewName));

    const canvasView = document.getElementById('at-view-canvas');
    const buttonsView = document.getElementById('at-view-buttons');
    const cardsView = document.getElementById('at-view-cards');
    const mathView = document.getElementById('at-view-math');

    if (canvasView) canvasView.hidden = viewName !== 'canvas';
    if (buttonsView) buttonsView.hidden = viewName !== 'buttons';
    if (cardsView) cardsView.hidden = viewName !== 'cards';
    if (mathView) mathView.hidden = viewName !== 'math';

    updateStatsBar();

    if (viewName === 'canvas') {
      renderFlowCanvas();
    } else if (viewName === 'buttons') {
      renderButtonMatrix();
    } else if (viewName === 'cards') {
      renderCardsWorkbench();
    } else if (viewName === 'math') {
      renderMathWorkbench();
    }
  }

  function updateStatsBar() {
    const stats = evaluateFormulas();
    const elSteps = document.getElementById('at-stat-steps');
    const elTime = document.getElementById('at-stat-time');
    const elAuto = document.getElementById('at-stat-auto');
    const elBottle = document.getElementById('at-stat-bottleneck');

    if (elSteps) elSteps.textContent = `Шагов: ${stats.stepCount}`;
    if (elTime) elTime.textContent = `Общее время: ~${stats.totalMin} мин (${stats.totalSec} с)`;
    if (elAuto) elAuto.textContent = `Автоматизация: ${stats.autoRatio}%`;
    if (elBottle) elBottle.textContent = `Узкое место: ${stats.bottleneck}`;

    const O = getO();
    const runBtn = document.getElementById('btn-seq-run');
    if (runBtn) {
      runBtn.disabled = !O.state?.connected || O.state?.busy || state.sequence.length === 0;
    }
  }

  function renderFlowCanvas() {
    const container = document.getElementById('at-canvas-container');
    const svgLayer = document.getElementById('at-svg-layer');
    const nodesLayer = document.getElementById('at-nodes-layer');
    if (!container || !svgLayer || !nodesLayer) return;

    evaluateFormulas();
    pruneGraphConnections();

    svgLayer.replaceChildren();
    nodesLayer.replaceChildren();

    state.sequence.forEach((step, idx) => {
      if (!step.id) step.id = `step_${idx + 1}`;
      if (!state.nodePositions[step.id]) {
        const col = idx % 3;
        const row = Math.floor(idx / 3);
        state.nodePositions[step.id] = { x: 60 + col * 320, y: 80 + row * 220 };
      }
    });

    nodesLayer.style.transform = `translate(${state.panX}px, ${state.panY}px) scale(${state.zoom})`;
    nodesLayer.style.transformOrigin = '0 0';
    svgLayer.setAttribute('transform', `translate(${state.panX}, ${state.panY}) scale(${state.zoom})`);

    graphConnections().forEach((connection, connectionIndex) => {
      const fromStep = state.sequence.find(step => step.id === connection.from);
      const toStep = state.sequence.find(step => step.id === connection.to);
      if (!fromStep || !toStep) return;
      const fromPos = state.nodePositions[fromStep.id] || { x: 0, y: 0 };
      const toPos = state.nodePositions[toStep.id] || { x: 0, y: 0 };

      const startX = fromPos.x + 280;
      const startY = fromPos.y + 90;
      const endX = toPos.x;
      const endY = toPos.y + 90;

      const dx = Math.max(40, Math.abs(endX - startX) * 0.5);
      const edge = { id: connection.id };
      const fallback = `M ${startX} ${startY} C ${startX + dx} ${startY}, ${endX - dx} ${endY}, ${endX} ${endY}`;
      const pathData = routedPath(edge, fallback);

      const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path.setAttribute('d', pathData);
      path.setAttribute('fill', 'none');
      const isExecuting = state.activeExecutingStepIdx === connectionIndex;
      path.setAttribute('stroke', isExecuting ? '#3b82f6' : 'var(--line-strong)');
      path.setAttribute('stroke-width', isExecuting ? '3' : '2');
      path.setAttribute('stroke-dasharray', isExecuting ? '6,3' : 'none');
      if (isExecuting) path.classList.add('at-wire-pulse');
      path.classList.add('at-graph-edge');
      path.dataset.edgeId = connection.id;
      path.onclick = (event) => {
        event.stopPropagation();
        state.portSelection = { edgeId: connection.id };
        getO().toast(`Связь выбрана: ${fromStep.name} → ${toStep.name}. Нажмите Delete для удаления.`, '');
      };
      svgLayer.appendChild(path);

      const arrow = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
      arrow.setAttribute('cx', String(endX));
      arrow.setAttribute('cy', String(endY));
      arrow.setAttribute('r', '4');
      arrow.setAttribute('fill', isExecuting ? '#3b82f6' : 'var(--accent)');
      svgLayer.appendChild(arrow);
    });

    state.sequence.forEach((step, idx) => {
      const pos = state.nodePositions[step.id] || { x: 60 + idx * 300, y: 80 };
      const autoKind = step.automation || (MODBUS_COMMANDS.find(c => Number(c.cmd) === Number(step.cmd))?.category || 'automatic');
      const autoMeta = AUTOMATION_TYPES[autoKind] || AUTOMATION_TYPES.automatic;
      const durationSec = state.formulaResults[step.id] || 15;
      const isExecuting = state.activeExecutingStepIdx === idx;

      const node = document.createElement('div');
      node.className = `at-node-card ${isExecuting ? 'is-executing' : ''} ${state.selectedStepId === step.id ? 'is-selected' : ''}`;
      node.id = `at-node-${step.id}`;
      node.style.left = `${pos.x}px`;
      node.style.top = `${pos.y}px`;
      node.style.borderTopColor = autoMeta.color;

      if (state.sequence.length > 1) {
        const pinIn = document.createElement('div');
        pinIn.className = 'at-port-pin at-port-in';
        pinIn.title = 'Входная связь';
        pinIn.onclick = (event) => handlePortClick(event, step.id, 'in');
        node.appendChild(pinIn);
      }
      if (state.sequence.length > 1) {
        const pinOut = document.createElement('div');
        pinOut.className = 'at-port-pin at-port-out';
        pinOut.title = 'Выходная связь';
        pinOut.onclick = (event) => handlePortClick(event, step.id, 'out');
        if (state.portSelection?.nodeId === step.id && state.portSelection.direction === 'out') pinOut.classList.add('is-selected');
        node.appendChild(pinOut);
      }

      const header = document.createElement('div');
      header.className = 'at-node-header';

      const headerLeft = document.createElement('div');
      headerLeft.className = 'at-node-header-left';

      const stepBadge = document.createElement('span');
      stepBadge.className = 'at-step-num';
      stepBadge.textContent = String(idx + 1).padStart(2, '0');

      const autoBadge = document.createElement('span');
      autoBadge.className = 'at-auto-badge';
      autoBadge.style.color = autoMeta.color;
      autoBadge.style.background = autoMeta.bg;
      autoBadge.style.borderColor = autoMeta.line;
      autoBadge.textContent = autoMeta.label;

      headerLeft.append(stepBadge, autoBadge);

      const headerActions = document.createElement('div');
      headerActions.className = 'at-node-header-actions';

      const btnTest = document.createElement('button');
      btnTest.className = 'at-mini-btn at-btn-test';
      btnTest.title = 'Тест шага на контроллере';
      btnTest.innerHTML = '▶ Тест';
      btnTest.onclick = (e) => {
        e.stopPropagation();
        testSingleStep(step);
      };

      const btnClone = document.createElement('button');
      btnClone.className = 'at-mini-btn';
      btnClone.title = 'Дублировать шаг';
      btnClone.innerHTML = '📋';
      btnClone.onclick = (e) => {
        e.stopPropagation();
        cloneStep(idx);
      };

      const btnDelete = document.createElement('button');
      btnDelete.className = 'at-mini-btn at-btn-del';
      btnDelete.title = 'Удалить шаг';
      btnDelete.innerHTML = '✕';
      btnDelete.onclick = (e) => {
        e.stopPropagation();
        deleteStep(idx);
      };

      headerActions.append(btnTest, btnClone, btnDelete);
      header.append(headerLeft, headerActions);

      const titleInp = document.createElement('input');
      titleInp.type = 'text';
      titleInp.className = 'inp at-node-title-inp';
      titleInp.value = step.name || `Шаг ${idx + 1}`;
      titleInp.placeholder = 'Название операции...';
      titleInp.oninput = () => {
        step.name = titleInp.value.trim();
        saveState();
        updateStatsBar();
      };
      titleInp.onmousedown = (e) => e.stopPropagation();

      const body = document.createElement('div');
      body.className = 'at-node-body';

      const rowCmd = document.createElement('div');
      rowCmd.className = 'at-field-row';
      const lblCmd = document.createElement('label');
      lblCmd.textContent = 'Команда Modbus:';
      const selCmd = document.createElement('select');
      selCmd.className = 'inp at-select-cmd';
      MODBUS_COMMANDS.forEach(opt => {
        const o = document.createElement('option');
        o.value = opt.cmd;
        o.textContent = opt.name;
        if (Number(opt.cmd) === Number(step.cmd)) o.selected = true;
        selCmd.appendChild(o);
      });
      selCmd.onchange = () => {
        step.cmd = Number(selCmd.value);
        const meta = MODBUS_COMMANDS.find(c => c.cmd === step.cmd);
        if (meta) step.automation = meta.category;
        saveState();
        renderFlowCanvas();
      };
      selCmd.onmousedown = (e) => e.stopPropagation();
      rowCmd.append(lblCmd, selCmd);

      const typeRow = document.createElement('div');
      typeRow.className = 'at-field-row';
      const typeLabel = document.createElement('label');
      typeLabel.textContent = 'Тип блока:';
      const typeSelect = document.createElement('select');
      typeSelect.className = 'inp at-select-cmd';
      BLOCK_TYPES.forEach(type => {
        const option = document.createElement('option');
        option.value = type.id;
        option.textContent = type.label;
        option.selected = type.id === step.block_type;
        typeSelect.appendChild(option);
      });
      typeSelect.onchange = () => { step.block_type = typeSelect.value; saveState(); renderFlowCanvas(); };
      typeSelect.onmousedown = (e) => e.stopPropagation();
      typeRow.append(typeLabel, typeSelect);

      const controlGrid = document.createElement('div');
      controlGrid.className = 'at-param-grid';
      const variantCol = document.createElement('div');
      variantCol.className = 'at-param-col';
      const variantLabel = document.createElement('span');
      variantLabel.textContent = 'Вариант';
      const variantInput = document.createElement('input');
      variantInput.className = 'inp at-compact-inp';
      variantInput.value = step.variant;
      variantInput.placeholder = 'default';
      variantInput.oninput = () => { step.variant = variantInput.value.trim() || 'default'; saveState(); };
      variantInput.onmousedown = (e) => e.stopPropagation();
      variantCol.append(variantLabel, variantInput);

      const retryCol = document.createElement('div');
      retryCol.className = 'at-param-col';
      const retryLabel = document.createElement('span');
      retryLabel.textContent = 'Повторы';
      const retryInput = document.createElement('input');
      retryInput.type = 'number';
      retryInput.min = '0';
      retryInput.max = '9';
      retryInput.className = 'inp at-compact-inp';
      retryInput.value = step.retry_count;
      retryInput.oninput = () => { step.retry_count = Math.min(9, Math.max(0, Number(retryInput.value) || 0)); saveState(); };
      retryInput.onmousedown = (e) => e.stopPropagation();
      retryCol.append(retryLabel, retryInput);
      controlGrid.append(variantCol, retryCol);

      const conditionInput = document.createElement('input');
      conditionInput.className = 'inp at-formula-inp';
      conditionInput.value = step.condition;
      conditionInput.placeholder = 'Условие перехода: reagent_empty == false';
      conditionInput.title = 'Декларативное условие; исполняется только проверенным backend-движком';
      conditionInput.oninput = () => { step.condition = conditionInput.value.trim(); saveState(); };
      conditionInput.onmousedown = (e) => e.stopPropagation();

      const paramGrid = document.createElement('div');
      paramGrid.className = 'at-param-grid';

      const colZone = document.createElement('div');
      colZone.className = 'at-param-col';
      const lblZone = document.createElement('span');
      lblZone.textContent = 'Зона';
      const selZone = document.createElement('select');
      selZone.className = 'inp at-compact-inp';
      [
        { val: 0, txt: 'Текущая' },
        { val: 1, txt: 'Зона 1' },
        { val: 2, txt: 'Зона 2' },
        { val: 3, txt: 'Зона 3' }
      ].forEach(z => {
        const opt = document.createElement('option');
        opt.value = z.val;
        opt.textContent = z.txt;
        if (Number(step.zone || 0) === z.val) opt.selected = true;
        selZone.appendChild(opt);
      });
      selZone.onchange = () => {
        step.zone = Number(selZone.value);
        saveState();
      };
      selZone.onmousedown = (e) => e.stopPropagation();
      colZone.append(lblZone, selZone);

      const colDelay = document.createElement('div');
      colDelay.className = 'at-param-col';
      const lblDelay = document.createElement('span');
      lblDelay.textContent = 'Задержка (с)';
      const inpDelay = document.createElement('input');
      inpDelay.type = 'number';
      inpDelay.className = 'inp at-compact-inp';
      inpDelay.min = '0';
      inpDelay.value = step.delay_sec ?? 0;
      inpDelay.oninput = () => {
        step.delay_sec = Math.max(0, Number(inpDelay.value));
        saveState();
        updateStatsBar();
      };
      inpDelay.onmousedown = (e) => e.stopPropagation();
      colDelay.append(lblDelay, inpDelay);

      const colTimeout = document.createElement('div');
      colTimeout.className = 'at-param-col';
      const lblTimeout = document.createElement('span');
      lblTimeout.textContent = 'Timeout (с)';
      const inpTimeout = document.createElement('input');
      inpTimeout.type = 'number';
      inpTimeout.min = '1';
      inpTimeout.max = '86400';
      inpTimeout.className = 'inp at-compact-inp';
      inpTimeout.value = step.timeout_sec;
      inpTimeout.oninput = () => { step.timeout_sec = Math.min(86400, Math.max(1, Number(inpTimeout.value) || 1)); saveState(); };
      inpTimeout.onmousedown = (e) => e.stopPropagation();
      colTimeout.append(lblTimeout, inpTimeout);

      paramGrid.append(colZone, colDelay, colTimeout);

      const footer = document.createElement('div');
      footer.className = 'at-node-footer';

      const formulaInp = document.createElement('input');
      formulaInp.type = 'text';
      formulaInp.className = 'inp at-formula-inp';
      formulaInp.value = step.formula || '';
      formulaInp.placeholder = idx === 0 ? 'Формула: 15' : 'Формула: prev.time + 5';
      formulaInp.title = 'Формула Process Math (секунды)';
      formulaInp.oninput = () => {
        step.formula = formulaInp.value.trim();
        saveState();
        updateStatsBar();
      };
      formulaInp.onmousedown = (e) => e.stopPropagation();

      const durationBadge = document.createElement('span');
      durationBadge.className = 'at-duration-badge';
      durationBadge.textContent = `⏱ ~${durationSec}с`;

      footer.append(formulaInp, durationBadge);
      body.append(rowCmd, typeRow, controlGrid, conditionInput, paramGrid, footer);
      node.append(header, titleInp, body);

      setupNodeDrag(node, step.id);
      nodesLayer.appendChild(node);
    });
  }

  function setupNodeDrag(nodeElem, stepId) {
    let isDragging = false;
    let startMouseX = 0;
    let startMouseY = 0;
    let startNodeX = 0;
    let startNodeY = 0;

    nodeElem.addEventListener('mousedown', (e) => {
      if (['INPUT', 'SELECT', 'BUTTON', 'OPTION'].includes(e.target.tagName)) return;
      isDragging = true;
      state.selectedStepId = stepId;
      startMouseX = e.clientX;
      startMouseY = e.clientY;
      const currentPos = state.nodePositions[stepId] || { x: 0, y: 0 };
      startNodeX = currentPos.x;
      startNodeY = currentPos.y;
      nodeElem.classList.add('is-dragging');
      e.stopPropagation();

      const onMouseMove = (moveEvent) => {
        if (!isDragging) return;
        const dx = (moveEvent.clientX - startMouseX) / state.zoom;
        const dy = (moveEvent.clientY - startMouseY) / state.zoom;
        const newX = Math.round(startNodeX + dx);
        const newY = Math.round(startNodeY + dy);
        state.nodePositions[stepId] = { x: newX, y: newY };
        nodeElem.style.left = `${newX}px`;
        nodeElem.style.top = `${newY}px`;
        updateWiresOnly();
      };

      const onMouseUp = () => {
        isDragging = false;
        nodeElem.classList.remove('is-dragging');
        window.removeEventListener('mousemove', onMouseMove);
        window.removeEventListener('mouseup', onMouseUp);
        saveState();
      };

      window.addEventListener('mousemove', onMouseMove);
      window.addEventListener('mouseup', onMouseUp);
    });
  }

  function updateWiresOnly() {
    const svgLayer = document.getElementById('at-svg-layer');
    if (!svgLayer) return;
    svgLayer.replaceChildren();

    graphConnections().forEach((connection, connectionIndex) => {
      const fromStep = state.sequence.find(step => step.id === connection.from);
      const toStep = state.sequence.find(step => step.id === connection.to);
      if (!fromStep || !toStep) return;
      const fromPos = state.nodePositions[fromStep.id] || { x: 0, y: 0 };
      const toPos = state.nodePositions[toStep.id] || { x: 0, y: 0 };

      const startX = fromPos.x + 280;
      const startY = fromPos.y + 90;
      const endX = toPos.x;
      const endY = toPos.y + 90;

      const dx = Math.max(40, Math.abs(endX - startX) * 0.5);
      const edge = { id: connection.id };
      const fallback = `M ${startX} ${startY} C ${startX + dx} ${startY}, ${endX - dx} ${endY}, ${endX} ${endY}`;
      const pathData = routedPath(edge, fallback);

      const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path.setAttribute('d', pathData);
      path.setAttribute('fill', 'none');
      const isExecuting = state.activeExecutingStepIdx === connectionIndex;
      path.setAttribute('stroke', isExecuting ? '#3b82f6' : 'var(--line-strong)');
      path.setAttribute('stroke-width', isExecuting ? '3' : '2');
      if (isExecuting) path.classList.add('at-wire-pulse');
      svgLayer.appendChild(path);

      const arrow = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
      arrow.setAttribute('cx', String(endX));
      arrow.setAttribute('cy', String(endY));
      arrow.setAttribute('r', '4');
      arrow.setAttribute('fill', isExecuting ? '#3b82f6' : 'var(--accent)');
      svgLayer.appendChild(arrow);
    });
  }

  function setupCanvasPanZoom() {
    const viewport = document.getElementById('at-canvas-viewport');
    if (!viewport) return;

    viewport.addEventListener('mousedown', (e) => {
      if (e.target !== viewport && e.target.id !== 'at-canvas-container' && e.target.id !== 'at-svg-layer') return;
      state.isPanning = true;
      state.dragStart = { x: e.clientX - state.panX, y: e.clientY - state.panY };
      viewport.style.cursor = 'grabbing';
    });

    window.addEventListener('mousemove', (e) => {
      if (!state.isPanning) return;
      state.panX = e.clientX - state.dragStart.x;
      state.panY = e.clientY - state.dragStart.y;
      renderFlowCanvas();
    });

    window.addEventListener('mouseup', () => {
      if (state.isPanning) {
        state.isPanning = false;
        viewport.style.cursor = 'grab';
        saveState();
      }
    });

    viewport.addEventListener('wheel', (e) => {
      e.preventDefault();
      const zoomFactor = e.deltaY < 0 ? 1.1 : 0.9;
      const newZoom = Math.min(2.0, Math.max(0.4, state.zoom * zoomFactor));
      state.zoom = parseFloat(newZoom.toFixed(2));
      renderFlowCanvas();
      const zoomBadge = document.getElementById('at-zoom-val');
      if (zoomBadge) zoomBadge.textContent = `${Math.round(state.zoom * 100)}%`;
    }, { passive: false });
  }

  function renderButtonMatrix() {
    const container = document.getElementById('at-buttons-grid');
    if (!container) return;
    container.replaceChildren();

    const O = getO();

    state.customButtons.forEach((btn, idx) => {
      const card = document.createElement('div');
      card.className = `at-btn-matrix-card at-btn-color-${btn.color || 'blue'}`;

      const header = document.createElement('div');
      header.className = 'at-btn-card-header';

      const cmdBadge = document.createElement('span');
      cmdBadge.className = 'at-btn-cmd-code';
      cmdBadge.textContent = `CMD ${btn.cmd}`;

      const cardActions = document.createElement('div');
      cardActions.className = 'at-btn-card-actions';

      const btnEdit = document.createElement('button');
      btnEdit.className = 'at-btn-icon-action';
      btnEdit.title = 'Настроить кнопку';
      btnEdit.textContent = '✎';
      btnEdit.onclick = (e) => {
        e.stopPropagation();
        openButtonEditModal(idx);
      };

      const btnDel = document.createElement('button');
      btnDel.className = 'at-btn-icon-action at-btn-del-action';
      btnDel.title = 'Удалить кнопку';
      btnDel.textContent = '✕';
      btnDel.onclick = (e) => {
        e.stopPropagation();
        deleteCustomButton(idx);
      };

      cardActions.append(btnEdit, btnDel);
      header.append(cmdBadge, cardActions);

      const triggerBtn = document.createElement('button');
      triggerBtn.type = 'button';
      triggerBtn.className = 'at-matrix-trigger-btn requires-connection';
      triggerBtn.disabled = !O.state?.connected || O.state?.busy;

      const title = document.createElement('strong');
      title.className = 'at-matrix-btn-title';
      title.textContent = btn.title;

      const subtitle = document.createElement('span');
      subtitle.className = 'at-matrix-btn-subtitle';
      subtitle.textContent = btn.subtitle || `Команда Modbus ${btn.cmd}`;

      triggerBtn.append(title, subtitle);
      triggerBtn.onclick = () => executeMatrixButton(btn, triggerBtn);

      card.append(header, triggerBtn);
      container.appendChild(card);
    });

    const addCard = document.createElement('div');
    addCard.className = 'at-btn-matrix-card at-btn-add-card';
    addCard.innerHTML = `
      <div class="at-add-btn-inner">
        <span class="at-plus-icon">+</span>
        <strong>Добавить кнопку Modbus</strong>
        <small>Настроить команду, цвет и параметры</small>
      </div>
    `;
    addCard.onclick = () => openButtonEditModal(-1);
    container.appendChild(addCard);
  }

  async function executeMatrixButton(btn, btnElem) {
    const O = getO();
    if (btn.confirm && btn.prompt) {
      if (!await O.confirm('Подтвердите команду', btn.prompt, 'Выполнить', btn.cmd === 999)) return;
    }

    if (btnElem) btnElem.classList.add('is-running');
    O.setBusy(true, `Выполнение: ${btn.title}`);

    try {
      if (btn.cmd === 999) {
        await O.request('/programs/emergency-stop', { method: 'POST' });
        O.toast('Аварийный останов выполнен.', 'ok');
      } else {
        await O.request('/programs/sequence/execute', {
          method: 'POST',
          body: {
            name: btn.title,
            steps: [{ name: btn.title, cmd: Number(btn.cmd), zone: Number(btn.zone || 3), delay_sec: 0, timeout_sec: 300 }]
          },
          timeout: 120000
        });
        O.toast(`Команда «${btn.title}» успешно отправлена.`, 'ok');
      }
    } catch (err) {
      O.toast(`Ошибка: ${err.message}`, 'error');
    } finally {
      O.setBusy(false);
      if (btnElem) btnElem.classList.remove('is-running');
    }
  }

  function openButtonEditModal(index) {
    const isNew = index < 0;
    const btnData = isNew
      ? { title: 'Новая команда', subtitle: 'Описание операции', cmd: 120, zone: 3, color: 'blue', confirm: true, prompt: 'Выполнить команду?' }
      : { ...state.customButtons[index] };

    const modal = document.getElementById('at-button-modal');
    if (!modal) return;

    document.getElementById('at-bm-title').value = btnData.title || '';
    document.getElementById('at-bm-subtitle').value = btnData.subtitle || '';
    document.getElementById('at-bm-cmd').value = btnData.cmd || 120;
    document.getElementById('at-bm-zone').value = btnData.zone || 3;
    document.getElementById('at-bm-color').value = btnData.color || 'blue';
    document.getElementById('at-bm-confirm').checked = Boolean(btnData.confirm);
    document.getElementById('at-bm-prompt').value = btnData.prompt || '';

    const saveBtn = document.getElementById('at-bm-save-btn');
    saveBtn.onclick = () => {
      btnData.title = document.getElementById('at-bm-title').value.trim() || 'Команда';
      btnData.subtitle = document.getElementById('at-bm-subtitle').value.trim();
      btnData.cmd = Number(document.getElementById('at-bm-cmd').value);
      btnData.zone = Number(document.getElementById('at-bm-zone').value);
      btnData.color = document.getElementById('at-bm-color').value;
      btnData.confirm = document.getElementById('at-bm-confirm').checked;
      btnData.prompt = document.getElementById('at-bm-prompt').value.trim();

      if (isNew) {
        state.customButtons.push(btnData);
      } else {
        state.customButtons[index] = btnData;
      }
      saveState();
      renderButtonMatrix();
      modal.hidden = true;
      getO().toast('Кнопка сохранена', 'ok');
    };

    modal.hidden = false;
  }

  function deleteCustomButton(index) {
    state.customButtons.splice(index, 1);
    saveState();
    renderButtonMatrix();
    getO().toast('Кнопка удалена', 'ok');
  }

  function renderCardsWorkbench() {
    const container = document.getElementById('at-cards-container');
    if (!container) return;
    container.replaceChildren();

    evaluateFormulas();

    state.sequence.forEach((step, idx) => {
      const autoKind = step.automation || (MODBUS_COMMANDS.find(c => Number(c.cmd) === Number(step.cmd))?.category || 'automatic');
      const autoMeta = AUTOMATION_TYPES[autoKind] || AUTOMATION_TYPES.automatic;
      const durationSec = state.formulaResults[step.id] || 15;

      const card = document.createElement('div');
      card.className = 'at-workbench-card';
      card.style.borderLeft = `4px solid ${autoMeta.color}`;

      card.innerHTML = `
        <div class="at-wb-card-header">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span class="at-step-num">${String(idx + 1).padStart(2, '0')}</span>
            <strong>${step.name}</strong>
            <span class="at-auto-badge" style="color:${autoMeta.color};background:${autoMeta.bg};border-color:${autoMeta.line};">${autoMeta.label}</span>
          </div>
          <div style="display: flex; gap: 6px;">
            <button class="btn btn-secondary at-mini-btn" onclick="AutoTraceEditor.moveStep(${idx}, -1)" ${idx === 0 ? 'disabled' : ''}>↑</button>
            <button class="btn btn-secondary at-mini-btn" onclick="AutoTraceEditor.moveStep(${idx}, 1)" ${idx === state.sequence.length - 1 ? 'disabled' : ''}>↓</button>
            <button class="btn btn-secondary at-mini-btn" onclick="AutoTraceEditor.cloneStep(${idx})">📋</button>
            <button class="btn btn-secondary at-mini-btn at-btn-del" onclick="AutoTraceEditor.deleteStep(${idx})">✕</button>
          </div>
        </div>
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-top: 10px;">
          <div><small class="muted">Команда Modbus:</small><div><strong>${step.cmd} · ${MODBUS_COMMANDS.find(c => c.cmd === step.cmd)?.name || 'Команда'}</strong></div></div>
          <div><small class="muted">Зона обработки:</small><div>Зона ${step.zone || '3 (Обе)'}</div></div>
          <div><small class="muted">Задержка / Таймаут:</small><div>${step.delay_sec || 0} с / ${step.timeout_sec || 300} с</div></div>
          <div><small class="muted">Process Math Время:</small><div><strong style="color:var(--accent);">⏱ ~${durationSec} с</strong> (${step.formula || 'константа'})</div></div>
        </div>
      `;
      container.appendChild(card);
    });
  }

  function renderMathWorkbench() {
    const stats = evaluateFormulas();
    const container = document.getElementById('at-math-container');
    if (!container) return;

    container.innerHTML = `
      <div class="at-math-grid">
        <div class="work-panel">
          <div class="panel-heading"><div><h3>Сводные метрики DAG-процесса</h3><p>Математическая модель времени и надежности цикла</p></div></div>
          <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; padding: 16px;">
            <div class="measurement"><span>Критический путь DAG</span><strong>${stats.totalSec}</strong><small>секунд (~${stats.totalMin} мин)</small></div>
            <div class="measurement"><span>Степень автоматизации</span><strong>${stats.autoRatio}%</strong><small>автоматических фаз</small></div>
            <div class="measurement"><span>Автоматическое время</span><strong>${stats.autoSec}</strong><small>секунд</small></div>
            <div class="measurement"><span>Ручные / QC операции</span><strong>${stats.manualSec}</strong><small>секунд</small></div>
            <div class="measurement"><span>Ожидание / Экспозиция</span><strong>${stats.waitSec}</strong><small>секунд</small></div>
            <div class="measurement"><span>Узкое место (Bottleneck)</span><strong style="font-size:16px;">${stats.bottleneck}</strong><small>максимальная длительность</small></div>
          </div>
        </div>
      </div>
    `;
  }

  function addStep() {
    const nextIdx = state.sequence.length + 1;
    const newStep = normalizeStep({
      id: `step_${Date.now()}`,
      name: `Шаг ${nextIdx}`,
      cmd: 120,
      zone: 3,
      delay_sec: 0,
      timeout_sec: 300,
      automation: 'automatic',
      formula: '15'
    }, state.sequence.length);
    state.sequence.push(newStep);
    saveState();
    autoLayoutNodes();
    updateStatsBar();
    getO().toast(`Шаг ${nextIdx} добавлен в сценарий`, 'ok');
  }

  function cloneStep(index) {
    const orig = state.sequence[index];
    if (!orig) return;
    const cloned = { ...orig, id: `step_${Date.now()}`, name: `${orig.name} (копия)` };
    state.sequence.splice(index + 1, 0, cloned);
    saveState();
    autoLayoutNodes();
    updateStatsBar();
    getO().toast('Шаг дублирован', 'ok');
  }

  function deleteStep(index) {
    state.sequence.splice(index, 1);
    saveState();
    autoLayoutNodes();
    updateStatsBar();
    getO().toast('Шаг удален', 'ok');
  }

  function moveStep(index, delta) {
    const targetIdx = index + delta;
    if (targetIdx < 0 || targetIdx >= state.sequence.length) return;
    const temp = state.sequence[index];
    state.sequence[index] = state.sequence[targetIdx];
    state.sequence[targetIdx] = temp;
    saveState();
    autoLayoutNodes();
    updateStatsBar();
  }

  function loadPreset(presetKey) {
    const preset = PRESET_SEQUENCES[presetKey];
    if (!preset) return;
    state.sequence = preset.steps.map((s, index) => normalizeStep({ ...s, id: `step_${Date.now()}_${index}` }, index));
    saveState();
    autoLayoutNodes();
    updateStatsBar();
    getO().toast(`Шаблон «${preset.name}» загружен`, 'ok');
  }

  function validateSequence() {
    const issues = [];
    state.sequence.forEach((step, index) => {
      const label = `Шаг ${index + 1} «${step.name || 'без названия'}»`;
      if (!BLOCK_TYPES.some(type => type.id === step.block_type)) issues.push(`${label}: неизвестный тип блока`);
      if (step.block_type === 'condition' && !step.condition) issues.push(`${label}: задайте условие`);
      if (step.block_type === 'variant' && (!step.variant || step.variant === 'default')) issues.push(`${label}: задайте имя варианта`);
      if (step.block_type === 'subgraph' && !step.subgraph_id) issues.push(`${label}: не выбран подграф`);
      if (!Number.isInteger(Number(step.cmd)) || Number(step.cmd) < 0) issues.push(`${label}: некорректная Modbus-команда`);
      if (Number(step.timeout_sec) < 1) issues.push(`${label}: timeout должен быть больше нуля`);
      if (Number(step.retry_count) < 0 || Number(step.retry_count) > 9) issues.push(`${label}: повторы должны быть от 0 до 9`);
    });
    return issues;
  }

  function buildAlgorithmModel() {
    const nodes = state.sequence.map((step, index) => ({
      id: step.id || `step_${index + 1}`,
      kind: step.block_type || 'command',
      label: step.name || `Шаг ${index + 1}`,
      variant: step.variant || 'default',
      command: Number(step.cmd || 0),
      config: { zone: String(step.zone ?? 3), timeout_sec: String(step.timeout_sec || 300), retry_count: String(step.retry_count || 0) }
    }));
    const edges = graphConnections();
    return { id: 'onepap-main-algorithm', nodes, edges, parameters: state.parameters, subgraphs: state.subgraphs };
  }

  async function validateAlgorithmModel() {
    const O = getO();
    try {
      const result = await O.request('/autotrace/algorithm/validate', { method: 'POST', body: buildAlgorithmModel(), timeout: 10000 });
      O.toast(`Алгоритм валиден: ${result.nodes} блоков, ${result.edges} связей, ${result.subgraphs} подграфов.`, 'ok');
    } catch (error) {
      O.toast(`Граф невалиден: ${error.message}`, 'error');
    }
  }

  function createSubgraph() {
    const O = getO();
    if (state.sequence.length < 2) { O.toast('Для подграфа нужно минимум два блока.', 'error'); return; }
    const name = window.prompt('Название нового подграфа:', 'Подграф ONEPAP');
    if (!name?.trim()) return;
    const parameterName = window.prompt('Имя входного параметра (можно оставить пустым):', 'zone');
    const id = `subgraph_${Date.now()}`;
    state.subgraphs.push({
      id, name: name.trim(), entry: state.sequence[0].id, exit: state.sequence[state.sequence.length - 1].id,
      nodeIds: state.sequence.map(step => step.id),
      inputs: parameterName?.trim() ? [{ name: parameterName.trim(), type: 'string', required: false, default: '' }] : []
    });
    saveState();
    O.toast(`Подграф «${name.trim()}» создан с ${state.sequence.length} блоками.`, 'ok');
  }

  async function calculateRoutes() {
    const O = getO();
    if (state.sequence.length < 2) {
      O.toast('Добавьте минимум два шага для расчёта трасс.', 'error');
      return;
    }
    O.setBusy(true, 'AutoTrace Lab рассчитывает трассы');
    try {
      const nodes = state.sequence.map((step, index) => {
        if (!step.id) step.id = `step_${index + 1}`;
        const position = state.nodePositions[step.id] || { x: 60 + (index % 3) * 320, y: 80 + Math.floor(index / 3) * 220 };
        return {
          id: step.id,
          title: step.name || `Шаг ${index + 1}`,
          subtitle: `Modbus ${step.cmd}`,
          category: step.automation || 'automatic',
          x: position.x,
          y: position.y,
          width: 280,
          height: 180,
          inputs: [{ id: 'in', name: 'Вход', type: 'process', side: 'left' }],
          outputs: [{ id: 'out', name: 'Выход', type: 'process', side: 'right' }]
        };
      });
      const edges = graphConnections().map(connection => ({
        id: connection.id,
        sourceBlockId: connection.from,
        sourcePortId: 'out',
        targetBlockId: connection.to,
        targetPortId: 'in'
      }));
      const result = await O.request('/autotrace/route', {
        method: 'POST',
        body: { graphId: 'onepap-autotrace', nodes, edges, options: {} },
        timeout: 10000
      });
      state.routedEdges = {};
      (result.edges || []).forEach(edge => { state.routedEdges[edge.id] = edge.path || []; });
      renderFlowCanvas();
      O.toast(`AutoTrace Lab: рассчитано трасс ${state.routedEdges ? Object.keys(state.routedEdges).length : 0}.`, 'ok');
    } catch (error) {
      state.routedEdges = {};
      renderFlowCanvas();
      O.toast(`AutoTrace Lab недоступен: ${error.message}`, 'error');
    } finally {
      O.setBusy(false);
    }
  }

  async function testSingleStep(step) {
    const O = getO();
    if (!await O.confirm(`Тест шага: ${step.name}?`, `Будет отправлена Modbus-команда ${step.cmd} (Зона ${step.zone || 3}).`, 'Запустить тест')) return;
    O.setBusy(true, `Тест: ${step.name}`);
    try {
      await O.request('/programs/sequence/execute', {
        method: 'POST',
        body: {
          name: `Тест: ${step.name}`,
          steps: [{ name: step.name, cmd: Number(step.cmd), zone: Number(step.zone || 3), delay_sec: 0, timeout_sec: 120 }]
        },
        timeout: 120000
      });
      O.toast(`Тест «${step.name}» успешно запущен`, 'ok');
    } catch (e) {
      O.toast(`Ошибка теста: ${e.message}`, 'error');
    } finally {
      O.setBusy(false);
    }
  }

  function exportJSON() {
    const data = {
      generator: 'ONEPAP.24 AutoTrace Modbus Editor',
      version: '2.0',
      exportedAt: new Date().toISOString(),
      sequence: state.sequence,
      buttons: state.customButtons,
      algorithm: buildAlgorithmModel()
    };
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `onepap_modbus_sequence_${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
    getO().toast('Конфигурация экспортирована в JSON', 'ok');
  }

  function importJSON(file) {
    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        const parsed = JSON.parse(e.target.result);
        if (Array.isArray(parsed.sequence)) {
          state.sequence = parsed.sequence;
        } else if (Array.isArray(parsed)) {
          state.sequence = parsed;
        }
        if (Array.isArray(parsed.buttons)) {
          state.customButtons = parsed.buttons;
        }
        if (Array.isArray(parsed.algorithm?.subgraphs)) state.subgraphs = parsed.algorithm.subgraphs;
        if (Array.isArray(parsed.algorithm?.parameters)) state.parameters = parsed.algorithm.parameters;
        saveState();
        autoLayoutNodes();
        updateStatsBar();
        getO().toast('Конфигурация успешно импортирована!', 'ok');
      } catch (err) {
        getO().toast(`Ошибка импорта JSON: ${err.message}`, 'error');
      }
    };
    reader.readAsText(file);
  }

  function handleTelemetryProgress(data) {
    if (data && data.step != null && data.total != null) {
      state.activeExecutingStepIdx = data.step - 1;
      updateWiresOnly();
      const node = document.getElementById(`at-node-${state.sequence[data.step - 1]?.id}`);
      if (node) {
        document.querySelectorAll('.at-node-card').forEach(n => n.classList.remove('is-executing'));
        node.classList.add('is-executing');
      }
    } else {
      state.activeExecutingStepIdx = null;
      document.querySelectorAll('.at-node-card').forEach(n => n.classList.remove('is-executing'));
      updateWiresOnly();
    }
  }

  function init() {
    loadState();
    setupCanvasPanZoom();

    const btnZoomIn = document.getElementById('at-btn-zoom-in');
    const btnZoomOut = document.getElementById('at-btn-zoom-out');
    const btnZoomReset = document.getElementById('at-btn-zoom-reset');
    const btnAutoLayout = document.getElementById('at-btn-autolayout');
    let btnRoute = document.getElementById('at-btn-route');
    if (!btnRoute && btnAutoLayout?.parentElement) {
      btnRoute = document.createElement('button');
      btnRoute.id = 'at-btn-route';
      btnRoute.type = 'button';
      btnRoute.className = 'btn btn-secondary at-tool-btn';
      btnRoute.title = 'Маршрутизация через AutoTrace Lab Go engine';
      btnRoute.textContent = '⟐ Рассчитать трассы';
      btnAutoLayout.parentElement.appendChild(btnRoute);
    }
    let btnValidate = document.getElementById('at-btn-validate');
    if (!btnValidate && btnAutoLayout?.parentElement) {
      btnValidate = document.createElement('button');
      btnValidate.id = 'at-btn-validate';
      btnValidate.type = 'button';
      btnValidate.className = 'btn btn-secondary at-tool-btn';
      btnValidate.title = 'Проверить общий алгоритм на backend';
      btnValidate.textContent = '✓ Проверить граф';
      btnAutoLayout.parentElement.appendChild(btnValidate);
    }
    let btnSubgraph = document.getElementById('at-btn-subgraph');
    if (!btnSubgraph && btnAutoLayout?.parentElement) {
      btnSubgraph = document.createElement('button');
      btnSubgraph.id = 'at-btn-subgraph';
      btnSubgraph.type = 'button';
      btnSubgraph.className = 'btn btn-secondary at-tool-btn';
      btnSubgraph.title = 'Сохранить текущую цепочку как параметризованный подграф';
      btnSubgraph.textContent = '＋ Подграф';
      btnAutoLayout.parentElement.appendChild(btnSubgraph);
    }
    const btnAddStep = document.getElementById('btn-seq-add');
    const btnExport = document.getElementById('at-btn-export');
    const btnImport = document.getElementById('at-btn-import');
    const fileImport = document.getElementById('at-file-import');

    if (btnZoomIn) btnZoomIn.onclick = () => { state.zoom = Math.min(2.0, parseFloat((state.zoom + 0.15).toFixed(2))); renderFlowCanvas(); };
    if (btnZoomOut) btnZoomOut.onclick = () => { state.zoom = Math.max(0.4, parseFloat((state.zoom - 0.15).toFixed(2))); renderFlowCanvas(); };
    if (btnZoomReset) btnZoomReset.onclick = () => { state.zoom = 1.0; state.panX = 40; state.panY = 30; renderFlowCanvas(); };
    if (btnAutoLayout) btnAutoLayout.onclick = () => autoLayoutNodes();
    if (btnRoute) btnRoute.onclick = () => calculateRoutes();
    if (btnValidate) btnValidate.onclick = () => validateAlgorithmModel();
    if (btnSubgraph) btnSubgraph.onclick = () => createSubgraph();
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Delete' && state.portSelection?.edgeId && !['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement?.tagName)) {
        removeSelectedConnection();
      }
      if (event.key === 'Escape' && state.portSelection) {
        state.portSelection = null;
        renderFlowCanvas();
      }
    });
    if (btnAddStep) btnAddStep.onclick = () => addStep();
    const runSequence = document.getElementById('btn-seq-run');
    if (runSequence) runSequence.addEventListener('click', (event) => {
      const issues = validateSequence();
      if (issues.length > 0) {
        event.preventDefault();
        event.stopImmediatePropagation();
        getO().toast(`Сценарий не готов: ${issues[0]}${issues.length > 1 ? ` (+${issues.length - 1})` : ''}`, 'error');
      }
    }, true);
    if (btnExport) btnExport.onclick = () => exportJSON();
    if (btnImport && fileImport) {
      btnImport.onclick = () => fileImport.click();
      fileImport.onchange = (e) => {
        if (e.target.files?.[0]) importJSON(e.target.files[0]);
      };
    }

    document.querySelectorAll('.at-view-tab').forEach(tab => {
      tab.onclick = () => switchView(tab.dataset.view);
    });

    document.querySelectorAll('.seq-preset-btn').forEach(btn => {
      btn.onclick = () => loadPreset(btn.dataset.preset);
    });

    switchView(state.activeView || 'canvas');
  }

  return {
    init,
    switchView,
    addStep,
    cloneStep,
    deleteStep,
    moveStep,
    loadPreset,
    autoLayoutNodes,
    calculateRoutes,
    buildAlgorithmModel,
    validateAlgorithmModel,
    createSubgraph,
    exportJSON,
    importJSON,
    validateSequence,
    handleTelemetryProgress,
    getState: () => state
  };
})();
