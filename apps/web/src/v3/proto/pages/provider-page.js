import {esc} from '../components/controls.js';
// Lazy page modules keep registry discovery side-effect free and dispose late reads.
export function providerPage(load, name, el, ctx) {
  let alive=true, dispose;
  const open=()=>{
    el.innerHTML='<div class="statebox" role="status">正在打开页面…</div>';
    void load().then(module=>{if(alive)dispose=module[name](el,ctx);}).catch(error=>{if(alive){el.innerHTML=`<div class="statebox" role="alert"><h2>暂时无法打开此页</h2><p class="muted">页面资源可能已更新或暂时无法读取。刷新后可重新打开当前页面。</p><details><summary>读取失败详情</summary><p class="small muted">${esc(error.message)}</p></details><button class="btn" data-module-retry>刷新并重新打开</button><a class="btn ghost" href="#/library">返回角色库</a></div>`;el.querySelector('[data-module-retry]').onclick=()=>location.reload();}});
  };
  open();
  return()=>{alive=false;if(typeof dispose==='function')dispose();};
}
