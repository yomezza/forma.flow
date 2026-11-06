// Calendar transfer is deliberately independent of the booking database.
const CalendarEvent = (() => {
  const encoder=new TextEncoder();
  const marker='#event=';
  const state='Заявка ожидает подтверждения специалистом.';
  const el=(tag,text,cls)=>{
    const node=document.createElement(tag);
    if(text!==undefined)node.textContent=text;
    if(cls)node.className=cls;
    return node;
  };
  function invalid(){
    throw new Error('Проверьте данные календарного события.');
  }
  function localInstant(date,time,zone){
    if(!zone)throw new Error('Для экспорта администратору нужно указать часовой пояс кабинета в настройках.');
    if(!/^\d{4}-\d{2}-\d{2}$/.test(date)||!/^([01]\d|2[0-3]):[0-5]\d$/.test(time))invalid();
    const wall=Date.parse(date+'T'+time+':00Z');
    if(!Number.isFinite(wall)||new Date(wall).toISOString().slice(0,10)!==date)invalid();
    let formatter;
    try {
      formatter=new Intl.DateTimeFormat('en-GB',{timeZone:zone,year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hourCycle:'h23'});
    } catch {throw new Error('Укажите корректный часовой пояс кабинета в формате IANA.')}
    function projected(ms){
      const parts=Object.fromEntries(formatter.formatToParts(ms).map(p=>[p.type,p.value]));
      return Date.parse(parts.year+'-'+parts.month+'-'+parts.day+'T'+parts.hour+':'+parts.minute+':00Z');
    }
    // Try offsets on both sides of a DST transition. The first occurrence
    // is used when the wall clock repeats. A nonexistent clock time is rejected.
    const day=86400000,candidates=[];
    for(const probe of [wall-day,wall,wall+day]){
      const candidate=wall-(projected(probe)-probe);
      if(projected(candidate)===wall)candidates.push(candidate);
    }
    if(!candidates.length)throw new Error('Это местное время пропущено при переводе часов. Уточните время сеанса у специалиста.');
    return Math.min(...candidates);
  }
  function plain(value,min,max){
    return typeof value==='string'&&value.trim().length>=min&&value.length<=max&&!/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(value);
  }
  function validate(data){
    if(!data||data.v!==1||!plain(data.n,1,120)||!plain(data.t,1,120)||!plain(data.a,0,500)
      ||!plain(data.z,0,120)||!Number.isInteger(data.m)||data.m<1||data.m>1440
      ||!Number.isInteger(data.p)||data.p<0||data.p>100000000||!/^[A-Z]{3}$/.test(data.c)
      ||!/^e-[a-zA-Z0-9-]{8,100}$/.test(data.u))invalid();
    const start=localInstant(data.d,data.h,data.z);
    return {start,end:start+data.m*60000};
  }
  function fromBooking(b,s){
    const random=globalThis.crypto?.randomUUID?.()||Date.now().toString(36)+'-'+Math.random().toString(36).slice(2);
    // The transferable ID is separate from customer and reservation IDs.
    return {v:1,u:'e-'+random,t:s.calendarTitle||'FORMA TELA',n:b.name,d:b.date,h:b.time,
      z:s.timeZone||'',m:b.durationMinutes,p:b.priceAmount,c:b.currency,a:s.address||''};
  }
  const title=data=>data.t+' — массаж (ожидает подтверждения)';
  const price=data=>new Intl.NumberFormat('ru',{style:'currency',currency:data.c,maximumFractionDigits:0}).format(data.p);
  const description=data=>[data.n,data.m+' мин · '+price(data),state,data.a].filter(Boolean).join('\n');
  const stamp=ms=>new Date(ms).toISOString().replace(/[-:]/g,'').replace(/\.\d{3}/,'');
  const escapeText=text=>text.replace(/\\/g,'\\\\').replace(/\r\n|\r|\n/g,'\\n').replace(/[,;]/g,c=>'\\'+c);
  function fold(line){
    const lines=[];let current='',bytes=0;
    for(const cp of line){
      const size=encoder.encode(cp).length;
      if(bytes+size>75){lines.push(current);current=' ';bytes=1}
      current+=cp;bytes+=size;
    }
    lines.push(current);return lines.join('\r\n');
  }
  function ics(data){
    const {start,end}=validate(data);
    const lines=['BEGIN:VCALENDAR','VERSION:2.0','PRODID:-//FORMA TELA//Bookings//RU','CALSCALE:GREGORIAN','BEGIN:VEVENT',
      'UID:'+data.u+'@forma-tela','DTSTAMP:'+stamp(Date.now()),'DTSTART:'+stamp(start),'DTEND:'+stamp(end),
      'SUMMARY:'+escapeText(title(data)),'DESCRIPTION:'+escapeText(description(data))];
    if(data.a)lines.push('LOCATION:'+escapeText(data.a));
    lines.push('STATUS:TENTATIVE','END:VEVENT','END:VCALENDAR');
    return lines.map(fold).join('\r\n')+'\r\n';
  }
  function google(data){
    const {start,end}=validate(data),url=new URL('https://calendar.google.com/calendar/render');
    for(const [key,value] of Object.entries({action:'TEMPLATE',text:title(data),dates:stamp(start)+'/'+stamp(end),
      ctz:data.z,details:description(data),location:data.a})){
      if(value)url.searchParams.set(key,value);
    }
    return url.href;
  }
  function transferUrl(data){
    const url=new URL('calendar-event.html',location.href);
    const binary=String.fromCharCode(...encoder.encode(JSON.stringify(data)));
    url.hash=marker+btoa(binary).replace(/=/g,'').replace(/\+/g,'-').replace(/\//g,'_');
    return url.href;
  }
  function decode(hash){
    if(!hash.startsWith(marker)||hash.length>9000)invalid();
    const raw=hash.slice(marker.length);
    if(!/^[A-Za-z0-9_-]+$/.test(raw))invalid();
    const binary=atob(raw.replace(/-/g,'+').replace(/_/g,'/'));
    const data=JSON.parse(new TextDecoder('utf-8',{fatal:true}).decode(Uint8Array.from(binary,c=>c.charCodeAt(0))));
    validate(data);return data;
  }
  function publicHost(){
    const host=location.hostname.toLowerCase();
    return location.protocol==='https:' && !/^(localhost$|127\.|10\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.|169\.254\.|\[)/.test(host)
      && !host.endsWith('.local') && !host.endsWith('.localhost') && host.includes('.');
  }
  function qrSvg(url){
    const cells=FormaQR(url),n=cells.length+8,ns='http://www.w3.org/2000/svg';
    const svg=document.createElementNS(ns,'svg');svg.setAttribute('viewBox','0 0 '+n+' '+n);
    svg.setAttribute('shape-rendering','crispEdges');svg.setAttribute('role','img');svg.setAttribute('aria-label','QR-код календарного события');
    const paper=document.createElementNS(ns,'rect');paper.setAttribute('width',n);paper.setAttribute('height',n);paper.setAttribute('fill','white');
    const path=document.createElementNS(ns,'path'),d=[];
    cells.forEach((row,y)=>row.forEach((dark,x)=>{if(dark)d.push('M'+(x+4)+' '+(y+4)+'h1v1h-1z')}));
    path.setAttribute('d',d.join(''));path.setAttribute('fill','#000');svg.append(paper,path);return svg;
  }
  function link(label,href){
    const a=el('a',label,'calendar-action');a.href=href;return a;
  }
  function control(label,click){
    const b=el('button',label,'calendar-action');b.type='button';b.onclick=click;return b;
  }
  function actions(data){
    const box=el('div',undefined,'calendar-actions'),notice=el('small');notice.setAttribute('role','status');
    const download=control('Скачать файл .ics',()=>{
      const url=URL.createObjectURL(new Blob([ics(data)],{type:'text/calendar;charset=utf-8'}));
      const a=link('',url);a.download='forma-tela-'+data.d+'.ics';document.body.append(a);a.click();a.remove();
      setTimeout(()=>URL.revokeObjectURL(url),30000);
    });
    const googleLink=link('Добавить в Google Календарь',google(data));googleLink.target='_blank';googleLink.rel='noopener noreferrer';
    box.append(download,googleLink);
    if(navigator.share){
      box.append(control('Поделиться событием',async()=>{
        try{
          const file=new File([ics(data)],'forma-tela.ics',{type:'text/calendar'});
          if(navigator.canShare?.({files:[file]}))await navigator.share({files:[file],title:title(data)});
          else await navigator.share({title:title(data),url:publicHost()?transferUrl(data):google(data)});
        }catch(e){if(e.name!=='AbortError')notice.textContent='Вы можете скачать файл .ics или скопировать ссылку.'}
      }));
    }
    box.append(notice);return box;
  }
  function render(container,booking,settings){
    const data=fromBooking(booking,settings);
    const details=el('details',undefined,'calendar-choice');details.append(el('summary','Сохранить в календарь'));
    container.append(details);
    try{validate(data)}catch(e){details.append(el('p',e.message));return}
    details.append(actions(data),el('p','После звонка событие в личном календаре можно обновить вручную.','calendar-note'));
    const hosted=publicHost(),target=hosted?transferUrl(data):google(data);
    const figure=el('figure',undefined,'transfer-qr');
    try {
      figure.append(qrSvg(target),el('figcaption','Сканируйте, чтобы добавить событие в календарь телефона'));
      if(!hosted)figure.append(el('small','При локальном запуске QR-код открывает Google Календарь.'));
    } catch {figure.append(el('small','Для переноса события используйте ссылку.'))}
    const notice=el('small');notice.setAttribute('role','status');
    figure.append(control('Скопировать ссылку',async()=>{
      try{await navigator.clipboard.writeText(target);notice.textContent='Ссылка скопирована.'}
      catch{const input=el('input');input.type='text';input.readOnly=true;input.value=target;
        input.setAttribute('aria-label','Ссылка на событие');notice.replaceChildren(input);input.focus();input.select()}
    }),notice);
    container.append(figure);
  }
  function landing(main){
    main.className='event-sheet';
    const mark=el('img');mark.src='assets/brand/logos/logo-01.svg';mark.width=42;mark.height=42;mark.alt='';
    main.append(mark,el('p','FORMA TELA','calendar-brand'));
    try{
      const data=decode(location.hash),{start}=validate(data);
      const date=new Intl.DateTimeFormat('ru',{timeZone:data.z,dateStyle:'long',timeStyle:'short'}).format(start);
      main.append(el('h1',data.n),el('p',date+' · '+data.z),el('p',data.m+' мин · '+price(data)));
      if(data.a)main.append(el('p',data.a));
      main.append(el('p',state),actions(data),el('p','Копия события. Изменения записи нужно внести в личный календарь вручную.','calendar-note'));
    }catch{main.append(el('h1','Событие недоступно'),el('p','Проверьте ссылку или отсканируйте QR-код заново.'))}
    main.append(link('На сайт кабинета','index.html'));
  }
  const main=document.getElementById('transferred-event');if(main)landing(main);
  return {render,ics,google,validate,fromBooking,transferUrl,decode};
})();
