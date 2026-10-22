import {get,subscribe} from '../store.js';
const readingViews=new Map();
subscribe(()=>{if(!get('session'))readingViews.clear();});
import { CursorPager,bindCursorPager } from './cursor-pager.js';
import { esc } from './controls.js';

// Independent read-only panels retain their own cursor and response epoch.
// A filter/scope change calls reload({reset:true}); it never edits domain data.
export function mountCursorRecords(box,{read,render,alive=()=>true,bind=()=>{},label='记录',empty='尚无记录',scope=null}={}) {
  const key=()=>scope?JSON.stringify([get('session')?.actorId,typeof scope==='function'?scope():scope]):'';
  const restored=key()?readingViews.get(key()):null;const cursors=restored?[...restored.cursors]:[''];let at=restored?.at||0,epoch=0,disposed=false,viewKey=key(),lastPage=restored?.page||null,lastAt=restored?.at||0;
  const remember=()=>{if(viewKey){readingViews.set(viewKey,{cursors:[...cursors],at:lastAt,page:lastPage});if(readingViews.size>40)readingViews.delete(readingViews.keys().next().value);}};
  function paint(page){box.innerHTML=page.items.map(render).join('')||`<p class="small muted">${esc(empty)}</p>`;box.insertAdjacentHTML('beforeend',CursorPager({more:!!page.next_cursor,previous:at>0,label}));bindCursorPager(box,{next:()=>{cursors[++at]=page.next_cursor;void load();},previous:()=>{at--;void load();}});bind(box,page);}
  const current=()=>!disposed&&alive()&&box.isConnected;
  async function load() {
    const version=++epoch,requestedAt=at;
    if(!lastPage)box.innerHTML='<p role="status">正在读取…</p>';
    try {
      const page=await read(cursors[at]);if(!current()||version!==epoch)return;
      lastPage=page;lastAt=at;remember();paint(page);
    } catch(error) {
      if(!current()||version!==epoch)return;
      if(lastPage){at=lastAt;paint(lastPage);}
      box.querySelector('[data-panel-failure]')?.remove();box.insertAdjacentHTML('beforeend',`<div data-panel-failure><p class="banner err" role="alert">${esc(error.message)}</p><button class="btn" data-panel-retry>重新读取</button></div>`);
      box.querySelector('[data-panel-retry]').onclick=()=>{at=requestedAt;void load();};
    }
  }
  if(lastPage)paint(lastPage);void load();
  return {reload({reset=false,onlyIfFirst=false}={}){if(onlyIfFirst&&at>0)return Promise.resolve();if(reset){remember();viewKey=key();cursors.splice(0,cursors.length,'');at=0;lastAt=0;lastPage=null;}return load();},dispose(){remember();disposed=true;epoch++;}};
}
