'use strict';

(() => {
  const O = window.ONE24;
  const STEP_DEFAULTS = [
    ['Спирт 96% (фиксация)',10,50],['Гематоксилин Харриса',120,30],['Вода дистиллированная',10,80],
    ['Вода дистиллированная',50,80],['Спирт 87%',10,60],['OG-6',10,40],['Спирт 96%',10,60],
    ['EA-50',120,40],['Спирт 87%',10,60],['Спирт 87%',10,60],['Спирт 96%',10,60]
  ];
  const HOLES = ['Исходное положение','Воздух','EA-50','Воздух','Вода дистиллированная','Воздух','Спирт 87%','Воздух','Спирт 96%','Воздух','OG-6','Воздух','Гематоксилин Харриса','Воздух','Хлорка'];
  const COORDS1 = [0,700,1400,2771,4142,5513,6884,8256,9627,10998,12369,13741,15112,16483,17853];
  const COORDS2 = [0,600,1200,2571,3942,5313,6684,8056,9427,10798,12169,13541,14912,16283,17653];
  Object.assign(O.state, { steps: [], valves1: [], valves2: [], stepsFromDevice: false, valvesFromDevice: false,
    detection: { reagent_empty: false, reagent_empty_delta: 10, original: 10, dirty: false } });

  const defaultSteps = () => STEP_DEFAULTS.map(([name,t,v],i) => ({ id:i+1,name,exposure_time:t,fill_volume:v,originalT:t,originalV:v,dirty:false,error:'' }));
  const defaultValves = (selector) => HOLES.map((name,hole) => {
    const coords = selector === 1 ? COORDS1 : COORDS2;
    return { selector,hole,name,coord:coords[hole],original:coords[hole],dirty:false,error:'' };
  });
  const normalizeSteps = (items) => (items || []).map((item,i) => {
    const t=Number(item.exposure_time??STEP_DEFAULTS[i][1]), v=Number(item.fill_volume??STEP_DEFAULTS[i][2]);
    return { id:Number(item.id??i+1),name:String(item.name||STEP_DEFAULTS[i][0]),exposure_time:t,fill_volume:v,originalT:t,originalV:v,dirty:false,error:'' };
  });
  const normalizeValves = (items,selector) => (items || []).map((item,i) => {
    const coords = selector === 1 ? COORDS1 : COORDS2;
    const coord=Number(item.coord??coords[i]);
    return { selector,hole:Number(item.hole??i),name:String(item.name||HOLES[i]),coord,original:coord,dirty:false,error:'' };
  });
  const settingsCount = () => O.state.steps.filter((item) => item.dirty).length + (O.state.detection.dirty ? 1 : 0);
  const valvesCount = () => [...O.state.valves1,...O.state.valves2].filter((item) => item.dirty).length;

  function updateCounters() {
    const settings=settingsCount(), valves=valvesCount();
    O.text('#settings-dirty-count', settings ? `Изменено: ${settings}` : 'Изменений нет');
    O.text('#valves-dirty-count', valves ? `Изменено: ${valves}` : 'Изменений нет');
    O.$('#settings-dirty-count').classList.toggle('has-changes', settings>0);
    O.$('#valves-dirty-count').classList.toggle('has-changes', valves>0);
    O.updateAvailability();
  }
  O.updateSettingsAvailability = () => {
    O.$('#btn-write-all').disabled = !O.state.connected || O.state.busy || settingsCount()===0;
    O.$('#btn-write-valves').disabled = !O.state.connected || O.state.busy || valvesCount()===0;
  };
  function numberInput(value,min,max,label) {
    const input=document.createElement('input'); input.type='number'; input.className='inp'; input.value=String(value);
    input.min=String(min); input.max=String(max); input.inputMode='numeric'; input.setAttribute('aria-label',label); return input;
  }
  function cell(child) { const td=document.createElement('td'); td.append(child); return td; }

  function renderSteps() {
    const body=O.$('#steps-tbody'); body.replaceChildren();
    O.state.steps.forEach((step,index) => {
      const row=document.createElement('tr'); row.classList.toggle('is-dirty',step.dirty); row.classList.toggle('is-error',Boolean(step.error));
      const number=document.createElement('td'); number.className='mono'; number.textContent=String(step.id);
      const name=document.createElement('td'); name.textContent=step.name;
      const time=numberInput(step.exposure_time,1,600,`Шаг ${step.id}: экспозиция`);
      const volume=numberInput(step.fill_volume,1,6000,`Шаг ${step.id}: налив`);
      time.classList.toggle('dirty',step.exposure_time!==step.originalT); volume.classList.toggle('dirty',step.fill_volume!==step.originalV);
      time.addEventListener('input',() => changeStep(index,'exposure_time',time));
      volume.addEventListener('input',() => changeStep(index,'fill_volume',volume));
      const status=document.createElement('td'); status.className=`row-state${step.dirty?' is-dirty':step.error?' is-error':' is-ok'}`;
      status.textContent=step.error || (step.dirty?'Изменено':O.state.stepsFromDevice?'Прочитано':'По умолчанию');
      row.append(number,name,cell(time),cell(volume),status); body.append(row);
    });
  }
  function changeStep(index,key,input) {
    const step=O.state.steps[index], value=Number(input.value), max=key==='exposure_time'?600:6000;
    step[key]=value; step.error=Number.isInteger(value)&&value>=1&&value<=max?'':`Допустимо 1–${max}`;
    step.dirty=step.exposure_time!==step.originalT||step.fill_volume!==step.originalV;
    renderSteps(); updateCounters();
  }
  function changeDelta() {
    const value=Number(O.$('#det-delta').value); O.state.detection.reagent_empty_delta=value;
    O.state.detection.dirty=value!==O.state.detection.original; O.$('#det-delta').classList.toggle('dirty',O.state.detection.dirty); updateCounters();
  }
  async function readSettings() {
    O.setBusy(true,'Чтение настроек'); O.message('#settings-log','Чтение 11 шагов…');
    try {
      const data=await O.request('/settings/read-all',{method:'POST',timeout:120000});
      O.state.steps=normalizeSteps(data.steps); const delta=Number(data.detection?.reagent_empty_delta??10);
      O.state.detection={reagent_empty:Boolean(data.detection?.reagent_empty),reagent_empty_delta:delta,original:delta,dirty:false};
      O.state.stepsFromDevice=true; O.$('#det-delta').value=String(delta); O.$('#det-delta').classList.remove('dirty');
      O.label('#settings-state','Прочитано','ok'); O.label('#det-status',O.state.detection.reagent_empty?'Реагент пуст':'Реагент в норме',O.state.detection.reagent_empty?'error':'ok');
      O.text('#settings-source',`Данные прочитаны ${new Date().toLocaleTimeString()}.`); O.text('#data-freshness',`Настройки: ${new Date().toLocaleTimeString()}`);
      O.message('#settings-log','Настройки прочитаны.','ok'); renderSteps(); updateCounters();
    } catch(error) { O.message('#settings-log',error.message,'error'); O.toast(`Чтение не выполнено: ${error.message}`,'error'); }
    finally { O.setBusy(false); }
  }
  O.writeSettings = async () => {
    const changed=O.state.steps.filter((item)=>item.dirty), invalid=changed.find((item)=>item.error), count=settingsCount();
    if(invalid){O.toast(`Исправьте шаг ${invalid.id}: ${invalid.error}`,'error');return;} if(!count)return;
    if(!await O.confirm('Записать изменения?',`В контроллер будут записаны только изменённые значения: ${count}.`,'Записать'))return;
    O.setBusy(true,'Запись настроек'); O.message('#settings-log','Подготовка изменений…');
    try {
      for(const step of changed) await O.request(`/settings/steps/${step.id}`,{method:'PUT',body:{exposure_time:step.exposure_time,fill_volume:step.fill_volume}});
      if(O.state.detection.dirty) await O.request('/settings/detection',{method:'PUT',body:{delta:O.state.detection.reagent_empty_delta}});
      const data=await O.request('/settings/write-all',{method:'POST',timeout:120000});
      changed.forEach((step)=>{step.originalT=step.exposure_time;step.originalV=step.fill_volume;step.dirty=false;});
      O.state.detection.original=O.state.detection.reagent_empty_delta; O.state.detection.dirty=false; O.$('#det-delta').classList.remove('dirty');
      O.message('#settings-log',`Записано параметров: ${data.written}.`,'ok'); O.toast('Изменения записаны и подтверждены контроллером.','ok'); renderSteps(); updateCounters();
    } catch(error) { O.message('#settings-log',`Частичная запись возможна: ${error.message}`,'error'); O.toast(`Запись не завершена: ${error.message}`,'error'); }
    finally { O.setBusy(false); }
  };

  function renderValves() { renderValveBody('#valves1-tbody',O.state.valves1); renderValveBody('#valves2-tbody',O.state.valves2); }
  function renderValveBody(selector,items) {
    const body=O.$(selector); body.replaceChildren();
    items.forEach((item,index) => {
      const row=document.createElement('tr'); row.classList.toggle('is-dirty',item.dirty); row.classList.toggle('is-error',Boolean(item.error));
      const number=document.createElement('td'); number.className='mono'; number.textContent=String(item.hole);
      const name=document.createElement('td'); name.textContent=item.name;
      const input=numberInput(item.coord,0,65535,`Селектор ${item.selector}, позиция ${item.hole}`); input.classList.toggle('dirty',item.dirty);
      input.addEventListener('input',()=>{const value=Number(input.value);item.coord=value;item.error=Number.isInteger(value)&&value>=0&&value<=65535?'':'Допустимо 0–65535';item.dirty=value!==item.original;renderValves();updateCounters();});
      const status=document.createElement('td'); status.className=`row-state${item.dirty?' is-dirty':item.error?' is-error':' is-ok'}`;
      status.textContent=item.error || (item.dirty?'Изменено':O.state.valvesFromDevice?'Прочитано':'По умолчанию');
      row.append(number,name,cell(input),status); body.append(row);
    });
  }
  async function readValves() {
    O.setBusy(true,'Чтение координат'); O.message('#valves-log','Чтение 30 положений…');
    try { const data=await O.request('/settings/valves/read-all',{method:'POST',timeout:180000}); O.state.valves1=normalizeValves(data.selector1,1); O.state.valves2=normalizeValves(data.selector2,2); O.state.valvesFromDevice=true; O.message('#valves-log','Координаты прочитаны.','ok'); renderValves(); updateCounters(); }
    catch(error){O.message('#valves-log',error.message,'error');O.toast(error.message,'error');} finally{O.setBusy(false);}
  }
  O.writeValves = async () => {
    const changed=[...O.state.valves1,...O.state.valves2].filter((item)=>item.dirty), invalid=changed.find((item)=>item.error);
    if(invalid){O.toast(`Исправьте селектор ${invalid.selector}, позиция ${invalid.hole}.`,'error');return;} if(!changed.length)return;
    if(!await O.confirm('Записать координаты?',`Будут записаны ${changed.length} изменённых положений. Не перемещайте механизм вручную.`,'Записать'))return;
    O.setBusy(true,'Запись координат'); O.message('#valves-log','Передача изменений…');
    try { for(const item of changed) await O.request(`/settings/valves/${item.selector}/${item.hole}`,{method:'PUT',body:{coord:item.coord}}); const data=await O.request('/settings/valves/write-all',{method:'POST',timeout:180000}); changed.forEach((item)=>{item.original=item.coord;item.dirty=false;}); O.message('#valves-log',`Записано положений: ${data.written}.`,'ok');O.toast('Координаты записаны.','ok');renderValves();updateCounters(); }
    catch(error){O.message('#valves-log',`Частичная запись возможна: ${error.message}`,'error');O.toast(error.message,'error');} finally{O.setBusy(false);}
  };

  O.initSettings = async () => {
    const [settings,valves]=await Promise.all([O.request('/settings/steps').catch(()=>null),O.request('/settings/valves').catch(()=>null)]);
    O.state.steps=settings?.steps?normalizeSteps(settings.steps):defaultSteps();
    if(settings?.detection){const delta=Number(settings.detection.reagent_empty_delta??10);O.state.detection={reagent_empty:Boolean(settings.detection.reagent_empty),reagent_empty_delta:delta,original:delta,dirty:false};O.$('#det-delta').value=String(delta);}
    O.state.valves1=valves?.selector1?normalizeValves(valves.selector1,1):defaultValves(1); O.state.valves2=valves?.selector2?normalizeValves(valves.selector2,2):defaultValves(2);
    renderSteps();renderValves();updateCounters();
    O.$('#btn-read-all').onclick=readSettings;O.$('#btn-write-all').onclick=O.writeSettings;O.$('#det-delta').oninput=changeDelta;
    O.$('#btn-read-valves').onclick=readValves;O.$('#btn-write-valves').onclick=O.writeValves;
  };
})();
