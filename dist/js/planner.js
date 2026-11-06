// View state belongs to the open planner. Persisted rules are accessed through SiteAPI.
const Planner = (() => {
  const names={simple:'Простой',advanced:'Расширенный'};
  const kinds={working:'Рабочий день',off:'Выходной',unplanned:'Не запланировано'};
  const weekdays=['Вс','Пн','Вт','Ср','Чт','Пт','Сб'];
  const asDate=key=>new Date(key+'T12:00');
  let first=dateKey(new Date()),last=first,monthKey=first.slice(0,7),anchor='';
  const act=(title,click)=>button(title,click,'admin-action');
  function field(parent,label,type,value) {
    const wrap=element('label',undefined,'field'),input=element('input');
    input.type=type;input.value=value;wrap.append(element('span',label),input);parent.append(wrap);return input;
  }
  function select(parent,label,items,value) {
    const wrap=element('label',undefined,'field'),control=element('select');
    for(const [key,title] of Object.entries(items)){const opt=element('option',title);opt.value=key;control.append(opt)}
    control.value=value;wrap.append(element('span',label),control);parent.append(wrap);return control;
  }
  function details(parent,title) {
    const d=element('details');d.append(element('summary',title));parent.append(d);return d;
  }
  function confirmView(parent,title,lines,accept,run,cancel='Отмена'){
    parent.querySelector('.plan-confirm')?.remove();
    const box=element('section',undefined,'plan-confirm');box.setAttribute('aria-label',title);
    box.append(element('h4',title),...lines.map(t=>element('p',t)),act(cancel,()=>box.remove()),button(accept,run,'btn'));
    parent.prepend(box);box.tabIndex=-1;box.focus();
  }
  function formatPreview(p) {
    return ['Дат: '+p.total+' · рабочих: '+p.working+' · выходных: '+p.off,
      'Закрытые для записи даты: '+(p.unavailable.join(', ')||'нет'),
      'Вне графика: '+(p.conflicts.map(b=>b.date+' '+b.time).join(', ')||'нет')];
  }
  function render(page,data,simpleForm,save,message) {
    const schedule=data.schedule,mode=schedule.activeMode;
    const switches=element('div',undefined,'segments');
    for(const [key,title] of Object.entries(names)){
      const b=act(title,async()=>{
        if(key===mode)return;
        const start=new Date(),end=new Date(start);end.setDate(end.getDate()+89);
        const a=dateKey(start),z=dateKey(end),p=await SiteAPI.previewScheduleModeChange(key,a,z);
        confirmView(page,'Переключение расписания',[names[mode]+' → '+title,a+' — '+z,'Изменится доступное время для '+p.changed+' дат. Записей вне нового графика: '+p.conflicts+'.'],'Подтвердить переключение',()=>save(()=>SiteAPI.setScheduleMode(key)));
      });b.setAttribute('aria-pressed',key===mode);switches.append(b);
    }
    page.append(switches,element('p',mode==='simple'?'Недельный график повторяется каждую неделю.':'Посетители видят только опубликованные даты.','plan-note'));
    if(mode==='simple'){simpleForm(data);return}
    const previous=AdminDraftStore.keys().filter(key=>key.startsWith('schedule/plan/')).at(-1);
    if(previous){[, ,first,last]=previous.split('/');monthKey=first.slice(0,7)}
    const layout=element('div',undefined,'plan-layout'),left=element('div'),right=element('div',undefined,'plan-editor');
    page.append(layout);layout.append(left,right);
    const bar=element('div',undefined,'plan-bar'),title=element('strong'),grid=element('div',undefined,'plan-grid');
    const turn=n=>{const d=asDate(monthKey+'-01');d.setMonth(d.getMonth()+n);monthKey=dateKey(d).slice(0,7);draw()};
    bar.append(act('‹',()=>turn(-1)),title,act('›',()=>turn(1)),act('Сегодня',()=>{monthKey=dateKey(new Date()).slice(0,7);draw()}));
    bar.firstChild.setAttribute('aria-label','Предыдущий месяц');bar.children[2].setAttribute('aria-label','Следующий месяц');
    left.append(bar,grid);
    const legend=element('div',undefined,'plan-legend');
    for(const [key,label] of Object.entries(kinds))legend.append(element('span',label,key));
    legend.append(element('span','◇ Есть черновик'),element('span','• Есть записи'));left.append(legend);
    const inputs=element('div',undefined,'plan-pair');left.append(inputs);
    const from=field(inputs,'С','date',first),to=field(inputs,'По','date',last);
    function range(a,b){[first,last]=[a,b].sort();from.value=first;to.value=last}
    function manual(){if(!from.value||!to.value)return;range(from.value,to.value);anchor='';monthKey=first.slice(0,7);draw();edit()}
    from.onchange=to.onchange=manual;
    left.append(act('Сбросить выделение',()=>{first=last=anchor=from.value=to.value='';draw();edit()}));
    let dragging='',dragged=false;
    function highlight(){for(const b of grid.querySelectorAll('[data-date]')) b.setAttribute('aria-pressed',!!first&&b.dataset.date>=first&&b.dataset.date<=last)}
    function draw(){
      const d=asDate(monthKey+'-01');title.textContent=d.toLocaleDateString('ru',{month:'long',year:'numeric'});
      grid.replaceChildren(...[1,2,3,4,5,6,0].map(n=>element('span',weekdays[n])));
      for(let i=0;i<(d.getDay()+6)%7;i++)grid.append(element('span'));
      while(dateKey(d).startsWith(monthKey)){
        const key=dateKey(d),draft=schedule.drafts[key],rule=draft||schedule.published[key],kind=rule?.kind||'unplanned';
        const count=data.bookings.filter(b=>b.date===key&&b.status!=='cancelled').length;
        const b=button(d.getDate(),()=>{
          if(dragged){dragged=false;return}
          if(anchor){range(anchor,key);anchor=''}else{range(key,key);anchor=key}
          highlight();edit();
        },kind);
        b.dataset.date=key;b.setAttribute('aria-label',[key,kinds[kind],draft?'Есть черновик':'','Записей '+count].filter(Boolean).join(', '));
        if(draft)b.classList.add('draft');
        if(count)b.append(element('small','• '+count));
        b.onpointerdown=e=>{dragged=false;if(e.pointerType==='mouse'&&e.button===0)dragging=key};
        b.onpointerenter=e=>{if(e.buttons!==1){dragging='';return}if(dragging&&key!==dragging){range(dragging,key);dragged=true;anchor='';highlight()}};
        grid.append(b);d.setDate(d.getDate()+1);
      }
      highlight();
    }
    grid.onpointerup=()=>{dragging='';if(dragged)edit()};
    grid.onpointerleave=()=>{if(dragged)edit();dragging=''};
    function ruleEditor(parent,initial,optional=false){
      const kindsMenu=optional?{skip:'Пропустить день недели',...kinds}:kinds;
      const kind=select(parent,'Состояние',kindsMenu,initial.kind);
      const work=element('div',undefined,'plan-fields'),rows=element('div');parent.append(work);work.append(rows);
      function row(start='',end=''){
        const box=element('div',undefined,'interval');rows.append(box);
        field(box,'Начало','time',start);field(box,'Конец','time',end);box.append(act('×',()=>{box.remove();right.dispatchEvent(new Event('input'))}));box.lastChild.setAttribute('aria-label','Удалить интервал');
      }
      for(const r of initial.intervals||[])row(r.start,r.end);
      if(!initial.intervals?.length&&!initial.formState)row(data.settings.start,data.settings.end);
      work.append(act('+ Добавить интервал',()=>{row();right.dispatchEvent(new Event('input'))}));
      const common=field(work,'Использовать общий шаг ('+data.settings.step+' мин)','checkbox','');
      common.checked=initial.stepOverride==null;
      const step=field(work,'Свой шаг, мин','number',initial.stepOverride||data.settings.step);
      step.min=1;step.max=1440;
      common.onchange=()=>step.parentNode.hidden=common.checked;common.onchange();
      kind.onchange=()=>work.hidden=kind.value!=='working';kind.onchange();
      return ()=>({kind:kind.value,stepOverride:common.checked?null:Number(step.value),intervals:[...rows.children].map(box=>{
        const [start,end]=box.querySelectorAll('input');return {start:start.value,end:end.value};
      })});
    }
    function edit(){
      right.replaceChildren();page.querySelector('.plan-confirm')?.remove();
      if(!first){right.append(element('p','Выберите дату или диапазон.'));return}
      const a=first,b=last,key='schedule/plan/'+a+'/'+b,stored=AdminDraftStore.get(key);
      const base=JSON.stringify([Object.entries(schedule.published).filter(([d])=>d>=a&&d<=b),Object.entries(schedule.drafts).filter(([d])=>d>=a&&d<=b)]);

      right.append(element('h4',a===b?a:a+' — '+b));
      const days=Math.round((Date.parse(b)-Date.parse(a))/86400000)+1;
      const existing=new Set([...Object.keys(schedule.drafts),...Object.keys(schedule.published)].filter(d=>d>=a&&d<=b)).size;
      const type=days>1?select(right,'Применить график', {all:'Одинаковый для всех дат',week:'По дням недели'},'all'):null;
      const forms=element('div');right.append(forms);
      let collect,raw;
      if(type&&stored?.week)type.value='week';
      function prepare(restore=false){
        forms.replaceChildren();
        if(type?.value==='week'){
          const editors={};
          for(const n of [1,2,3,4,5,6,0])editors[n]=ruleEditor(details(forms,weekdays[n]),restore&&stored?.week?.[n]?stored.week[n]:{kind:'skip'},true);
          const fallback=select(forms,'Для пропущенных дней',{'':'Выберите действие',off:'Назначить выходным',unplanned:'Оставить незапланированным'},'');
          if(restore)fallback.value=stored?.fallback||'';
          raw=()=>({week:Object.fromEntries(Object.entries(editors).map(([n,fn])=>[n,{...fn(),formState:true}])),fallback:fallback.value});
          collect=()=>({week:Object.fromEntries(Object.entries(editors).map(([n,fn])=>{let r=fn();if(r.kind==='skip'){if(!fallback.value)throw Error('Выберите действие для пропущенных дней.');r={kind:fallback.value}}return [n,r]}))});
        }else{
          collect=ruleEditor(forms,(restore&&stored?.day)||schedule.drafts[a]||schedule.published[a]||{kind:'working'});
          raw=()=>({day:{...collect(),formState:true}});
        }
      }
      if(type)type.onchange=()=>{prepare();capture()};prepare(true);
      const mark=element('div',undefined,'draft-notice');
      mark.append(element('span',stored&&stored.base!==base?'Расписание изменилось. Проверьте черновик.':'Есть несохранённые изменения'),
        act('Отменить изменения',()=>{if(confirm('Отменить изменения выбранного диапазона?')){AdminDraftStore.clear(key);edit()}}));
      mark.hidden=!stored;right.append(mark);
      function capture(){AdminDraftStore.set(key,{...raw(),base});mark.hidden=false}
      right.oninput=right.onchange=capture;
      function stage(label,task,lines){
        confirmView(page,label,[a+' — '+b,'Выбрано дат: '+days+'. Уже настроено: '+existing+'.',...lines],'Сохранить в черновик',()=>save(async()=>{await task();AdminDraftStore.clear(key)},'Черновик сохранён.'));
      }
      function ruleText(r){return kinds[r.kind]+(r.kind==='working'?' '+r.intervals.map(i=>i.start+'–'+i.end).join(', ')+' · шаг '+(r.stepOverride||data.settings.step)+' мин':'')}
      right.append(act('Сохранить черновик',()=>{
        try{const r=collect(),lines=r.week?Object.entries(r.week).map(([n,v])=>weekdays[n]+' · '+ruleText(v)):[ruleText(r)];
          stage('Предпросмотр черновика',()=>SiteAPI.setDraftScheduleRange(a,b,r),lines);
        }catch(e){message(e.message)}
      }),act('Заполнить по недельному шаблону',()=>stage('Копирование недельного графика',()=>SiteAPI.copySimpleScheduleToDraft(a,b),[data.settings.workDays.map(n=>weekdays[n]).join(', ')+' · '+data.settings.start+'–'+data.settings.end+'; закрытые даты учитываются.'])),
      act('Опубликовать выбранные даты',async()=>{
        const p=await SiteAPI.previewSchedulePublication(a,b);
        if(!p.total){message('В выбранном диапазоне нет черновиков.');return}
        confirmView(page,'Предпросмотр публикации',formatPreview(p),p.conflicts.length?'Опубликовать с предупреждением':'Опубликовать',()=>save(()=>SiteAPI.publishScheduleRange(a,b),'Выбранные даты опубликованы.'),'Вернуться к редактированию');
      }));
      if(a===b){
        right.append(element('h4','Записи на выбранную дату'));
        const records=data.bookings.filter(r=>r.date===a).sort((x,y)=>x.time.localeCompare(y.time));
        if(!records.length)right.append(element('p','Записей нет.'));
        for(const r of records) right.append(element('p',[r.time,r.name,r.durationMinutes+' мин',{pending_confirmation:'Необходимо подтверждение',confirmed:'Подтверждена',completed:'Завершена',cancelled:'Отменена'}[r.status],data.warnings.includes(r.id)?'Вне графика':''].filter(Boolean).join(' · ')));
      }
    }
    draw();edit();
    const history=details(page,'История изменений');
    history.append(element('p','Хранятся последние 30 действий. Более ранние доступны в ваших JSON-экспортах.'));
    for(const h of schedule.history){
      const entry=details(history,new Date(h.at).toLocaleString('ru')+' · '+h.action);
      if(typeof h.before==='string'){entry.append(element('p',names[h.before]+' → '+names[h.after]));continue}
      entry.addEventListener('toggle',()=>{
        if(!entry.open || entry.dataset.loaded)return;entry.dataset.loaded='1';
        for(const d of Object.keys(h.before)){
        const summary=r=>kinds[r.kind]+' '+r.intervals.map(i=>i.start+'–'+i.end).join(', ')+' · шаг '+(r.stepOverride||'общий');
        entry.append(element('p',d+' · '+summary(h.before[d])+' → '+summary(h.after[d])));
      }
      });
      entry.append(act('Восстановить предыдущее состояние',async()=>{
        const p=await SiteAPI.previewRestore(h.id);
        confirmView(page,'Восстановление в черновик',formatPreview(p),'Восстановить',()=>save(()=>SiteAPI.restoreScheduleVersion(h.id),'Версия восстановлена в черновик.'));
      }));
    }
  }
  return {render};
})();
