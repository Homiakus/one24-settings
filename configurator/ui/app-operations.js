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

  function sensorPlaceholders(){const grid=O.$('#sensor-grid');grid.replaceChildren();SENSOR_NAMES.forEach((name)=>{const item=document.createElement('div');item.className='sensor unknown';const title=document.createElement('span');title.textContent=name;const value=document.createElement('strong');value.textContent='—';item.append(title,value);grid.append(item);});}
  function renderSensors(data){const grid=O.$('#sensor-grid');grid.replaceChildren();(data.inputs||[]).forEach((value,index)=>{const item=document.createElement('div');item.className=`sensor ${value?'on':'off'}`;const title=document.createElement('span');title.textContent=SENSOR_NAMES[index]||`Вход ${index}`;const output=document.createElement('strong');output.textContent=value?'1 · активен':'0 · неактивен';item.append(title,output);grid.append(item);});if(data.mcu_temp!=null)O.text('#s-mcu',Number(data.mcu_temp).toFixed(1));if(data.ntc_temp!=null)O.text('#s-ntc',Number(data.ntc_temp).toFixed(1));if(data.hx711_weight!=null)O.text('#s-hx711',data.hx711_weight);}
  async function readSensors(){try{renderSensors(await O.request('/testing/sensors'));O.text('#data-freshness',`Датчики: ${new Date().toLocaleTimeString()}`);}catch(error){O.toast(error.message,'error');}}

  O.addLog=(level='INFO',message='',timestamp)=>{const container=O.$('#log-container'),row=document.createElement('div');row.className=`log-entry ${level}`;const time=document.createElement('span');time.className='time';const date=timestamp?new Date(timestamp):new Date();time.textContent=Number.isNaN(date.getTime())?'—':date.toLocaleTimeString();const type=document.createElement('span');type.className='level';type.textContent=level;const body=document.createElement('span');body.textContent=message;row.append(time,type,body);container.prepend(row);while(container.children.length>100)container.lastElementChild.remove();};
  async function loadDiagnostics(){const [error,logs]=await Promise.all([O.request('/diagnostics/error').catch(()=>null),O.request('/diagnostics/log').catch(()=>null)]);if(error)O.text('#diag-error',(error.code===0||error.status_error===0)?'Активных ошибок нет.':JSON.stringify(error,null,2));const rows=Array.isArray(logs)?logs:(logs?.entries||logs?.log||[]);rows.slice(-100).forEach((entry)=>O.addLog(entry.level||'INFO',entry.message||'',entry.timestamp));}
  async function clearError(){if(!await O.confirm('Сбросить ошибку?','Сначала устраните причину неисправности.','Сбросить',true))return;try{await O.request('/diagnostics/error/clear',{method:'POST'});O.text('#diag-error','Ошибка сброшена.');O.refreshStatus();}catch(error){O.toast(error.message,'error');}}
  async function clearLog(){if(!await O.confirm('Очистить журнал?','Записи текущего запуска будут удалены.','Очистить',true))return;try{await O.request('/diagnostics/log',{method:'DELETE'});O.$('#log-container').replaceChildren();}catch(error){O.toast(error.message,'error');}}
  function reagentDialog(data){O.text('#reagent-modal-msg',data.message||`Реагент закончился на шаге ${data.step||'—'}: ${data.step_name||'название не получено'}.`);O.$('#reagent-modal').hidden=false;O.$('#btn-reagent-replace').focus();}
  async function reagentReplace(){O.$('#reagent-modal').hidden=true;try{await O.request('/programs/stain/reagent-replaced',{method:'POST'});O.toast('Продолжение цикла подтверждено.','ok');}catch(error){O.toast(error.message,'error');}}
  async function reagentCancel(){if(!await O.confirm('Отменить цикл окраски?','Цикл завершится без продолжения.','Отменить цикл',true))return;O.$('#reagent-modal').hidden=true;try{await O.request('/programs/stain/reagent-cancel',{method:'POST'});O.hideProgress();}catch(error){O.toast(error.message,'error');}}
  O.handleEvent=(type,data)=>{if(type==='sensors')renderSensors(data);else if(type==='reagent_low')reagentDialog(data);else if(type==='error'){O.addLog('ERROR',data.message||String(data.code||'Ошибка'));O.text('#diag-error',JSON.stringify(data,null,2));}else if(type==='log')O.addLog(data.level||'INFO',data.message||'');};

  O.initOperations=async()=>{
    sensorPlaceholders();loadDiagnostics();
    O.$$('.prog-btn').forEach((button)=>button.onclick=()=>runProgram(button.dataset.prog));O.$('#btn-emergency').onclick=emergency;
    O.$('#sel-btn-1').onclick=()=>selectSelector(1);O.$('#sel-btn-2').onclick=()=>selectSelector(2);O.$$('.sel-hole-btn').forEach((button)=>button.onclick=()=>moveSelector(Number(button.dataset.hole)));O.$('#btn-calibrate-sel').onclick=calibrate;O.$('#btn-reset-plc').onclick=resetPLC;
    O.$('#btn-read-sensors').onclick=readSensors;O.$('#btn-clear-error').onclick=clearError;O.$('#btn-clear-log').onclick=clearLog;O.$('#btn-reagent-replace').onclick=reagentReplace;O.$('#btn-reagent-cancel').onclick=reagentCancel;
  };
})();
