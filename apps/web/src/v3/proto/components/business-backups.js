import {InstantLabel} from './instant-label.js';
import { randomId } from '../../../random-id.mjs';
import {dataToolsClient} from '../services.js';
import {esc,badge} from '../ui.js';
import {Field} from './forms.js';
import {mountCursorRecords} from './cursor-records.js';
import {durableCommandDrawer} from './durable-command-drawer.js';
const labels={queued:'等待维护执行',running:'维护进行中',completed:'已完成实际核验',failed:'失败',cancelled:'已取消'};
export function BackupCard(job,{detail=false}={}){const stage=({backup_verified:'备份已核验',isolated_restore_verified:'隔离恢复已核验'})[job.stage]||'维护阶段已记录',tag=detail?'div':'a';return `<${tag} data-domain-ui="item" class="lrow replica-backup-card" ${detail?'':`href="#/data/backup/${encodeURIComponent(job.id)}"`} title="${esc(job.id)}"><span class="grow"><b>${job.kind==='restore'?'隔离恢复':'业务备份'}</b><p class="small muted">${job.created_at?InstantLabel(job.created_at)+' · ':''}第 ${esc(job.attempt)} 次 · ${esc(stage)}</p></span>${badge(labels[job.status]||'状态待确认','plain')}</${tag}>`;}
export function backupCommand(job,action,{onSaved=()=>{}}={}){
 const restore=action==='restore'||action==='retry'&&job.kind==='restore';
 return durableCommandDrawer({slot:`backup:${job.id||'new'}:${action}`,title:{create:'创建备份任务',restore:'创建全新隔离恢复任务',retry:'明确重试维护任务',cancel:'取消未开始的维护任务'}[action],body:`<p>维护工具会真实核验与归档数据卷，任务排队不表示已经备份。备份需要停止源系统写入；恢复只创建新隔离卷与网络，不覆盖源库，不启动 API/Worker 或模型。</p>${job.id?`<p class="small muted">原任务 ${esc(job.id)} · v${esc(job.revision)} · ${esc(job.status)}</p>`:''}${restore?`${Field({label:'全新的隔离目标名',name:'target',value:'fluctlight_restore_'+randomId().replaceAll('-','').slice(0,12),hint:'必须以 fluctlight_restore_ 开头；现有目标一律拒绝覆盖。',attrs:{required:true,pattern:'fluctlight_restore_[a-z0-9]{8,32}'}})}<label class="toggle"><input data-domain-ui="field" type="checkbox" name="confirm" required> 确认仅创建全新隔离副本，旧副本和源数据保留</label>`:''}${Field({label:'操作理由',name:'reason',type:'textarea',attrs:{required:true,maxlength:1000}})}`,read:el=>({reason:el.querySelector('[name=reason]').value,...(job.id?{expectedRevision:job.revision}:{}),...(restore?{targetName:el.querySelector('[name=target]').value,...(action==='restore'?{confirm:true}:{})}:{})}),execute:(body,key)=>dataToolsClient.backupCommand(job.id||'',action,{...body,idempotencyKey:key}),onSaved});
}
export function mountBusinessBackups(host,{alive=()=>true,navigate=()=>{}}={}){
 host.innerHTML='<div class="stack"><p>创建任务后，按详情页指引执行备份。完成后才能用于恢复，进度和结果会保留在记录中。</p><div class="row"><button data-domain-ui="button" class="btn primary" data-create-backup>创建备份任务</button><button data-domain-ui="button" class="btn" data-refresh-backups>刷新任务</button></div><div data-backup-list></div></div>';
 const panel=mountCursorRecords(host.querySelector('[data-backup-list]'),{alive,label:'维护任务',read:cursor=>dataToolsClient.backups(cursor),render:BackupCard,empty:'还没有备份记录。创建任务后，可在这里查看进度和结果。'});
 host.querySelector('[data-create-backup]').onclick=()=>backupCommand({},'create',{onSaved:job=>{if(alive()&&job)navigate(`#/data/backup/${encodeURIComponent(job.id)}`);}});host.querySelector('[data-refresh-backups]').onclick=()=>void panel.reload({reset:true});return {dispose:()=>panel.dispose()};
}
