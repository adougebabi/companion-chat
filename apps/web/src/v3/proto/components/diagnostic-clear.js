import {errorMessage} from '../../domain-presentation.mjs';
import {get} from '../store.js';
import {diagnosticsClient} from '../services.js';
import {drawer,esc,confirm} from '../ui.js';
import {Field} from './forms.js';
import {JsonDetails} from './records.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';

export function diagnosticClear(page,onSaved=()=>{}){
 const owner=get('session')?.actorId,slot=`fluctlight-studio:${owner}:diagnostic-clear`,storage=studioDraftStorage();let active=true,busy=false;
 const command=persistentCommand({key:slot+':command',storage:studioDraftStorage,execute:(body,key)=>{if(get('session')?.actorId!==owner)throw new Error('会话已切换');return diagnosticsClient.clear({...body,idempotencyKey:key});}});
 const d=drawer({title:'清理诊断记录',body:`<p>仅清理所选诊断表中本次快照已可见且早于截止时刻的记录。角色内容、记忆、Agent执行围栏、媒体来源和操作审计保留。清理无法恢复，请先按需导出。</p><p class="small muted">截止 ${esc(page.as_of)}</p>${JsonDetails(page.filters,'本次实际筛选')}${['events','model-runs','turns','links'].map((kind,i)=>`<label class="toggle"><input type="checkbox" data-kind="${kind}" ${i<2?'checked':''}>${{events:'诊断事件','model-runs':'物理模型诊断',turns:'回合诊断',links:'工作流诊断链接'}[kind]}</label>`).join('')}${Field({label:'清理理由',name:'reason',type:'textarea',attrs:{required:true,maxlength:1000}})}<p role="alert" data-error></p><div data-pending></div>`,footer:'<button class="btn" data-close>关闭</button><button class="btn" data-retry>重试原清理</button><button class="btn danger" data-clear>确认清理</button>',onClose:()=>{active=false;}});
 const alive=()=>active&&d.el.isConnected&&get('session')?.actorId===owner;
 function paint(){if(!alive())return;const pending=command.pending();d.el.querySelector('[data-clear]').disabled=busy||!!pending||!d.el.querySelector('[name=reason]').value.trim()||!d.el.querySelector('[data-kind]:checked');d.el.querySelector('[data-retry]').hidden=!pending;d.el.querySelector('[data-retry]').disabled=busy||!pending;d.el.querySelector('[data-pending]').textContent=pending?'原清理结果待核对，重试保持原快照、范围和身份。':'';}
 async function save(retry){if(busy||!alive()||!retry&&!d.el.querySelector('[name=reason]').reportValidity())return;if(!retry&&!await confirm({title:'不可撤销的诊断清理',type:'danger',text:'仅删除所选诊断表在本次快照中的记录；已导出的文件和领域历史保留。确认执行？'}))return;if(!alive())return;busy=true;paint();try{const body={datasets:[...d.el.querySelectorAll('[data-kind]:checked')].map(e=>e.dataset.kind),before:page.as_of,snapshot:page.snapshot,filters:page.filters||{},confirm:true,reason:d.el.querySelector('[name=reason]').value};const result=await(retry?command.retry():command.submit(body));if(alive()){d.el.querySelector('[data-error]').textContent='实际清理 '+result.cleared+' 条；'+JSON.stringify(result.counts);await onSaved(result);}}catch(e){if(alive())d.el.querySelector('[data-error]').textContent=errorMessage(e);}finally{busy=false;paint();}}
 d.el.querySelector('[name=reason]').addEventListener('input',paint);d.el.querySelectorAll('[data-kind]').forEach(input=>input.addEventListener('change',paint));
 d.el.querySelector('[data-close]').onclick=()=>d.close();d.el.querySelector('[data-clear]').onclick=()=>void save(false);d.el.querySelector('[data-retry]').onclick=()=>void save(true);paint();return d;
}
