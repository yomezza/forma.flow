(() => {
  const dialog=$('request'),form=$('requestForm'),phone=form.elements.phone;
  let selected;
  const clearErrors=()=>{for(const id of ['requestError','nameError','phoneError'])$(id).textContent=''};
  function format(raw){
    let digits=raw.replace(/\D/g,'');
    if(digits.length>10 && /^[78]/.test(digits))digits=digits.slice(1);
    const d=digits.slice(0,10);
    return '+7'+(d?' ('+d.slice(0,3):'')+(d.length>3?') '+d.slice(3,6):'')+(d.length>6?'-'+d.slice(6,8):'')+(d.length>8?'-'+d.slice(8):'');
  }
  phone.oninput=()=>{
    const raw=phone.value,caret=phone.selectionStart;
    const body=raw.startsWith('+7')?raw.slice(2):raw;
    const count=raw.slice(0,caret).replace(/\D/g,'').length;
    phone.value=format(body);
    let seen=0,pos=0;
    while(pos<phone.value.length && seen<count){if(/\d/.test(phone.value[pos]))seen++;pos++}
    phone.setSelectionRange(Math.max(2,pos),Math.max(2,pos));
  };
  phone.onpaste=e=>{
    const value=e.clipboardData.getData('text');if(value.replace(/\D/g,'').length>=10){e.preventDefault();phone.value=format(value);phone.setSelectionRange(phone.value.length,phone.value.length)}
  };
  phone.onkeydown=e=>{
    if(e.key!=='Backspace'||phone.selectionStart!==phone.selectionEnd)return;
    let p=phone.selectionStart;while(p>2 && /\D/.test(phone.value[p-1]))p--;
    if(p!==phone.selectionStart)phone.setSelectionRange(p, p);
  };
  $('book').onclick=()=>{
    if(!time)return;const s=catalog.find(row=>row.id===serviceId);if(!s)return;
    selected={serviceId,date:dateKey(date),time};
    form.reset();clearErrors();$('requestCancel').textContent='Отмена';form.hidden=false;$('requestResult').replaceChildren();$('requestResult').hidden=true;
    $('requestTitle').textContent='Оформление записи';
    $('requestDetails').replaceChildren(element('strong',s.name),element('p',[date.toLocaleDateString('ru',{dateStyle:'long'}),time,s.durationMinutes+' мин',money(s)].join(' · ')));
    dialog.showModal();form.elements.fullName.focus();
  };
  $('requestCancel').onclick=$('requestClose').onclick=()=>dialog.close();
  dialog.onclose=()=>{$('book').disabled?$('daylabel').parentElement.querySelector('button:not(:disabled)')?.focus():$('book').focus()};
  form.onsubmit=async e=>{
    e.preventDefault();clearErrors();const fullName=form.elements.fullName.value.trim();
    if(fullName.split(/\s+/).length<2){$('nameError').textContent='Укажите фамилию и имя.';form.elements.fullName.focus();return}
    const normalized='+'+phone.value.replace(/\D/g,'');
    if(!/^\+7\d{10}$/.test(normalized)){$('phoneError').textContent='Введите десять цифр после +7.';phone.focus();return}
    if(!form.elements.consent.checked){$('requestError').textContent='Подтвердите сохранение демо-данных.';form.elements.consent.focus();return}
    const submit=form.querySelector('[type=submit]');if(submit.disabled)return;submit.disabled=true;
    try{
      const b=await SiteAPI.createBookingRequest({...selected,fullName,phone:normalized});
      time='';form.hidden=true;$('requestTitle').textContent='Ваша заявка принята!';
      const result=$('requestResult');result.hidden=false;
      const soon=new Date(b.date+'T'+b.time)-Date.now()<86400000;
      const note=soon?'Специалист свяжется с вами для подтверждения записи.':'Специалист свяжется с вами по телефону для подтверждения записи, обычно за сутки до сеанса.';
      result.append(symbol('logo-03'),element('p',note),element('strong','Ожидает подтверждения'),element('p','В демо заявка хранится только в этом браузере.','note'));
      const actions=element('div');result.append(actions);CalendarEvent.render(actions,b,settings);
      const close=button('Закрыть',()=>dialog.close(),'admin-action');result.append(close);close.focus();
      await renderTime();
    }catch(error){$('requestError').textContent=error.message;if(error.code==='SLOT_UNAVAILABLE'){time='';await renderTime();$('requestCancel').textContent='Выбрать другое время'}}
    finally{submit.disabled=false}
  };
})();
