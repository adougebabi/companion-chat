import { Button } from './forms.js';
export function CursorPager({more=false,previous=false,label='记录'}={}) {
  return `<div class="row">${previous?Button({label:'上一页',attrs:{'data-page-previous':true}}):''}${more?Button({label:`下一页${label}`,attrs:{'data-page-next':true}}):''}</div>`;
}
export function bindCursorPager(el,{next,previous}={}) {
  if(next)el.querySelector('[data-page-next]')?.addEventListener('click',next);
  if(previous)el.querySelector('[data-page-previous]')?.addEventListener('click',previous);
}
