'use strict';

(() => {
  const O=window.ONE24;
  const PROGRAMS={
    'system-check':['/programs/system-check','Запустить проверку системы?'],load:['/programs/load','Запустить загрузку образцов?'],
    sedimentation:['/programs/sedimentation','Запустить осаждение на 300 секунд?'],'stain-start':['/programs/stain/start','Запустить цикл окраски?'],
    'stain-pause':['/programs/stain/pause',null],'stain-resume':['/programs/stain/resume',null],'stain-stop':['/programs/stain/stop','Остановить текущий цикл окраски?'],
    'wash-start':['/programs/wash/start','Запустить промывку системы?'],'full-start':['/programs/full/start','Запустить полный цикл? Проверьте реагенты, образцы и защитные элементы.']
  };
  const SENSOR_NAMES=['Вход 0 · жидкость','Вход 1 · ротор','Вход 2 · стекло 1-1','Вход 3 · стекло 1-2','Вход 4 · стекло 2-1','Вход 5 · стекло 2-2','Вход 6','Вход 7','Вход 8','Вход 9 · селектор 1','Вход 10 · селектор 2','Вход 11','Вход 12'];
const SENSOR_GROUPS=[{label:'Жидкость и ротор',range:[0,1]},{label:'Концевики стёкол',range:[2,5]},{label:'Дискретные входы',range:[6,8]},{label:'Селекторы',range:[9,10]},{label:'Резерв',range:[11,12]}];
  Object.assign(O.state,{selector:1,selectorPos:{1:null,2:null}});
  O.updateOperationsAvailability=()=>O.$$('.sel-hole-btn').forEach((button)=>{button.disabled=!O.state.connected||O.state.busy;});

  async function runProgram(key){
    const item=PROGRAMS[key];if(!item)return;const [path,question]=item;
    if(question&&!await O.confirm('Подтвердите запуск',question,key.includes('stop')?'Остановить':'Запустить',key.includes('stop')))return;
    O.setBusy(true,'Отправка команды');
    try{await O.request(path,{method:'POST',timeout:120000});O.toast('Команда принята контроллером.','ok');if(key.includes('start'))O.$('#progress-wrap').hidden=false;}
    catch(error){O.toast(`Команда не выполнена: ${error.message}`,'error');}finally{O.setBusy(false);}
  }
  async function emergency(){if(!await O.confirm('Аварийный останов','Немедленно остановить исполнительные механизмы и текущую программу?','Аварийный останов',true))return;try{await O.request('/programs/emergency-stop',{method:'POST'});O.hideProgress();O.toast('Команда аварийного останова отправлена.','ok');}catch(error){O.toast(error.message,'error');}}
  function selectSelector(number){O.state.selector=number;O.$('#sel-btn-1').classList.toggle('active',number===1);O.$('#sel-btn-2').classList.toggle('active',number===2);O.$('#sel-btn-1').setAttribute('aria-pressed',String(number===1));O.$('#sel-btn-2').setAttribute('aria-pressed',String(number===2));O.text('#btn-calibrate-sel',`Обнулить селектор ${number}`);renderSelector();}
  function renderSelector(){const number=O.state.selector;O.$$('.sel-hole-btn').forEach((button)=>button.classList.toggle('sel-active',Number(button.dataset.hole)===O.state.selectorPos[number]));O.text('#sel-status',O.state.selectorPos[number]?`Селектор ${number}: позиция ${O.state.selectorPos[number]}.`:`Позиция селектора ${number} не подтверждена контроллером.`);}
  async function moveSelector(hole){O.setBusy(true,`Перемещение селектора ${O.state.selector}`);try{await O.request('/testing/selector',{method:'POST',body:{num:O.state.selector,hole},timeout:120000});O.state.selectorPos[O.state.selector]=hole;renderSelector();O.toast(`Селектор ${O.state.selector} установлен в позицию ${hole}.`,'ok');}catch(error){O.toast(error.message,'error');}finally{O.setBusy(false);}}
  async function calibrate(){const number=O.state.selector;if(!await O.confirm(`Обнулить селектор ${number}?`,'Механизм вернётся в исходное положение. Убедитесь, что зона перемещения свободна.','Обнулить',true))return;O.setBusy(true,'Калибровка селектора');try{await O.request('/testing/selector/calibrate',{method:'POST',body:{num:number},timeout:120000});O.state.selectorPos[number]=1;renderSelector();O.toast(`Селектор ${number} обнулён.`,'ok');}catch(error){O.toast(error.message,'error');}finally{O.setBusy(false);}}
  async function resetPLC(){if(!await O.confirm('Сбросить PLC?','Контроллер будет перезагружен, текущая операция прервётся.','Сбросить PLC',true))return;O.setBusy(true,'Сброс PLC');try{await O.request('/programs/reset-plc',{method:'POST',timeout:120000});O.toast('PLC сброшен. Проверяется состояние.','ok');await O.refreshStatus();}catch(error){O.toast(error.message,'error');}finally{O.setBusy(false);}}

  function sensorPlaceholders(){const grid=O.$('#sensor-grid');grid.replaceChildren();SENSOR_GROUPS.forEach((group)=>{if(group.label){const header=document.createElement('div');header.className='sensor-group-label';header.textContent=group.label;grid.append(header);}for(let i=group.range[0];i<=group.range[1];i++){const item=document.createElement('div');item.className='sensor unknown';const title=document.createElement('span');title.textContent=SENSOR_NAMES[i];const value=document.createElement('strong');value.textContent='—';item.append(title,value);grid.append(item);}});}
  function renderSensors(data){const grid=O.$('#sensor-grid');grid.replaceChildren();const inputs=data.inputs||[];SENSOR_GROUPS.forEach((group)=>{if(group.label){const header=document.createElement('div');header.className='sensor-group-label';header.textContent=group.label;grid.append(header);}for(let i=group.range[0];i<=group.range[1];i++){const value=Boolean(inputs[i]);const item=document.createElement('div');item.className=`sensor ${value?'on':'off'}`;const title=document.createElement('span');title.textContent=SENSOR_NAMES[i]||`Вход ${i}`;const output=document.createElement('strong');output.textContent=value?'1 · активен':'0 · неактивен';item.append(title,output);grid.append(item);}});if(data.mcu_temp!=null)O.text('#s-mcu',Number(data.mcu_temp).toFixed(1));if(data.ntc_temp!=null)O.text('#s-ntc',Number(data.ntc_temp).toFixed(1));if(data.hx711_weight!=null)O.text('#s-hx711',data.hx711_weight);}
  async function readSensors(){try{renderSensors(await O.request('/testing/sensors'));O.text('#data-freshness',`Датчики: ${new Date().toLocaleTimeString()}`);}catch(error){O.toast(error.message,'error');}}

  O.addLog=(level='INFO',message='',timestamp)=>{const container=O.$('#log-container'),row=document.createElement('div');row.className=`log-entry ${level}`;const time=document.createElement('span');time.className='time';const date=timestamp?new Date(timestamp):new Date();time.textContent=Number.isNaN(date.getTime())?'—':date.toLocaleTimeString();const type=document.createElement('span');type.className='level';type.textContent=level;const body=document.createElement('span');body.textContent=message;row.append(time,type,body);container.prepend(row);while(container.children.length>200)container.lastElementChild.remove();const limit=O.$('#log-limit');if(limit)limit.textContent=container.children.length>=200?'Показано последние 200 записей.':'';};
  async function loadDiagnostics(){const [error,logs]=await Promise.all([O.request('/diagnostics/error').catch(()=>null),O.request('/diagnostics/log').catch(()=>null)]);if(error)O.text('#diag-error',(error.code===0||error.status_error===0)?'Активных ошибок нет.':JSON.stringify(error,null,2));const rows=Array.isArray(logs)?logs:(logs?.entries||logs?.log||[]);rows.slice(-100).forEach((entry)=>O.addLog(entry.level||'INFO',entry.message||'',entry.timestamp));}
  async function clearError(){if(!await O.confirm('Сбросить ошибку?','Сначала устраните причину неисправности.','Сбросить',true))return;try{await O.request('/diagnostics/error/clear',{method:'POST'});O.text('#diag-error','Ошибка сброшена.');O.refreshStatus();}catch(error){O.toast(error.message,'error');}}
  async function clearLog(){if(!await O.confirm('Очистить журнал?','Записи текущего запуска будут удалены.','Очистить',true))return;try{await O.request('/diagnostics/log',{method:'DELETE'});O.$('#log-container').replaceChildren();}catch(error){O.toast(error.message,'error');}}
  function reagentDialog(data){O.text('#reagent-modal-msg',data.message||`Реагент закончился на шаге ${data.step||'—'}: ${data.step_name||'название не получено'}.`);O.$('#reagent-modal').hidden=false;O.$('#btn-reagent-replace').focus();}
  async function reagentReplace(){O.$('#reagent-modal').hidden=true;try{await O.request('/programs/stain/reagent-replaced',{method:'POST'});O.toast('Продолжение цикла подтверждено.','ok');}catch(error){O.toast(error.message,'error');}}
  async function reagentCancel(){if(!await O.confirm('Отменить цикл окраски?','Цикл завершится без продолжения.','Отменить цикл',true))return;O.$('#reagent-modal').hidden=true;try{await O.request('/programs/stain/reagent-cancel',{method:'POST'});O.hideProgress();}catch(error){O.toast(error.message,'error');}}
  O.handleEvent=(type,data)=>{if(type==='sensors')renderSensors(data);else if(type==='reagent_low')reagentDialog(data);else if(type==='error'){O.addLog('ERROR',data.message||String(data.code||'Ошибка'));O.text('#diag-error',JSON.stringify(data,null,2));}else if(type==='log')O.addLog(data.level||'INFO',data.message||'');};

  // ─── Sequence Builder Logic (Dedicated Tab) ──────────────────────────────────

  const COMMAND_OPTIONS = [
    { cmd: 120, name: '120 · Калибровка клапана' },
    { cmd: 100, name: '100 · Калибровка селектора 1' },
    { cmd: 105, name: '105 · Калибровка селектора 2' },
    { cmd: 200, name: '200 · Хомирование ротора' },
    { cmd: 130, name: '130 · Слив промежуточной ёмкости' },
    { cmd: 140, name: '140 · Загрузка материала' },
    { cmd: 160, name: '160 · Осаждение (300с)' },
    { cmd: 150, name: '150 · Цикл окраски' },
    { cmd: 170, name: '170 · Промывка системы' },
    { cmd: 222, name: '222 · Сохранение в EEPROM' },
    { cmd: 999, name: '999 · Аварийный сброс / стоп' }
  ];

  const PRESETS = {
    sys_check: [
      { name: 'Калибровка клапана', cmd: 120, zone: 3, delay_sec: 0, timeout_sec: 300 },
      { name: 'Калибровка селектора 1', cmd: 100, zone: 3, delay_sec: 0, timeout_sec: 300 },
      { name: 'Хомирование ротора', cmd: 200, zone: 3, delay_sec: 0, timeout_sec: 300 },
      { name: 'Слив', cmd: 130, zone: 3, delay_sec: 0, timeout_sec: 300 }
    ],
    stain_cycle: [
      { name: 'Калибровка клапана', cmd: 120, zone: 3, delay_sec: 0, timeout_sec: 300 },
      { name: 'Загрузка образцов', cmd: 140, zone: 3, delay_sec: 0, timeout_sec: 300 },
      { name: 'Осаждение', cmd: 160, zone: 3, delay_sec: 2, timeout_sec: 600 },
      { name: 'Цикл окраски', cmd: 150, zone: 3, delay_sec: 0, timeout_sec: 600 },
      { name: 'Промывка системы', cmd: 170, zone: 3, delay_sec: 0, timeout_sec: 300 }
    ],
    wash_system: [
      { name: 'Промывка (Хлорка)', cmd: 170, zone: 3, delay_sec: 5, timeout_sec: 300 },
      { name: 'Промывка (Спирт)', cmd: 170, zone: 3, delay_sec: 5, timeout_sec: 300 },
      { name: 'Промывка (Вода)', cmd: 170, zone: 3, delay_sec: 0, timeout_sec: 300 },
      { name: 'Финишный слив', cmd: 130, zone: 3, delay_sec: 0, timeout_sec: 300 }
    ],
    calib_full: [
      { name: 'Калибровка клапана', cmd: 120, zone: 3, delay_sec: 1, timeout_sec: 300 },
      { name: 'Калибровка селектора 1', cmd: 100, zone: 3, delay_sec: 1, timeout_sec: 300 },
      { name: 'Калибровка селектора 2', cmd: 105, zone: 3, delay_sec: 1, timeout_sec: 300 },
      { name: 'Хомирование ротора', cmd: 200, zone: 3, delay_sec: 0, timeout_sec: 300 }
    ]
  };

  O.state.customSequence = JSON.parse(localStorage.getItem('onepap_custom_seq') || 'null') || PRESETS.sys_check.map(s => ({ ...s }));

  function renderSequenceCards() {
    const container = O.$('#seq-step-cards-container');
    if (!container) return;
    container.replaceChildren();

    let totalEstSec = 0;

    O.state.customSequence.forEach((step, idx) => {
      totalEstSec += (step.delay_sec || 0) + 15; // base est ~15s per step

      const card = document.createElement('div');
      card.className = 'seq-step-card';
      card.style.cssText = 'background: var(--surface-2); border: 1px solid var(--border-strong); border-radius: 12px; padding: 16px; display: grid; gap: 14px; box-shadow: 0 4px 14px rgba(0,0,0,0.15); transition: all 0.18s ease;';

      // Header row
      const header = document.createElement('div');
      header.style.cssText = 'display: flex; align-items: center; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--border); padding-bottom: 10px;';

      const headerLeft = document.createElement('div');
      headerLeft.style.cssText = 'display: flex; align-items: center; gap: 10px; flex: 1;';

      const badge = document.createElement('span');
      badge.style.cssText = 'display: grid; place-items: center; width: 28px; height: 28px; border-radius: 50%; background: var(--surface-3); border: 1px solid var(--accent-strong); color: var(--accent-blue); font: 700 12px var(--mono);';
      badge.textContent = String(idx + 1);

      const inpName = document.createElement('input');
      inpName.type = 'text';
      inpName.className = 'inp';
      inpName.style.cssText = 'flex: 1; font-weight: 700; font-size: 14px; height: 36px; background: var(--bg);';
      inpName.value = step.name || `Шаг ${idx + 1}`;
      inpName.placeholder = 'Название шага...';
      inpName.oninput = () => { step.name = inpName.value.trim(); saveSequenceLocally(); };

      headerLeft.append(badge, inpName);

      // Actions
      const actions = document.createElement('div');
      actions.style.cssText = 'display: flex; align-items: center; gap: 6px;';

      const btnUp = createActionButton('▲ Вверх', 'Вверх', idx === 0, () => moveSequenceStep(idx, -1));
      const btnDown = createActionButton('▼ Вниз', 'Вниз', idx === O.state.customSequence.length - 1, () => moveSequenceStep(idx, 1));
      const btnDup = createActionButton('📋 Клон', 'Дублировать шаг', false, () => duplicateSequenceStep(idx));
      const btnDel = createActionButton('🗑️ Удалить', 'Удалить шаг', false, () => removeSequenceStep(idx), 'var(--danger)');

      actions.append(btnUp, btnDown, btnDup, btnDel);
      header.append(headerLeft, actions);

      // Controls Grid
      const bodyGrid = document.createElement('div');
      bodyGrid.style.cssText = 'display: grid; grid-template-columns: minmax(220px, 1.2fr) minmax(200px, 1.1fr) minmax(130px, 0.8fr) minmax(130px, 0.8fr); gap: 14px; align-items: end;';

      // 1. Modbus Command
      const fieldCmd = document.createElement('label');
      fieldCmd.style.cssText = 'display: grid; gap: 4px; font-size: 11px; font-weight: 700; text-transform: uppercase; color: var(--text-3);';
      fieldCmd.append('Команда Modbus');

      const selCmd = document.createElement('select');
      selCmd.className = 'inp';
      selCmd.style.cssText = 'width: 100%; height: 38px; font-weight: 600;';
      COMMAND_OPTIONS.forEach(opt => {
        const o = document.createElement('option');
        o.value = opt.cmd;
        o.textContent = opt.name;
        if (Number(opt.cmd) === Number(step.cmd)) o.selected = true;
        selCmd.append(o);
      });
      selCmd.onchange = () => { step.cmd = Number(selCmd.value); saveSequenceLocally(); };
      fieldCmd.append(selCmd);

      // 2. Zone Select Pills
      const fieldZone = document.createElement('label');
      fieldZone.style.cssText = 'display: grid; gap: 4px; font-size: 11px; font-weight: 700; text-transform: uppercase; color: var(--text-3);';
      fieldZone.append('Целевая зона');

      const zoneGroup = document.createElement('div');
      zoneGroup.className = 'segmented';
      zoneGroup.style.cssText = 'margin: 0; height: 38px;';
      [
        { val: 0, txt: 'Текущая' },
        { val: 1, txt: 'Зона 1' },
        { val: 2, txt: 'Зона 2' },
        { val: 3, txt: 'Зона 3' }
      ].forEach(z => {
        const btnZ = document.createElement('button');
        btnZ.type = 'button';
        btnZ.className = `segment ${Number(step.zone || 0) === z.val ? 'active' : ''}`;
        btnZ.style.cssText = 'padding: 6px 10px; font-size: 12px;';
        btnZ.textContent = z.txt;
        btnZ.onclick = () => {
          step.zone = z.val;
          saveSequenceLocally();
          renderSequenceCards();
        };
        zoneGroup.append(btnZ);
      });
      fieldZone.append(zoneGroup);

      // 3. Delay Sec
      const fieldDelay = document.createElement('label');
      fieldDelay.style.cssText = 'display: grid; gap: 4px; font-size: 11px; font-weight: 700; text-transform: uppercase; color: var(--text-3);';
      fieldDelay.append('Задержка (сек)');

      const inpDelay = document.createElement('input');
      inpDelay.type = 'number';
      inpDelay.className = 'inp inp-number';
      inpDelay.style.cssText = 'width: 100%; height: 38px; text-align: center; font-family: var(--mono);';
      inpDelay.min = '0';
      inpDelay.value = step.delay_sec ?? 0;
      inpDelay.oninput = () => { step.delay_sec = Math.max(0, Number(inpDelay.value)); saveSequenceLocally(); renderSequenceCards(); };
      fieldDelay.append(inpDelay);

      // 4. Timeout Sec
      const fieldTimeout = document.createElement('label');
      fieldTimeout.style.cssText = 'display: grid; gap: 4px; font-size: 11px; font-weight: 700; text-transform: uppercase; color: var(--text-3);';
      fieldTimeout.append('Таймаут (сек)');

      const inpTimeout = document.createElement('input');
      inpTimeout.type = 'number';
      inpTimeout.className = 'inp inp-number';
      inpTimeout.style.cssText = 'width: 100%; height: 38px; text-align: center; font-family: var(--mono);';
      inpTimeout.min = '10';
      inpTimeout.value = step.timeout_sec ?? 300;
      inpTimeout.oninput = () => { step.timeout_sec = Math.max(10, Number(inpTimeout.value)); saveSequenceLocally(); };
      fieldTimeout.append(inpTimeout);

      bodyGrid.append(fieldCmd, fieldZone, fieldDelay, fieldTimeout);
      card.append(header, bodyGrid);
      container.append(card);
    });

    // Update Summary Header
    O.text('#seq-total-steps', `Всего шагов: ${O.state.customSequence.length}`);
    const minEst = Math.ceil(totalEstSec / 60);
    O.text('#seq-est-time', `Ориентировочное время: ~${minEst} мин`);

    const runBtn = O.$('#btn-seq-run');
    if (runBtn) {
      runBtn.disabled = !O.state.connected || O.state.busy || O.state.customSequence.length === 0;
    }
  }

  function createActionButton(text, title, disabled, onClick, color) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'btn btn-secondary';
    btn.style.cssText = `height: 32px; padding: 4px 8px; font-size: 11px; ${color ? `color: ${color}; border-color: rgba(239,68,68,0.3);` : ''}`;
    btn.textContent = text;
    btn.title = title;
    btn.disabled = disabled;
    btn.onclick = onClick;
    return btn;
  }

  function saveSequenceLocally() {
    localStorage.setItem('onepap_custom_seq', JSON.stringify(O.state.customSequence));
  }

  function addSequenceStep() {
    O.state.customSequence.push({
      name: `Шаг ${O.state.customSequence.length + 1}`,
      cmd: 120,
      zone: 3,
      delay_sec: 0,
      timeout_sec: 300
    });
    saveSequenceLocally();
    renderSequenceCards();
  }

  function duplicateSequenceStep(index) {
    const step = O.state.customSequence[index];
    if (!step) return;
    const cloned = { ...step, name: `${step.name} (копия)` };
    O.state.customSequence.splice(index + 1, 0, cloned);
    saveSequenceLocally();
    renderSequenceCards();
  }

  function removeSequenceStep(index) {
    O.state.customSequence.splice(index, 1);
    saveSequenceLocally();
    renderSequenceCards();
  }

  function moveSequenceStep(index, direction) {
    const newIdx = index + direction;
    if (newIdx < 0 || newIdx >= O.state.customSequence.length) return;
    const temp = O.state.customSequence[index];
    O.state.customSequence[index] = O.state.customSequence[newIdx];
    O.state.customSequence[newIdx] = temp;
    saveSequenceLocally();
    renderSequenceCards();
  }

  function loadPreset(presetKey) {
    if (!PRESETS[presetKey]) return;
    O.state.customSequence = PRESETS[presetKey].map(s => ({ ...s }));
    saveSequenceLocally();
    renderSequenceCards();
    O.toast('Шаблон сценария успешно загружен', 'ok');
  }

  async function runCustomSequence() {
    if (O.state.customSequence.length === 0) {
      O.toast('Добавьте хотя бы один шаг в сценарий', 'error');
      return;
    }
    if (!await O.confirm('Запустить пользовательский сценарий?', `Будет выполнено шагов: ${O.state.customSequence.length}. Убедитесь в готовности системы.`,'Запустить')) return;

    O.setBusy(true, 'Запуск сценария');
    try {
      await O.request('/programs/sequence/execute', {
        method: 'POST',
        body: { name: 'Пользовательский сценарий', steps: O.state.customSequence },
        timeout: 600000
      });
      O.toast('Сценарий принят контроллером.', 'ok');
      O.$('#progress-wrap').hidden = false;
    } catch (error) {
      O.toast(`Ошибка запуска сценария: ${error.message}`, 'error');
    } finally {
      O.setBusy(false);
    }
  }

  O.initOperations=async()=>{
    sensorPlaceholders();loadDiagnostics();
    O.$$('.prog-btn').forEach((button)=>button.onclick=()=>runProgram(button.dataset.prog));O.$('#btn-emergency').onclick=emergency;
    O.$('#sel-btn-1').onclick=()=>selectSelector(1);O.$('#sel-btn-2').onclick=()=>selectSelector(2);O.$$('.sel-hole-btn').forEach((button)=>button.onclick=()=>moveSelector(Number(button.dataset.hole)));O.$('#btn-calibrate-sel').onclick=calibrate;O.$('#btn-reset-plc').onclick=resetPLC;
    O.$('#btn-read-sensors').onclick=readSensors;O.$('#btn-clear-error').onclick=clearError;O.$('#btn-clear-log').onclick=clearLog;O.$('#btn-reagent-replace').onclick=reagentReplace;O.$('#btn-reagent-cancel').onclick=reagentCancel;

    // Sequence builder event bindings
    renderSequenceCards();
    O.$$('.seq-preset-btn').forEach((button) => {
      button.onclick = () => loadPreset(button.dataset.preset);
    });
    const btnAdd = O.$('#btn-seq-add');
    if (btnAdd) btnAdd.onclick = addSequenceStep;
    const btnReset = O.$('#btn-seq-reset');
    if (btnReset) btnReset.onclick = () => loadPreset('sys_check');
    const btnRun = O.$('#btn-seq-run');
    if (btnRun) btnRun.onclick = runCustomSequence;
  };
})();

