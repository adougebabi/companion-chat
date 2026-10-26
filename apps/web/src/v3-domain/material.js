import {studioClient} from '../v3/proto/services.js';
import {get} from '../v3/proto/store.js';
import {asyncPage} from '../v3/proto/components/async-page.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
import {studioMaterialRetention} from '../v3/proto/components/studio-material-retention.js';
import {JsonDetails} from '../v3/proto/components/records.js';
import {esc} from '../v3/proto/ui.js';
export function renderRealMaterial(el,ctx){
 const owner=get('session')?.actorId,id=ctx.params.materialId;return asyncPage(el,()=>studioClient.material(id),(material,reload,connected)=>{
  const alive=()=>connected()&&get('session')?.actorId===owner;
  el.innerHTML=`<div data-domain-ui="card" class="card stack"><h2>创作素材</h2><p>${esc(material.label)}</p><p>${material.released_at?'已取消保留':'当前保留为资源素材'} · v${material.revision}</p>${material.text?`<p class="[white-space:pre-wrap]">${esc(material.text)}</p>`:''}${material.asset_id?`<a data-domain-ui="button" class="btn" href="#/studio/media/${encodeURIComponent(material.asset_id)}">查看实际图片与引用</a>`:''}<div class="row"><button data-domain-ui="button" class="btn" data-retention>${material.released_at?'恢复保留':'取消保留'}</button><a data-domain-ui="button" class="btn" href="#/studio/edit/${encodeURIComponent(material.resource_id)}?job=${encodeURIComponent(material.job_id)}">原资源与任务</a></div>${JsonDetails({job_id:material.job_id,source_digest:material.source_digest,created_at:material.created_at},'原始来源')}</div><div data-domain-ui="card" class="card"><h3>保留状态修订</h3><p class="small muted">从本版本开始记录；历史状态不会从当前资料补造。</p><div data-history></div></div>`;
  el.querySelector('[data-retention]').onclick=()=>studioMaterialRetention(material,()=>{if(alive())return reload();});const history=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'素材状态历史',read:cursor=>studioClient.materialHistory(id,cursor),render:row=>`<div data-domain-ui="item" class="lrow">${JsonDetails(row,`v${row.revision} · ${row.action} · ${row.reason}`)}</div>`,empty:'尚无已记录的保留状态修订。'});return()=>history.dispose();
 });
}
