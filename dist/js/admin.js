(() => {
  if(!APP_CONFIG.demoMode) return;
  const dialog=$('admin'),page=$('adminPage'),trigger=$('adminOpen');
  const sections=['Обзор','Записи','Клиенты','Услуги','Расписание','Содержимое сайта','Оформление','Демо-настройки','Специалисты'];
  const keys=['overview','bookings','clients','services','schedule','content','appearance','demo','specialists'];
  const statuses={pending_confirmation:'Необходимо подтверждение',confirmed:'Подтверждена',completed:'Завершена',cancelled:'Отменена'};
  let current=0,generation=0,restoreTheme,publicScroll=0,switching=false;
  const visitor=$('visitor'),motion=()=>!matchMedia('(prefers-reduced-motion:reduce)').matches&&!document.documentElement.classList.contains('no-motion');
  async function switchView(admin){
    if(switching || !APP_CONFIG.demoMode)return;
    switching=true;const from=admin?visitor:dialog,to=admin?dialog:visitor;
    try{
      if(admin){publicScroll=scrollY;focusSection('');hideHint()}else discardPreview();
      if(motion())await from.animate([{opacity:1},{opacity:0}],{duration:140}).finished;
      from.hidden=true;to.hidden=false;
      if(admin)await show(0);else trigger.focus({preventScroll:true});
      window.scrollTo({top:admin?0:publicScroll,behavior:'instant'});
      if(motion())await to.animate([{opacity:0,transform:'translateY(5px)'},{opacity:1,transform:'none'}],{duration:255,easing:'ease-out'}).finished;
    }finally{switching=false}
  }
  trigger.hidden=false;trigger.onclick=()=>switchView(true);$('adminClose').onclick=()=>switchView(false);
  function updateMarks(){
    [...$('adminNav').children].forEach((b,i)=>b.classList.toggle('unsaved',AdminDraftStore.keys().some(k=>k.split('/')[0]===keys[i])));
  }
  window.addEventListener('admin-drafts',updateMarks);
  function bindDraft(form,key){
    form.dataset.draftKey=key;
    const inputs=[...form.querySelectorAll('input,textarea,select')].filter(x=>x.type!=='file');
    const values=()=>inputs.map(x=>x.type==='checkbox'?x.checked:x.value);
    const base=JSON.stringify(values()),draft=AdminDraftStore.get(key);
    const bar=element('div',undefined,'draft-notice'),note=element('span','Есть несохранённые изменения');
    const discard=button('Отменить изменения',()=>{if(confirm('Отменить несохранённые изменения этой формы?')){AdminDraftStore.clear(key);show(current)}},'admin-action');
    bar.append(note,discard);form.append(bar);bar.hidden=!draft;
    if(draft){
      if(draft.base!==base)note.textContent='Исходные данные изменились. Проверьте черновик перед сохранением.';
      inputs.forEach((x,i)=>{if(draft.values[i]!==undefined){if(x.type==='checkbox')x.checked=draft.values[i];else x.value=draft.values[i]}});
    }
    function capture(){
      const v=values();if(JSON.stringify(v)===base)AdminDraftStore.clear(key);else AdminDraftStore.set(key,{base,values:v});
      bar.hidden=!AdminDraftStore.hasChanges(key);
    }
    form.addEventListener('input',capture);form.addEventListener('change',capture);
  }
  function discardPreview(){if(restoreTheme){applyTheme(restoreTheme);restoreTheme=null}}
  const message=value=>{$('adminMsg').textContent=value};
  async function save(operation,notice='Сохранено') {
    try {await operation();await show(current);message(notice)} catch(e){message(e.message)}
  }
  function action(label,task,question,tone='') {
    return button(label,()=>{if(!question || confirm(question)) save(task)},'admin-action '+tone);
  }
  function inputField(form,label,name,type,value,options={}) {
    const wrapper=element('label',undefined,'field');
    const control=element(type==='textarea'?'textarea':type==='select'?'select':'input');
    if(control.tagName==='INPUT') control.type=type;
    control.name=name;
    if(type==='select') for(const [val,title] of options.choices) {
      const opt=element('option',title);opt.value=val;control.append(opt);
    }
    if(type==='checkbox') control.checked=Boolean(value);else control.value=value;
    if(['textarea','text','number','time','select'].includes(type)) control.required=options.required!==false;
    for(const attr of ['min','max','step','maxLength']) if(options[attr]!==undefined) control[attr]=options[attr];
    wrapper.append(element('span',label),control);form.append(wrapper);return control;
  }
  function formWithSave(callback) {
    const form=element('form',undefined,'edit-form');
    form.onsubmit=e=>{e.preventDefault();save(async()=>{await callback(form.elements);AdminDraftStore.clear(form.dataset.draftKey)})};
    return form;
  }
  function formActions(form,cancel) {
    const bar=element('div',undefined,'form-actions');
    const submit=element('button','Сохранить','btn');submit.type='submit';bar.append(submit);
    if(cancel) bar.append(button('Назад к списку',cancel,'admin-action'));
    form.append(bar);
  }
  function card(name,detail) {
    const row=element('article',undefined,'admin-card');
    const copy=element('div',undefined,'card-copy');copy.append(element('h4',name),element('p',detail));row.append(copy);return row;
  }
  function controls(row,...items){const bar=element('div',undefined,'row-actions');bar.append(...items);row.append(bar)}
  let filter='all',order='upcoming',focusedBooking=null,profileId=null,query='';
  const appointment=b=>new Date(b.date+'T'+b.time);
  const stamp=value=>value?new Date(value).toLocaleString('ru',{dateStyle:'short',timeStyle:'short'}):'';
  const titles={all:'Все записи',pending_confirmation:'Необходимо подтверждение',confirmed:'Подтверждённые',completed:'Завершённые',cancelled:'Отменённые'};
  function openBooking(b){filter=b.status;focusedBooking=b.id;show(1)}
  function contact(bar,phone){
    if(!/^\+7\d{10}$/.test(phone||''))return;
    const link=element('a','Позвонить','admin-action');link.href='tel:'+phone;
    bar.append(link,button('Скопировать номер',async()=>{
      try{await navigator.clipboard.writeText(phone);message('Номер скопирован.')}catch{message('Скопируйте номер вручную: '+phone)}
    },'admin-action'));
  }
  function overview(data){
    const now=new Date(),today=dateKey(now),pending=data.bookings.filter(b=>b.status==='pending_confirmation');
    const upcoming=data.bookings.filter(b=>['pending_confirmation','confirmed'].includes(b.status)&&appointment(b)>=now).sort((a,b)=>appointment(a)-appointment(b));
    page.append(element('p','Записи, клиенты и расписание в одном пространстве','dash-sub'));
    const metrics=element('div',undefined,'metrics');page.append(metrics);
    for(const [label,n] of [['Требуют подтверждения',pending.length],['Записей сегодня',data.bookings.filter(b=>b.date===today&&b.status!=='cancelled').length],['Подтверждённых впереди',upcoming.filter(b=>b.status==='confirmed').length],['Клиентов',data.clients.length],['Активных услуг',data.services.filter(s=>s.isActive).length]]){
      const tile=element('div');tile.append(element('span',label),element('strong',n));metrics.append(tile);
    }
    const columns=element('div',undefined,'dashboard-columns');page.append(columns);
    for(const [title,records,empty] of [['Ближайшие записи',upcoming,'Предстоящих сеансов пока нет.'],['Требуют подтверждения',[...pending].sort((a,b)=>appointment(a)-appointment(b)),'Все заявки обработаны.']]){
      const block=element('section',undefined,'dash-block');block.append(element('h4',title));columns.append(block);
      if(!records.length)block.append(element('p',empty));
      for(const b of records.slice(0,4)){
        const row=card(b.clientName||b.name,[b.date,b.time,b.name,statuses[b.status]].join(' · '));controls(row,button('Открыть',()=>openBooking(b),'admin-action'));block.append(row);
      }
    }
    const summary=element('section',undefined,'dash-block'),advanced=data.schedule.activeMode==='advanced';
    let count=0,end=new Date(now);
    for(let i=0;i<30;i++){end=new Date(now);end.setDate(now.getDate()+i);const d=dateKey(end),s=data.settings;
      if(advanced?data.schedule.published[d]?.kind==='working':s.workDays.includes(end.getDay())&&!s.blockedDates.includes(d)&&!s.vacations.some(v=>v.startDate<=d&&d<=v.endDate))count++}
    summary.append(element('h4','Расписание'),element('p',(advanced?'Расширенный':'Простой')+' режим · '+count+' рабочих дней с '+today+' по '+dateKey(end)),button('Открыть расписание',()=>show(4),'admin-action'));page.append(summary);
  }
  function bookings(data){
    const filters=element('div',undefined,'status-filters');page.append(filters);
    for(const [id,label] of Object.entries(titles)){
      const count=data.bookings.filter(b=>id==='all'||b.status===id).length;
      const choice=button(label+' · '+count,()=>{filter=id;focusedBooking=null;show(1)},'admin-action');choice.setAttribute('aria-pressed',id===filter);filters.append(choice);
    }
    const sort=inputField(page,'Сортировка','sort','select',order,{choices:[['upcoming','Сначала ближайшие'],['new','Сначала новые заявки'],['old','Сначала старые заявки']]});
    sort.onchange=()=>{order=sort.value;show(1)};
    const rows=data.bookings.filter(b=>filter==='all'||b.status===filter).sort((a,b)=>{
      if(order==='upcoming'){const now=Date.now(),ta=appointment(a)-now,tb=appointment(b)-now;return ta>=0&&tb>=0?ta-tb:ta<0&&tb<0?tb-ta:ta>=0?-1:1}
      return (order==='new'?-1:1)*(a.createdAt||a.date).localeCompare(b.createdAt||b.date);
    });
    if(!rows.length)page.append(element('p',filter==='pending_confirmation'?'Заявок, ожидающих подтверждения, пока нет.':'В этой категории пока нет записей.'));
    for(const b of rows){
      const row=card(b.clientName||b.name,[b.date,b.time,b.name,b.durationMinutes+' мин',money(b)].join(' · ')),client=data.clients.find(c=>c.id===b.clientId);
      row.append(element('span',statuses[b.status],'badge '+b.status));page.append(row);
      if(data.warnings.includes(b.id))row.append(element('span',data.schedule.activeMode==='simple'&&data.settings.vacations.some(v=>v.startDate<=b.date&&v.endDate>=b.date)?'Вне графика — отпуск':'Вне графика','outside'));
      const more=element('details');more.append(element('summary','Подробности'));row.append(more);
      if(b.clientName)more.append(element('p',b.clientName));
      const phone=client?.phoneNormalized||b.clientPhone;
      if(phone)more.append(element('p',phone));
      if(b.createdAt)more.append(element('p','Заявка создана '+stamp(b.createdAt)));
      if(b.confirmedAt)more.append(element('p','Подтверждена '+stamp(b.confirmedAt)));
      if(b.cancelNote)more.append(element('p',b.cancelNote));
      if(client)more.append(button('Карточка клиента',()=>{profileId=client.id;show(2)},'admin-action'));
      const bar=element('div',undefined,'row-actions');row.append(bar);contact(bar,phone);
      if(b.status==='pending_confirmation'){
        if(appointment(b)>Date.now()&&appointment(b)-Date.now()<86400000)more.append(element('p','До сеанса меньше суток.'));
        bar.append(action('Подтвердить запись',()=>SiteAPI.confirmBooking(b.id)));
      }
      if(b.status==='confirmed')bar.append(action('Завершить',()=>SiteAPI.completeBooking(b.id)));
      if(['pending_confirmation','confirmed'].includes(b.status))bar.append(button('Отменить запись',()=>{const note=prompt('Отменить запись? При желании укажите причину.','');if(note!==null)save(()=>SiteAPI.cancelBooking(b.id,note))},'admin-action danger'));
      if(b.status==='cancelled')bar.append(action('Восстановить',()=>SiteAPI.restoreBooking(b.id),'Проверить доступность и восстановить запись?'));
      bar.append(action('Удалить',()=>SiteAPI.deleteBooking(b.id),'Удалить запись из истории?','danger'));
      if(b.id===focusedBooking){more.open=true;requestAnimationFrame(()=>row.scrollIntoView({block:'nearest'}))}
    }
  }
  function clients(data){
    const savedKey=AdminDraftStore.keys().find(k=>k.startsWith('clients/'));
    const client=data.clients.find(c=>c.id===(profileId||savedKey?.slice(8)));
    if(client){
      const history=data.bookings.filter(b=>b.clientId===client.id).sort((a,b)=>appointment(b)-appointment(a));
      const back=button('← Все клиенты',()=>{profileId=null;if(AdminDraftStore.hasChanges('clients/'+client.id)){if(!confirm('Отменить несохранённые изменения клиента?'))return;AdminDraftStore.clear('clients/'+client.id)}show(2)},'admin-action');
      page.append(back,element('h4',client.fullName),element('p','Карточка создана '+stamp(client.createdAt)+' · записей: '+history.length));
      const bar=element('div',undefined,'row-actions');contact(bar,client.phoneNormalized);page.append(bar);
      const f=formWithSave(fields=>SiteAPI.updateClient(client.id,{fullName:fields.fullName.value,phone:fields.phone.value}));
      inputField(f,'Фамилия и имя','fullName','text',client.fullName);inputField(f,'Телефон','phone','tel',client.phoneNormalized);formActions(f);page.append(f);bindDraft(f,'clients/'+client.id);
      page.append(element('h4','История записей'));
      for(const b of history){const row=card(b.name,bookingDetail(b));controls(row,button('Открыть запись',()=>openBooking(b),'admin-action'));page.append(row)}
      return;
    }
    const search=inputField(page,'Поиск по имени или телефону','search','search',query);const list=element('div');page.append(list);
    function draw(){
      query=search.value;const digits=query.replace(/\D/g,''),q=query.trim().toLocaleLowerCase('ru');
      const found=data.clients.filter(c=>c.fullName.toLocaleLowerCase('ru').includes(q)||(digits&&c.phoneNormalized.includes(digits.replace(/^8(?=\d{10}$)/,'7')))).sort((a,b)=>a.fullName.localeCompare(b.fullName,'ru'));
      list.replaceChildren();if(!found.length)list.append(element('p',query?'Клиенты не найдены.':'Клиент появится здесь после первой заявки.'));
      for(const c of found){
        const history=data.bookings.filter(b=>b.clientId===c.id),latest=history.map(b=>b.createdAt||b.date).sort().at(-1);
        const row=card(c.fullName,[c.phoneNormalized,'Записей: '+history.length,latest?'Последняя заявка '+stamp(latest):''].filter(Boolean).join(' · '));
        controls(row,button('Открыть карточку',()=>{profileId=c.id;show(2)},'admin-action'));list.append(row);
      }
    }
    search.oninput=draw;draw();
  }
  function serviceForm(s) {
    page.replaceChildren();$('adminSection').textContent=s?'Редактирование услуги':'Новая услуга';
    const f=formWithSave(fields=>{
      const payload={};
      for(const key of ['name','currency','description','visualSymbol']) payload[key]=fields[key].value.trim();
      for(const key of ['durationMinutes','priceAmount','sortOrder']) payload[key]=Number(fields[key].value);
      payload.isActive=fields.isActive.checked;
      return s?SiteAPI.updateService(s.id,payload):SiteAPI.createService(payload);
    });
    inputField(f,'Название','name','text',s?.name??'',{maxLength:120});
    inputField(f,'Описание','description','textarea',s?.description??'',{maxLength:3000});
    inputField(f,'Стоимость','priceAmount','number',s?.priceAmount??3500,{min:0,max:100000000});
    const currency=inputField(f,'Код валюты','currency','text',s?.currency??'RUB',{maxLength:3});currency.pattern='[A-Z]{3}';
    inputField(f,'Длительность, мин','durationMinutes','number',s?.durationMinutes??60,{min:1,max:1440});
    inputField(f,'Порядок показа','sortOrder','number',s?.sortOrder??10,{min:0,max:9999});
    const symbols=Array.from({length:6},(_,i)=>['logo-0'+(i+1),'Символ '+(i+1)]);
    const select=inputField(f,'Фирменный знак','visualSymbol','select',s?.visualSymbol??'logo-01',{choices:symbols});
    const preview=symbol(select.value);preview.className='symbol-preview';select.onchange=()=>{preview.src=symbol(select.value).src};f.append(preview);
    inputField(f,'Активна','isActive','checkbox',s?.isActive??true);
    formActions(f,()=>{const k=f.dataset.draftKey;if(!AdminDraftStore.hasChanges(k)||confirm('Отменить изменения услуги?')){AdminDraftStore.clear(k);show(3)}});page.append(f);bindDraft(f,'services/'+(s?.id||'new'));f.elements.name.focus();
  }
  function services(data) {
    const key=AdminDraftStore.keys().find(k=>k.startsWith('services/'));
    if(key){const id=key.slice('services/'.length),item=data.services.find(s=>s.id===id);if(item||id==='new'){serviceForm(item);return}AdminDraftStore.clear(key);message('Редактируемая услуга была удалена.')}

    page.append(button('Добавить услугу',()=>serviceForm(null),'btn'));
    for(const s of data.services.sort((a,b)=>a.sortOrder-b.sortOrder)) {
      const row=card(s.name,[s.durationMinutes+' мин',money(s),s.isActive?'Активна':'В архиве'].join(' · '));row.prepend(symbol(s.visualSymbol));
      controls(row,button('Изменить',()=>serviceForm(s),'admin-action'),
        action(s.isActive?'В архив':'Вернуть',()=>SiteAPI.updateService(s.id,{isActive:!s.isActive}),s.isActive?'Убрать услугу из каталога?':null),
        action('Удалить',()=>SiteAPI.deleteService(s.id),'Удалить услугу? История записей сохранится.','danger'));
      page.append(row);
    }
  }
  function bookingDetail(b) {
    const date=new Date(b.date+'T12:00').toLocaleDateString('ru',{dateStyle:'long'});
    return [date,b.time,b.durationMinutes+' мин',money(b),statuses[b.status]].join(' · ');
  }
  function schedule({settings:s}) {
    const f=formWithSave(fields=>SiteAPI.updateSettings({
      workDays:Array.from(f.querySelectorAll('[name=workDay]:checked'),c=>+c.value),start:fields.start.value,end:fields.end.value,
      step:+fields.step.value,breakMinutes:+fields.breakMinutes.value,blockedDates:fields.blockedDates.value.trim().split(/[\s,;]+/).filter(Boolean)
    }));
    const week=element('fieldset',undefined,'week');week.append(element('legend','Рабочие дни'));
    ['Пн','Вт','Ср','Чт','Пт','Сб','Вс'].forEach((label,i)=>{const day=(i+1)%7;inputField(week,label,'workDay','checkbox',s.workDays.includes(day)).value=day});f.append(week);
    inputField(f,'Начало дня','start','time',s.start);inputField(f,'Конец дня','end','time',s.end);
    inputField(f,'Шаг начала сеансов, мин','step','number',s.step,{min:1,max:1440});
    inputField(f,'Перерыв между сеансами, мин','breakMinutes','number',s.breakMinutes,{min:0,max:1440});
    inputField(f,'Закрытые даты (ГГГГ-ММ-ДД, по одной на строку)','blockedDates','textarea',s.blockedDates.join('\n'),{required:false});
    formActions(f);page.append(f);bindDraft(f,'schedule/week');vacationPanel(s.vacations);
  }
  function vacationPanel(initial){
    const block=element('section',undefined,'vacation-panel'),list=element('div'),editor=element('div');
    const add=button('+ Добавить отпуск',()=>edit(null,add),'admin-action');
    const heading=element('div',undefined,'vacation-heading');heading.append(element('h4','Отпуска и периоды отсутствия'),add);
    block.append(heading,list,editor);page.append(block);
    const format=d=>new Date(d+'T12:00').toLocaleDateString('ru',{day:'numeric',month:'short',year:'numeric'});
    function render(rows){
      list.replaceChildren();
      for(const v of [...rows].sort((a,b)=>a.startDate.localeCompare(b.startDate))){
        const days=Math.round((new Date(v.endDate+'T12:00')-new Date(v.startDate+'T12:00'))/86400000)+1;
        const row=card(v.startDate===v.endDate?format(v.startDate):format(v.startDate)+' — '+format(v.endDate),'Дней: '+days);
        const editButton=button('Изменить',()=>edit(v,editButton),'admin-action');
        controls(row,editButton,button('Удалить',async()=>{
          if(!confirm('Удалить отпуск? Доступность дат будет пересчитана.'))return;
          try{await SiteAPI.deleteVacation(v.id);await refresh('Отпуск удалён.')}catch(e){message(e.message)}
        },'admin-action danger'));
        list.append(row);
      }
    }
    async function refresh(notice){
      const rows=await SiteAPI.getVacations();
      editor.replaceChildren();render(rows);add.focus();message(notice);
    }
    function edit(v,opener){
      const f=element('form',undefined,'vacation-form');
      const start=inputField(f,'С','startDate','date',v?.startDate||''),end=inputField(f,'По','endDate','date',v?.endDate||'');
      start.required=end.required=true;
      const bar=element('div',undefined,'form-actions'),submit=element('button','Сохранить','btn');
      bar.append(submit,button('Отмена',()=>{if(AdminDraftStore.hasChanges(f.dataset.draftKey)&&!confirm('Отменить изменения отпуска?'))return;AdminDraftStore.clear(f.dataset.draftKey);editor.replaceChildren();opener.focus()},'admin-action'));
      const notice=element('div',undefined,'vacation-notice');notice.setAttribute('role','alert');
      f.append(bar,notice);editor.replaceChildren(f);bindDraft(f,'schedule/vacation-'+(v?.id||'new'));start.focus();
      let approved=[];
      f.oninput=()=>{approved=[];notice.replaceChildren();submit.textContent='Сохранить'};
      f.onsubmit=async event=>{
        event.preventDefault();submit.disabled=true;
        const period={startDate:start.value,endDate:end.value};
        try{
          if(v)await SiteAPI.updateVacation(v.id,period,approved);else await SiteAPI.createVacation(period,approved);
          AdminDraftStore.clear(f.dataset.draftKey);await refresh('Отпуск сохранён.');
        }catch(e){
          notice.replaceChildren(element('p',e.message));
          if(e.code==='VACATION_CONFLICT'){
            for(const [day,n] of Object.entries(e.counts).sort())notice.append(element('div',format(day)+' · записей: '+n));
            notice.append(element('p','Записи сохранятся с отметкой «Вне графика — отпуск».'));
            approved=e.ids;submit.textContent='Подтвердить отпуск';
          }
        }finally{submit.disabled=false}
      };
    }
    render(initial);
    const k=AdminDraftStore.keys().find(k=>k.startsWith('schedule/vacation-'));
    if(k){const id=k.slice('schedule/vacation-'.length),v=initial.find(x=>x.id===id);if(v||id==='new')edit(v,add)}

  }
  function content({content:c,settings}) {
    const f=formWithSave(()=>SiteAPI.updateContent(Object.fromEntries(new FormData(f))));
    for(const [key,label] of Object.entries({headline:'Заголовок главного экрана',subtitle:'Подзаголовок',servicesTitle:'Заголовок раздела услуг',about:'О кабинете',heroButton:'Кнопка главного экрана',bookingButton:'Кнопка бронирования'}))
      inputField(f,label,key,['headline','subtitle','about'].includes(key)?'textarea':'text',c[key],{maxLength:3000});
    formActions(f);page.append(f);bindDraft(f,'content/text');
    const cfg=formWithSave(fields=>SiteAPI.updateSettings({timeZone:fields.timeZone.value.trim(),address:fields.address.value.trim(),calendarTitle:fields.calendarTitle.value.trim()}));
    page.append(element('h4','События календаря'),cfg);
    inputField(cfg,'Часовой пояс IANA','timeZone','text',settings.timeZone||'',{required:false});
    inputField(cfg,'Адрес кабинета','address','text',settings.address||'',{required:false});
    inputField(cfg,'Название для календаря','calendarTitle','text',settings.calendarTitle||'FORMA TELA');
    formActions(cfg);bindDraft(cfg,'content/calendar');
  }
  function appearance({settings:s}) {
    restoreTheme=s;
    const palette={...s.palette};
    const f=formWithSave(async fields=>{
      const result=await SiteAPI.updateSettings({palette:Object.fromEntries(Object.keys(palette).map(k=>[k,fields[k].value])),animations:fields.animations.checked,revealDuration:+fields.revealDuration.value,preloaderDuration:+fields.preloaderDuration.value});
      restoreTheme=null;return result;
    });
    for(const [key,label] of Object.entries({green:'Зелёный — жизнь, тело, природа',sand:'Песочный — земля, материальный мир',paper:'Белый — пространство, тишина',gold:'Золотая точка — свет осознанности'})){
      const c=inputField(f,label,key,'color',s.palette[key]);c.oninput=()=>{palette[key]=c.value;applyTheme({...s,palette})};
    }
    inputField(f,'Декоративные анимации','animations','checkbox',s.animations);
    inputField(f,'Появление при прокрутке, мс','revealDuration','number',s.revealDuration,{min:0,max:3000});
    inputField(f,'Вступительная заставка, мс','preloaderDuration','number',s.preloaderDuration,{min:0,max:15000});
    formActions(f);page.append(f,action('Восстановить палитру',async()=>{await SiteAPI.updateSettings({palette:initialData().settings.palette});restoreTheme=null},'Восстановить исходные цвета?'));bindDraft(f,'appearance/theme');
  }
  function demo() {
    page.append(element('h4','Передача данных'));
    const download=button('Экспорт JSON',async()=>{
      try{const db=await SiteAPI.exportData(),a=element('a');if(db.clients.length&&!confirm('Экспорт JSON включает имена и телефоны клиентов. Скачать файл?'))return;
        const url=URL.createObjectURL(new Blob([JSON.stringify(db,null,2)],{type:'application/json'}));
        a.href=url;a.download='forma-tela-demo.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),3000);message('Экспорт подготовлен.');
      }catch(e){message(e.message)}
    },'admin-action');
    const importLabel=element('label','Импорт JSON','field'),file=element('input');file.type='file';file.accept='.json,application/json';importLabel.append(file);
    file.onchange=async()=>{const item=file.files[0];if(!item)return;
      if(confirm('Заменить локальные данные содержимым файла?')) await save(async()=>{
        if(item.size>10000000) throw new Error('Выберите JSON размером до 10 МБ.');
        let db;try{db=JSON.parse(await item.text())}catch{throw new Error('Файл должен содержать корректный JSON.')}
        await SiteAPI.importData(db);AdminDraftStore.clearAll();
      },'Данные импортированы.');file.value='';
    };
    page.append(download,importLabel,element('h4','Восстановление данных'));
    for(const [label,fn] of [['Сбросить услуги',SiteAPI.resetServices],['Очистить записи',SiteAPI.clearBookings],['Восстановить палитру',()=>SiteAPI.updateSettings({palette:initialData().settings.palette})],['Очистить клиентов и записи',SiteAPI.clearClientsAndBookings],['Полный сброс демо-данных',async()=>{await SiteAPI.resetDemoData();AdminDraftStore.clearAll()}]])
      page.append(action(label,fn,label+' в этом браузере?','danger'));
  }
${insert}  const views=[overview,bookings,clients,services,schedule,content,appearance,demo,specialists];
  async function show(index){
    const token=++generation;discardPreview();current=index;message('');
    $('adminSection').textContent=index===0?'Обзор кабинета':sections[index];
    [...$('adminNav').children].forEach((b,i)=>b.setAttribute('aria-current',i===index?'page':'false'));
    try{
      const data=index===7?null:await SiteAPI.exportData();
      if(data)data.warnings=await SiteAPI.getBookingWarnings();
      if(token!==generation||dialog.hidden)return;
      page.replaceChildren();
      if(index===4)Planner.render(page,data,schedule,save,message);else views[index](data);
      updateMarks();if(!page.contains(document.activeElement))$('adminSection').focus({preventScroll:true});
      if(motion())page.animate([{opacity:.2,transform:'translateY(3px)'},{opacity:1,transform:'none'}],{duration:200});
    }catch(e){message(e.message)}
  }
  sections.forEach((label,i)=>$('adminNav').append(button(label,()=>show(i))));
})();
