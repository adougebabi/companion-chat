import { CursorPager,bindCursorPager } from './cursor-pager.js';
import { esc } from './controls.js';

// Independent read-only panels retain their own cursor and response epoch.
// A filter/scope change calls reload({reset:true}); it never edits domain data.
export function mountCursorRecords(box,{read,render,alive=()=>true,bind=()=>{},label='记录',empty='尚无记录'}={}) {
  const cursors=[''];let at=0,epoch=0,disposed=false;
  const current=()=>!disposed&&alive()&&box.isConnected;
  async function load() {
    const version=++epoch;
    box.innerHTML='<p role="status">正在读取…</p>';
    try {
      const page=await read(cursors[at]);if(!current()||version!==epoch)return;
      box.innerHTML=page.items.map(render).join('')||`<p class="small muted">${esc(empty)}</p>`;
      box.insertAdjacentHTML('beforeend',CursorPager({more:!!page.next_cursor,previous:at>0,label}));
      bindCursorPager(box,{next:()=>{cursors[++at]=page.next_cursor;void load();},previous:()=>{at--;void load();}});
      bind(box,page);
    } catch(error) {
      if(!current()||version!==epoch)return;
      box.innerHTML=`<p class="banner err" role="alert">${esc(error.message)}</p><button class="btn" data-panel-retry>重新读取</button>`;
      box.querySelector('[data-panel-retry]').onclick=()=>void load();
    }
  }
  void load();
  return {reload({reset=false,onlyIfFirst=false}={}){if(onlyIfFirst&&at>0)return Promise.resolve();if(reset){cursors.length=1;cursors[0]='';at=0;}return load();},dispose(){disposed=true;epoch++;}};
}
