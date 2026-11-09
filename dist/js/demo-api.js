const DemoAPI = (() => {
  const key = 'formaTela.demo.v1';
  const clone = value => JSON.parse(JSON.stringify(value));
  const uid = () => globalThis.crypto?.randomUUID?.() || 'id-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2);
  const timePattern = /^([01]\d|2[0-3]):[0-5]\d$/;
  const minutes = time => Number(time.slice(0,2))*60 + Number(time.slice(3));
  const statuses = ['pending_confirmation','confirmed','completed','cancelled'];
  function check(test,message='Проверьте формат данных.',code='VALIDATION') {
    if(!test) throw Object.assign(new Error(message),{code});
  }
  const str = (v,max=3000) => typeof v==='string' && v.trim().length>0 && v.length<=max;
  const num = (v,min,max) => Number.isInteger(v) && v>=min && v<=max;
  const isId = v => str(v,100) && /^[\w-]+$/.test(v);
  function dateOK(value) {
    if(typeof value!=='string' || !/^\d{4}-\d\d-\d\d$/.test(value)) return false;
    const d = new Date(value+'T12:00:00');
    return d.getDate()===+value.slice(8) && d.getMonth()+1===+value.slice(5,7);
  }
  function serviceOK(s) {
    check(s && isId(s.id) && str(s.name,120) && str(s.description) && num(s.durationMinutes,1,1440) &&
      num(s.priceAmount,0,100000000) && /^[A-Z]{3}$/.test(s.currency) && num(s.sortOrder,0,9999) &&
      typeof s.isActive==='boolean' && /^logo-0[1-6]$/.test(s.visualSymbol),'Проверьте поля услуги.');
  }
  const activeRequest=b=>b.status==='confirmed'||b.status==='pending_confirmation';
  function phoneNumber(value){
    const digits=String(value||'').replace(/[() +\-]/g,'');
    const phone=/^\d{10}$/.test(digits)?'+7'+digits:/^[78]\d{10}$/.test(digits)?'+7'+digits.slice(1):'';
    check(phone,'Введите номер +7 и десять цифр.');return phone;
  }
  function identity(data){
    const fullName=String(data.fullName||'').trim().replace(/\s+/g,' ');
    check(str(fullName,120)&&fullName.split(' ').length>=2,'Укажите фамилию и имя.');
    return {fullName,phoneNormalized:phoneNumber(data.phoneNormalized??data.phone)};
  }
  function clientOK(c){
    check(c && isId(c.id) && identity(c).phoneNormalized===c.phoneNormalized && Number.isFinite(Date.parse(c.createdAt)) && Number.isFinite(Date.parse(c.updatedAt)),'Некорректная карточка клиента.');
  }
  function newClient(db,data){
    const fields=identity(data),now=new Date().toISOString();
    const c={...fields,id:uid(),createdAt:now,updatedAt:now};db.clients.push(c);return c;
  }
  function transition(db,id,status,note=''){
    const b=find(db.bookings,id);
    if(b.status===status)return b;
    const allowed={pending_confirmation:['confirmed','cancelled'],confirmed:['completed','cancelled'],completed:[],cancelled:[]};
    check(allowed[b.status].includes(status),'Недопустимое изменение статуса.');
    b.status=status;b.updatedAt=new Date().toISOString();
    if(status==='confirmed')b.confirmedAt=b.updatedAt;
    if(status==='cancelled'){b.cancelledAt=b.updatedAt;b.cancelNote=String(note).slice(0,500)}
    return b;
  }
  function bookingOK(b) {
    check(b && isId(b.id) && isId(b.serviceId) && str(b.name,120) && num(b.durationMinutes,1,1440) &&
      num(b.priceAmount,0,100000000) && /^[A-Z]{3}$/.test(b.currency) && dateOK(b.date) &&
      timePattern.test(b.time) && statuses.includes(b.status),'Некорректные данные записи.');
  }
  function vacationRange(v) {
    check(v && dateOK(v.startDate) && dateOK(v.endDate),'Укажите обе даты отпуска.');
    check(v.startDate<=v.endDate,'Дата начала отпуска должна быть раньше окончания или совпадать с ним.');
  }
  function vacationsOK(list) {
    check(Array.isArray(list),'Некорректный список отпусков.');
    const ids=new Set();
    for(const v of list){vacationRange(v);check(isId(v.id)&&!ids.has(v.id),'Повторяющийся или некорректный ID отпуска.');ids.add(v.id)}
    const sorted=[...list].sort((a,b)=>a.startDate.localeCompare(b.startDate));
    for(let i=1;i<sorted.length;i++)check(sorted[i-1].endDate<sorted[i].startDate,'Этот период пересекается с другим отпуском.');
  }
  const containsDate=(v,d)=>d>=v.startDate && d<=v.endDate;
  const onVacation=(db,date)=>db.settings.vacations.some(v=>containsDate(v,date));
  function storeVacation(db,id,input,confirmedIds) {
    const v={id:id||uid(),startDate:input.startDate,endDate:input.endDate};
    if(id)find(db.settings.vacations,id);
    const list=db.settings.vacations.filter(row=>row.id!==id).concat(v);
    vacationsOK(list);
    const conflicts=db.bookings.filter(b=>activeRequest(b) && containsDate(v,b.date));
    if(conflicts.some(b=>!confirmedIds.includes(b.id))){
      const counts={};
      for(const b of conflicts) counts[b.date]=(counts[b.date]||0)+1;
      throw Object.assign(new Error('В период отпуска есть записи.'),{code:'VACATION_CONFLICT',counts,ids:conflicts.map(b=>b.id)});
    }
    db.settings.vacations=list;return v;
  }
  function settingsOK(s) {
    check(s && Array.isArray(s.workDays) && s.workDays.every(d=>num(d,0,6)) &&
      timePattern.test(s.start) && timePattern.test(s.end) && s.start<s.end &&
      num(s.step,1,1440) && num(s.breakMinutes,0,1440) && Array.isArray(s.blockedDates) &&
      s.blockedDates.every(dateOK),'Проверьте дни, рабочие часы и интервалы расписания.');
    vacationsOK(s.vacations);
    check(['green','sand','paper','gold'].every(k=>/^#[0-9a-f]{6}$/i.test(s.palette?.[k])) &&
      typeof s.animations==='boolean' && num(s.revealDuration,0,3000) && num(s.preloaderDuration,0,15000),'Проверьте параметры оформления.');
  }
  const emptyDay = () => ({kind:'unplanned', intervals:[], stepOverride:null});
  function migrate(db) {
    if(db.version===1) {
      db.schedule={activeMode:'simple',published:{},drafts:{},history:[]};
      db.version=2;
    }
    if([2,3].includes(db.version) && db.settings && db.settings.vacations===undefined) db.settings.vacations=[];
    if(db.version===2){db.clients??=[];db.version=3}
    if(db.version===3){
      db.revision=0;
      const services=db.services.filter(service=>service.isActive);
      db.specialists=[{id:'sp-001',fullName:'Специалист FORMA TELA',state:'active',accountId:'acct-owner',createdAt:new Date().toISOString(),archivedAt:null}];
      db.rooms=[{id:'room-001',name:'Кабинет № 1',isActive:true,sortOrder:0}];
      db.specialistServices=services.map(service=>({specialistId:'sp-001',serviceId:service.id}));
      db.specialistSchedules=[{specialistId:'sp-001',revision:0,week:Object.fromEntries([0,1,2,3,4,5,6].map(day=>[day,[{start:db.settings.start,end:db.settings.end}]])),overrides:{}}];
      db.absenceRequests=[];
      db.accounts=[{id:'acct-owner',login:'owner',displayName:'Владелец',roles:['admin','specialist'],specialistId:'sp-001',state:'active',mustCompleteFirstDemoLogin:false}];
      db.demoSession={activeAccountId:'acct-owner',activeRole:'admin'};
      db.assignmentEvents=[];
      db.auditEvents=[];
      db.capacityExceptions=[];
      for(const booking of db.bookings){booking.specialistId=null;booking.roomId=null;booking.assignmentUpdatedAt=null;booking.completion=null;}
      db.version=4;
    }
    return db;
  }
  function dayRule(data) {
    check(data && ['working','off','unplanned'].includes(data.kind),'Выберите состояние дня.');
    const {kind,stepOverride=null}=data;
    check(stepOverride===null || num(stepOverride,1,1440),'Шаг должен быть от 1 до 1440 минут.');
    if(kind!=='working') return {...emptyDay(),kind};
    check(Array.isArray(data.intervals) && data.intervals.length,'Добавьте рабочий интервал.');
    const intervals=data.intervals.map(({start,end})=>{
      check(timePattern.test(start) && timePattern.test(end) && start<end,'Проверьте границы интервала.');
      return {start,end};
    }).sort((a,b)=>a.start.localeCompare(b.start));
    intervals.forEach((r,i)=>check(!i || intervals[i-1].end<=r.start,'Рабочие интервалы пересекаются.'));
    return {kind,intervals,stepOverride};
  }
  function dates(a,b) {
    check(dateOK(a)&&dateOK(b)&&a<=b,'Проверьте начало и конец диапазона.');
    const list=[],last=Date.parse(b+'T00:00Z');
    for(let t=Date.parse(a+'T00:00Z');t<=last;t+=86400000) list.push(new Date(t).toISOString().slice(0,10));
    return list;
  }
  function simple(db,date) {
    const s=db.settings;
    if(s.blockedDates.includes(date)||!s.workDays.includes(new Date(date+'T12:00').getDay())) return {...emptyDay(),kind:'off'};
    return {kind:'working',intervals:[{start:s.start,end:s.end}],stepOverride:null};
  }
  function effective(db,date,mode=db.schedule.activeMode) {
    if(mode==='advanced') return db.schedule.published[date]||emptyDay();
    if(onVacation(db,date)) return {...emptyDay(),kind:'off',reason:'vacation'};
    if(db.settings.blockedDates.includes(date)) return {...emptyDay(),kind:'off',reason:'blocked'};
    return simple(db,date);
  }
  function fits(db,b,rule) {
    const t=minutes(b.time),gap=db.settings.breakMinutes;
    return rule.intervals.some(r=>t>=minutes(r.start) && t+b.durationMinutes<=minutes(r.end) && (t-minutes(r.start))%(rule.stepOverride||db.settings.step)===0)
      && !db.bookings.some(other=>other.date===b.date && other.id!==b.id && other.status!=='cancelled' && t<minutes(other.time)+other.durationMinutes+gap && minutes(other.time)<t+b.durationMinutes+gap);
  }
  function log(db,action,before,after) {
    db.schedule.history.unshift({id:uid(),at:new Date().toISOString(),action,before,after});
    db.schedule.history.length=Math.min(db.schedule.history.length,30);
  }
  function applyDraft(db,changes,action) {
    const before={},after={};
    for(const [date,data] of Object.entries(changes)) {
      check(dateOK(date)); const rule=dayRule(data);
      before[date]=db.schedule.drafts[date]||db.schedule.published[date]||emptyDay();
      db.schedule.drafts[date]=after[date]=rule;
    }
    log(db,action,before,after);
  }
  function subset(map,a,b) {return Object.fromEntries(Object.entries(map).filter(([d])=>d>=a&&d<=b))}
  function preview(db,changes) {
    const rules=Object.values(changes);
    return {total:rules.length,working:rules.filter(r=>r.kind==='working').length,off:rules.filter(r=>r.kind==='off').length,
      unavailable:Object.keys(changes).filter(d=>changes[d].kind!=='working'),
      conflicts:db.bookings.filter(b=>activeRequest(b) && changes[b.date] && !fits(db,b,changes[b.date])).map(b=>({id:b.id,date:b.date,time:b.time}))};
  }
  function scheduleOK(s) {
    check(s && ['simple','advanced'].includes(s.activeMode) && Array.isArray(s.history) && s.history.length<=30);
    function validMap(map) {
      check(map && typeof map==='object' && !Array.isArray(map));
      for(const [date,r] of Object.entries(map)){check(dateOK(date));map[date]=dayRule(r)}
    }
    validMap(s.drafts);validMap(s.published);
    for(const h of s.history) {
      check(h && isId(h.id) && str(h.action) && Number.isFinite(Date.parse(h.at)));
      if(h.action==='Переключение режима') check([h.before,h.after].every(m=>['simple','advanced'].includes(m)));
      else {validMap(h.before);validMap(h.after)}
    }
  }
  function validate(db) {
    check(db?.version===4 && Number.isSafeInteger(db.revision) && db.revision>=0,'Неподдерживаемая версия данных.','VERSION');
    for(const [rows,validator] of [[db.services,serviceOK],[db.bookings,bookingOK],[db.clients,clientOK]]) {
      check(Array.isArray(rows));rows.forEach(validator);
      check(new Set(rows.map(r=>r.id)).size===rows.length,'Идентификаторы должны быть уникальными.');
    }
    check(new Set(db.clients.map(c=>c.phoneNormalized)).size===db.clients.length,'Номера клиентов должны быть уникальны.');
    check(db.bookings.every(b=>b.clientId==null||db.clients.some(c=>c.id===b.clientId)),'Клиент записи отсутствует.');
    if(db.settings.calendarTitle!==undefined)check(str(db.settings.calendarTitle,120),'Введите название длиной до 120 символов.');
    if(db.settings.address!==undefined)check(typeof db.settings.address==='string'&&db.settings.address.length<=500,'Введите адрес длиной до 500 символов.');
    if(db.settings.timeZone){try{new Intl.DateTimeFormat('ru',{timeZone:db.settings.timeZone})}catch{check(false,'Укажите часовой пояс IANA, например Europe/Moscow.')}}
    settingsOK(db.settings);scheduleOK(db.schedule);
    check(Object.keys(initialData().content).every(k=>str(db.content?.[k])),'Заполните текстовые поля.');
    check(Array.isArray(db.specialists)&&Array.isArray(db.rooms)&&Array.isArray(db.specialistServices)&&Array.isArray(db.specialistSchedules)&&Array.isArray(db.absenceRequests)&&Array.isArray(db.accounts)&&Array.isArray(db.assignmentEvents)&&Array.isArray(db.auditEvents)&&Array.isArray(db.capacityExceptions),'Некорректная схема v4.');
    check(db.demoSession&&typeof db.demoSession==='object','Некорректная сессия демо.');
    for(const b of db.bookings) check((b.specialistId==null||typeof b.specialistId==='string')&&(b.roomId==null||typeof b.roomId==='string')&&(b.assignmentUpdatedAt==null||typeof b.assignmentUpdatedAt==='string')&&(b.completion==null||typeof b.completion==='object'),'Некорректное назначение записи.');
  }
  let memory=migrate(initialData()), persistent=true, warning='';
  function storageFailed() {
    persistent=false;warning='Постоянное хранение недоступно. Изменения действуют до закрытия страницы. Вы можете экспортировать JSON.';
  }
  function read() {
    if(!persistent) return clone(memory);
    let raw;
    try {raw=localStorage.getItem(key)} catch {storageFailed();return clone(memory)}
    if(raw===null) return clone(memory);
    try {const db=JSON.parse(raw);const old=db.version<4 || db.settings?.vacations===undefined;migrate(db);validate(db);if(old) try{localStorage.setItem(key,JSON.stringify(db))}catch{storageFailed()}memory=db;return clone(db)}
    catch {throw Object.assign(new Error('Сохранённые данные повреждены или имеют другую версию. Используйте импорт или полный сброс.'),{code:'INVALID_DATABASE'})}
  }
  function write(db) {
    validate(db);
    if(persistent) try {localStorage.setItem(key,JSON.stringify(db))} catch {storageFailed()}
    memory=clone(db);
    window.dispatchEvent(new Event('forma-change'));
  }
  function snapshot(s) {
    return {serviceId:s.id,name:s.name,priceAmount:s.priceAmount,currency:s.currency,durationMinutes:s.durationMinutes};
  }
  try {
    if(localStorage.getItem(key)===null) {
      const legacy=JSON.parse(localStorage.getItem('forma-tela-demo-v1') || '[]');
      if(Array.isArray(legacy)) for(const b of legacy) {
        const s=memory.services.find(s=>s.name===b?.service);
        if(s && dateOK(b.date) && timePattern.test(b.time))
          memory.bookings.push({...snapshot(s),id:uid(),date:b.date,time:b.time,status:'confirmed'});
      }
      write(memory);
    }
  } catch {storageFailed()}
  function find(rows,id) {
    const row=rows.find(r=>r.id===id);
    check(row,'Объект не найден.','NOT_FOUND');return row;
  }
  function available(db,date,id) {
    check(dateOK(date),'Некорректная дата.');
    const s=find(db.services,id), cfg=db.settings, slots=[];
    const rule=effective(db,date);
    if(!s.isActive) return slots;
    const taken=db.bookings.filter(b=>b.date===date && b.status!=='cancelled');
    for(const range of rule.intervals) for(let t=minutes(range.start);t+s.durationMinutes<=minutes(range.end);t+=rule.stepOverride||cfg.step) {
      const label=String(Math.floor(t/60)).padStart(2,'0')+':'+String(t%60).padStart(2,'0');
      if(new Date(date+'T'+label)<=new Date()) continue;
      const candidate={id:'__slot__',serviceId:s.id,date,time:label,durationMinutes:s.durationMinutes,status:'pending_confirmation',specialistId:null,roomId:null};
      const placement=globalThis.FormaAssignmentEngine?.solveAssignments(db,{date,candidate});
      if(placement?.feasible) slots.push(label);
    }
    return slots;
  }
  async function change(fn) {
    const perform=()=>{const db=read(), result=fn(db);write(db);return clone(result??null)};
    return globalThis.navigator?.locks ? navigator.locks.request(key,perform) : perform();
  }
  window.addEventListener('storage',e=>{if(e.key===key || e.key===null) window.dispatchEvent(new Event('forma-change'))});
  return {
    async getVacations(){return read().settings.vacations.sort((a,b)=>a.startDate.localeCompare(b.startDate))},
    createVacation:(v,confirmedIds=[])=>change(db=>storeVacation(db,null,v,confirmedIds)),
    updateVacation:(id,v,confirmedIds=[])=>change(db=>storeVacation(db,id,v,confirmedIds)),
    deleteVacation:id=>change(db=>{find(db.settings.vacations,id);db.settings.vacations=db.settings.vacations.filter(v=>v.id!==id)}),
    async isVacationDate(date){check(dateOK(date));return onVacation(read(),date)},
    async getScheduleMode(){return read().schedule.activeMode},
    async getDateStates(a,b,id){
      const db=read(),today=new Date();today.setHours(0,0,0,0);
      return Object.fromEntries(dates(a,b).map(d=>{
        const r=effective(db,d);
        const state=new Date(d+'T00:00')<today?'past':r.kind==='working' && (!id||!available(db,d,id).length)?'full':r.reason||r.kind;
        return [d,state];
      }));
    },
    async getPublishedSchedule(a,b){return subset(read().schedule.published,a,b)},
    async getDraftSchedule(a,b){return subset(read().schedule.drafts,a,b)},
    async getDateSchedule(d){const db=read();return {active:effective(db,d),draft:db.schedule.drafts[d]||null,published:db.schedule.published[d]||null}},
    async getBookingWarnings(){const db=read();return db.bookings.filter(b=>activeRequest(b)&&!fits(db,b,effective(db,b.date))).map(b=>b.id)},
    async getBookingsForDate(d){return read().bookings.filter(b=>b.date===d)},
    async previewScheduleModeChange(mode,a,b){
      const db=read(),list=dates(a,b),live=db.services.filter(s=>s.isActive);
      const alternate={...db,schedule:{...db.schedule,activeMode:mode}};
      const changed=list.filter(d=>live.some(s=>available(db,d,s.id).join()!==available(alternate,d,s.id).join())).length;
      return {changed,conflicts:db.bookings.filter(r=>activeRequest(r)&&r.date>=a&&r.date<=b&&!fits(db,r,effective(db,r.date,mode))).length};
    },
    setScheduleMode:mode=>change(db=>{check(['simple','advanced'].includes(mode));log(db,'Переключение режима',db.schedule.activeMode,mode);db.schedule.activeMode=mode}),
    setDraftSchedule:(date,data)=>change(db=>applyDraft(db,{[date]:data},'Изменение дня')),
    setDraftScheduleRange:(a,b,data)=>change(db=>applyDraft(db,Object.fromEntries(dates(a,b).map(d=>[d,data.week?data.week[new Date(d+'T12:00').getDay()]:data])),'Черновик диапазона')),
    copySimpleScheduleToDraft:(a,b)=>change(db=>applyDraft(db,Object.fromEntries(dates(a,b).map(d=>[d,simple(db,d)])),'Копирование недельного графика')),
    async previewSchedulePublication(a,b){const db=read();return preview(db,subset(db.schedule.drafts,a,b))},
    publishScheduleRange:(a,b)=>change(db=>{
      const changes=subset(db.schedule.drafts,a,b),before={};
      for(const [d,rule] of Object.entries(changes)){
        before[d]=db.schedule.published[d]||emptyDay();
        if(rule.kind==='unplanned') delete db.schedule.published[d];else db.schedule.published[d]=rule;
        delete db.schedule.drafts[d];
      }
      if(Object.keys(changes).length) log(db,'Публикация',before,changes);
    }),
    unpublishDate:d=>change(db=>applyDraft(db,{[d]:emptyDay()},'Снятие с публикации в черновике')),
    async getScheduleHistory(){return read().schedule.history},
    async previewRestore(id){const db=read();return preview(db,find(db.schedule.history,id).before)},
    restoreScheduleVersion:id=>change(db=>{const h=find(db.schedule.history,id);check(typeof h.before==='object');applyDraft(db,h.before,'Восстановление в черновик')}),
    getWarning:()=>warning,
    async getServices(){return read().services.sort((a,b)=>a.sortOrder-b.sortOrder)},
    async getServiceById(id){return find(read().services,id)},
    createService:data=>change(db=>{const row={...data,id:uid()};db.services.push(row);return row}),
    updateService:(id,data)=>change(db=>Object.assign(find(db.services,id),data,{id})),
    deleteService:id=>change(db=>{find(db.services,id);db.services=db.services.filter(s=>s.id!==id)}),
    async getBookings(){return read().bookings},
    async getAvailableSlots(date,id){return available(read(),date,id)},
    createBooking:data=>change(db=>{
      check(available(db,data.date,data.serviceId).includes(data.time),'Это время уже недоступно. Выберите другое.','SLOT_UNAVAILABLE');
      const b={...snapshot(find(db.services,data.serviceId)),id:uid(),date:data.date,time:data.time,status:'confirmed'};
      db.bookings.push(b);return b;
    }),
    async getClients(){return read().clients},
    async getClientById(id){return find(read().clients,id)},
    async findClientByPhone(phone){return read().clients.find(c=>c.phoneNormalized===phoneNumber(phone))||null},
    createClient:data=>change(db=>newClient(db,data)),
    updateClient:(id,data)=>change(db=>{
      const c=find(db.clients,id),fields=identity({...c,...data,phoneNormalized:data.phoneNormalized??data.phone??c.phoneNormalized});
      check(!db.clients.some(row=>row.id!==id && row.phoneNormalized===fields.phoneNormalized),'Этот номер уже принадлежит другому клиенту.');
      return Object.assign(c,fields,{updatedAt:new Date().toISOString()});
    }),
    async getClientBookings(id){return read().bookings.filter(b=>b.clientId===id)},
    async getPendingBookings(){return read().bookings.filter(b=>b.status==='pending_confirmation')},
    createBookingRequest:data=>change(db=>{
      check(available(db,data.date,data.serviceId).includes(data.time),'Это время уже занято. Выберите другое время.','SLOT_UNAVAILABLE');
      const fields=identity(data);
      const now=new Date().toISOString();
      const request={...snapshot(find(db.services,data.serviceId)),id:uid(),date:data.date,time:data.time,
        clientId:null,specialistId:null,roomId:null,assignmentUpdatedAt:null,completion:null,clientName:fields.fullName,clientPhone:fields.phoneNormalized,status:'pending_confirmation',createdAt:now,updatedAt:now};
      db.bookings.push(request);return request;
    }),
    confirmBooking:id=>change(db=>{const b=find(db.bookings,id);check(b.clientId&&db.clients.some(c=>c.id===b.clientId),'Сначала привяжите карточку клиента.','CLIENT_LINK_REQUIRED');return transition(db,id,'confirmed')}),
    async findClientsByPhone(phone){const normalized=phoneNumber(phone);return read().clients.filter(c=>c.phoneNormalized===normalized)},
    assignBookingClient:(id,{clientId,newClient:clientData}={})=>change(db=>{
      const booking=find(db.bookings,id);
      const client=clientId?find(db.clients,clientId):clientData?newClient(db,clientData):null;
      check(client,'Выберите или создайте карточку клиента.');
      booking.clientId=client.id;
      db.auditEvents.push({id:uid(),action:'CLIENT_LINKED',bookingId:id,clientId:client.id,at:new Date().toISOString()});
      return booking;
    }),
    cancelBooking:(id,note)=>change(db=>transition(db,id,'cancelled',note)),
    completeBooking:id=>change(db=>transition(db,id,'completed')),
    restoreBooking:id=>change(db=>{
      const b=find(db.bookings,id);check(b.status==='cancelled');
      check(available(db,b.date,b.serviceId).includes(b.time)&&fits(db,b,effective(db,b.date)),'Время недоступно для восстановления.');
      b.status=b.clientId?'pending_confirmation':'confirmed';b.updatedAt=new Date().toISOString();delete b.confirmedAt;return b;
    }),
    updateBooking:(id,{status})=>change(db=>transition(db,id,status)),
    clearClientsAndBookings:()=>change(db=>{db.bookings=[];db.clients=[]}),
    deleteBooking:id=>change(db=>{find(db.bookings,id);db.bookings=db.bookings.filter(b=>b.id!==id)}),
    async getSettings(){return read().settings},
    updateSettings:data=>change(db=>(db.settings={...db.settings,...data,palette:{...db.settings.palette,...data.palette}})),
    resetSettings:()=>change(db=>(db.settings=initialData().settings)),
    async getContent(){return read().content},
    updateContent:data=>change(db=>Object.assign(db.content,data)),
    resetServices:()=>change(db=>(db.services=initialData().services)),
    clearBookings:()=>change(db=>(db.bookings=[])),
    async resetDemoData(){write(migrate(initialData()))},
    async exportData(){return read()},
    async importData(data){const db=migrate(clone(data));validate(db);write(db)}
  };
})();
const SiteAPI = DemoAPI;
