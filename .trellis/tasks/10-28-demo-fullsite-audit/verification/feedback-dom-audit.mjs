// Evidence collector only. It never infers a business state from copy or CSS.
export async function feedbackDOM(page){
 return page.evaluate(()=>{
  const visible=e=>!!(e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden');
  return [...document.querySelectorAll('[data-feedback-kind], [role=alert], [role=status]')].filter(visible).map(e=>{
   const r=e.getBoundingClientRect(),s=getComputedStyle(e);
   return {kind:e.getAttribute('data-feedback-kind'),role:e.getAttribute('role'),live:e.getAttribute('aria-live'),text:e.innerText,tag:e.tagName,testId:e.getAttribute('data-testid'),className:e.className,
    bounds:{x:r.x,y:r.y,width:r.width,height:r.height},style:{color:s.color,background:s.backgroundColor,border:s.borderColor,fontSize:s.fontSize},
    actions:[...e.querySelectorAll('button,a')].filter(visible).map(a=>({text:a.innerText,label:a.getAttribute('aria-label'),href:a.getAttribute('href'),disabled:a.disabled||false,expanded:a.getAttribute('aria-expanded')}))};
  });
 });
}
export async function recordFeedbackDOM(page,record,state){
 const nodes=await feedbackDOM(page);(record.feedback||=[]).push({state,nodes});record.version=process.env.AUDIT_VERSION||'after';
 if(record.version==='after')for(const node of nodes.filter(node=>node.kind)){
  const valid=node.kind==='error'?node.role==='alert':['progress','success','warning'].includes(node.kind)?node.role==='status':['note','status'].includes(node.role);
  if(!valid)throw Error('Declared feedback kind/role mismatch: '+JSON.stringify(node));
 }
}
