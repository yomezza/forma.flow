const $ = id => document.getElementById(id);
function element(tag,text,cls) {
  const node=document.createElement(tag);
  if(text!==undefined) node.textContent=text;
  if(cls) node.className=cls;
  return node;
}
function button(label,handler,cls) {
  const b=element('button',label,cls);b.type='button';b.onclick=handler;return b;
}
function symbol(id) {
  const img=element('img');img.src='assets/brand/logos/'+id+'.svg';img.alt='';img.loading='lazy';img.width=400;img.height=400;return img;
}
const money=s=>new Intl.NumberFormat('ru',{style:'currency',currency:s.currency,maximumFractionDigits:0}).format(s.priceAmount);
const dateKey=d=>[d.getFullYear(),String(d.getMonth()+1).padStart(2,'0'),String(d.getDate()).padStart(2,'0')].join('-');
const today=new Date();today.setHours(0,0,0,0);
let date=new Date(today),month=new Date(today.getFullYear(),today.getMonth(),1),serviceId='',time='',catalog=[],settings,slotRequest=0;
function applyTheme(cfg) {
  for(const key of ['green','sand','paper','gold']) document.documentElement.style.setProperty('--'+key,cfg.palette[key]);
  document.documentElement.style.setProperty('--reveal-ms',cfg.revealDuration+'ms');
  document.documentElement.classList.toggle('no-motion',!cfg.animations);
}
function updateWarning(error) {
  const note=$('storageNote');note.textContent=error || SiteAPI.getWarning();note.hidden=!note.textContent;
}
function chooseService(id) {
  serviceId=id;time='';$('status').textContent='';renderOptions();calendar();renderTime();
}
function renderOptions() {
  const list=$('options'),offset=list.scrollTop,hadFocus=list.contains(document.activeElement);
  list.replaceChildren();
  for(const s of catalog) {
    const active=s.id===serviceId;
    const b=button('',()=>chooseService(s.id),'option'+(active?' selected':''));
    b.setAttribute('aria-pressed',active);
    const label=element('span');label.append(element('b',s.name),element('em',s.durationMinutes+' мин · '+money(s)));
    b.append(element('span','','radio'),label);$('options').append(b);
  }
  fitOptions();list.scrollTop=offset;
  if(hadFocus)list.querySelector('.selected')?.focus({preventScroll:true});
  if(!catalog.length) $('options').textContent='Пока нет активных услуг.';
}
function fitOptions(){
  const list=$('options'),cards=list.children;
  list.style.maxHeight=cards.length>3?(cards[2].offsetTop-cards[0].offsetTop+cards[2].offsetHeight)+'px':'';
}
window.addEventListener('resize',fitOptions);
function renderCatalog() {
  $('serviceList').replaceChildren();
  for(const s of catalog) {
    const row=element('div','','service'),title=element('div','','service-title');
    title.append(symbol(s.visualSymbol),element('h3',s.name));
    row.append(title,element('p',s.description),button('Выбрать ↗',()=>{chooseService(s.id);location.hash='booking';const list=$('options'),selected=list.querySelector('.selected');if(selected)list.scrollTop=selected.offsetTop-list.firstChild.offsetTop}));
    $('serviceList').append(row);
  }
}
const reasons={vacation:'Отпуск',blocked:'Запись на эту дату недоступна',off:'Выходной',unplanned:'Расписание на эту дату пока не опубликовано',full:'На эту дату нет свободного времени',past:'Дата прошла'};
const hint=element('div',undefined,'calendar-tip');hint.id='date-reason';hint.hidden=true;hint.setAttribute('role','tooltip');document.body.append(hint);
function hideHint(){hint.hidden=true;document.querySelector('[aria-describedby="date-reason"]')?.removeAttribute('aria-describedby')}
function showHint(target,text){
  hideHint();target.setAttribute('aria-describedby',hint.id);hint.textContent=text;hint.hidden=false;
  const box=target.getBoundingClientRect();
  hint.style.left=Math.max(8,Math.min(innerWidth-hint.offsetWidth-8,box.left+(box.width-hint.offsetWidth)/2))+'px';
  hint.style.top=(box.top>hint.offsetHeight+10?box.top-hint.offsetHeight-5:box.bottom+5)+'px';
}
document.addEventListener('pointerdown',e=>{if(!e.target.closest('.day-off'))hideHint()});
window.addEventListener('scroll',hideHint,true);
let calendarGeneration=0;
async function calendar() {
  hideHint();const token=++calendarGeneration;
  const end=new Date(month.getFullYear(),month.getMonth()+1,0);
  const states=await SiteAPI.getDateStates(dateKey(month),dateKey(end),serviceId);
  if(token!==calendarGeneration)return;
  $('month').textContent=month.toLocaleDateString('ru',{month:'long',year:'numeric'});
  $('prev').disabled=month<=new Date(today.getFullYear(),today.getMonth(),1);
  $('days').replaceChildren(...['Пн','Вт','Ср','Чт','Пт','Сб','Вс'].map(v=>element('span',v)));
  for(let i=0;i<(month.getDay()+6)%7;i++) $('days').append(element('span'));
  for(let i=1;i<=new Date(month.getFullYear(),month.getMonth()+1,0).getDate();i++) {
    const d=new Date(month.getFullYear(),month.getMonth(),i);
    const label=d.toLocaleDateString('ru',{dateStyle:'long'});
    const reason=reasons[states[dateKey(d)]];
    if(reason){
      const closed=button(i,()=>{closed.focus();showHint(closed,reason)},'day-off '+states[dateKey(d)]);
      closed.setAttribute('aria-disabled','true');closed.setAttribute('aria-label',label+' · '+reason);
      closed.onfocus=closed.onpointerenter=()=>showHint(closed,reason);
      closed.onblur=closed.onpointerleave=hideHint;closed.onkeydown=e=>{if(e.key==='Escape')hideHint()};
      $('days').append(closed);
      continue;
    }
    const b=button(i,()=>{date=d;time='';$('status').textContent='';calendar();renderTime()});
    b.disabled=d<today;
    b.setAttribute('aria-label',label);b.setAttribute('aria-pressed',+d===+date);
    if(+d===+date) b.className='active';$('days').append(b);
  }
}
async function renderTime() {
  const request=++slotRequest;
  $('book').disabled=true;
  $('daylabel').textContent=date.toLocaleDateString('ru',{weekday:'long',day:'numeric',month:'long'});
  try {
    const slots=serviceId ? await SiteAPI.getAvailableSlots(dateKey(date),serviceId):[];
    if(request!==slotRequest) return;
    if(!slots.includes(time)) time='';
    $('times').replaceChildren();
    for(const slot of slots) {
      const b=button(slot,()=>{time=slot;$('status').textContent='';renderTime()},'slot'+(slot===time?' selected':''));
      b.setAttribute('aria-pressed',slot===time);$('times').append(b);
    }
    if(!slots.length) $('times').append(element('p',serviceId?'На эту дату нет свободного времени. Выберите другой день.':'Для записи нужна активная услуга.','empty-times'));
    $('book').disabled=!time;
  } catch(e){$('status').textContent=e.message}
}
for(const [id,delta] of [['prev',-1],['next',1]]) $(id).onclick=()=>{month.setMonth(month.getMonth()+delta);calendar()};
async function refreshPublic() {
  try {
    const [all,cfg,c]=await Promise.all([SiteAPI.getServices(),SiteAPI.getSettings(),SiteAPI.getContent()]);
    settings=cfg;catalog=all.filter(s=>s.isActive);
    if(!catalog.some(s=>s.id===serviceId)) {serviceId=catalog[0]?.id || '';time=''}
    applyTheme(cfg);
    const [first,...rest]=c.headline.split('\n');
    const h=document.querySelector('.hero h1');h.textContent=first;
    if(rest.length) h.append(element('br'),element('i',rest.join(' ')));
    document.querySelector('.hero p').textContent=c.subtitle;
    document.querySelector('#services h2').textContent=c.servicesTitle;
    document.querySelector('#about p').textContent=c.about;
    for(const [b,label] of [[document.querySelector('.hero .btn'),c.heroButton],[$('book'),c.bookingButton]]) b.replaceChildren(document.createTextNode(label),element('span','↗'));
    renderOptions();renderCatalog();calendar();await renderTime();updateWarning();
  } catch(e){updateWarning(e.message)}
}
window.addEventListener('forma-change',refreshPublic);
refreshPublic();
if('IntersectionObserver' in window) {
  const observer=new IntersectionObserver(entries=>{
    for(const entry of entries) if(entry.isIntersecting){
      entry.target.classList.add('arrived');observer.unobserve(entry.target);
    }
  });
  for(const section of document.querySelectorAll('.book,.lower')) observer.observe(section);
}

const sections=[...document.querySelectorAll('main > section')];
function focusSection(id){for(const section of sections)section.classList.toggle('focus-muted',!!id&&section.id!==id)}
document.querySelector('#visitor > header').addEventListener('click',e=>{
  const link=e.target.closest('a[href^="#"]');if(link)focusSection(link.hash==='#top'?'':link.hash.slice(1));
});
document.querySelector('footer a[href="#top"]').addEventListener('click',()=>focusSection(''));
for(const event of ['wheel','touchmove'])window.addEventListener(event,()=>focusSection(''),{passive:true});
window.addEventListener('keydown',e=>{if(['ArrowDown','ArrowUp','PageDown','PageUp','Home','End',' '].includes(e.key)&&!e.target.closest('input,select,textarea,dialog'))focusSection('')});
window.addEventListener('pointerdown',e=>{if(e.clientX>=document.documentElement.clientWidth)focusSection('')});
