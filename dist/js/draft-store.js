// Unsaved form values are scoped to this tab. Client contact edits stay in memory.
const AdminDraftStore=(()=>{
  const key='formaTela.adminForms.1',memory=new Map();
  try{for(const [id,v] of JSON.parse(sessionStorage.getItem(key)||'[]'))memory.set(id,v)}catch{}
  function sync(){
    try{sessionStorage.setItem(key,JSON.stringify([...memory].filter(([k])=>!k.startsWith('clients/'))))}catch{}
    window.dispatchEvent(new Event('admin-drafts'));
  }
  return {
    get:k=>memory.get(k),keys:()=>[...memory.keys()],hasChanges:k=>memory.has(k),
    set(k,v){memory.set(k,v);sync()},clear(k){memory.delete(k);sync()},clearAll(){memory.clear();sync()}
  };
})();
