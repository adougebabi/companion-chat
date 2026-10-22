import {errorMessage} from '../../domain-presentation.mjs';
import {CreationPreview,bindCreationPreview} from '../components/creation-preview.js';
import {finishCreationWorld} from '../../creation-world.mjs';
import {creationFeedback} from '../../creation-feedback.mjs';
import {readCreationProgress} from '../../initialization-progress.mjs';
import {randomId} from '../../../random-id.mjs';
import * as store from '../store.js';
import {directory,activationFromAnalysis,client} from '../../providers/real.ts';
import {esc,requireOwner,drawer,toast} from '../ui.js';
import {Field} from '../components/forms.js';
import {JsonDetails} from '../components/records.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {studioDraftStorage} from '../components/studio-draft-storage.js';
import {parseCreationAnalysis,parseCreationInput,creationAnalysisRequest} from '../../creation-json.mjs';
import {creationDraft,checkpointCreation,selectCreationWorld} from '../../creation-draft.mjs';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {localDate} from '../components/calendar-reading.js';

export function renderRealCreate(el,ctx){
 const owner=store.get('session')?.actorId;
 if(!owner){el.innerHTML='<p>请先登录所有者会话。</p><a href="#/auth">登录</a>';return;}
 const storage=studioDraftStorage(),recovery=creationDraft({owner,storage});
 let active=true,busy=false,epoch=0,picker=null,worldCursor='',timer=null,reads=0,progress=null,error='',draft=recovery.read();
 const live=()=>active&&el.isConnected&&store.get('session')?.actorId===owner;
 const command=persistentCommand({key:recovery.commandKey,storage:studioDraftStorage,preAdmissionConflicts:['activation_analysis_stale'],execute:async value=>{
  if(!live())throw new Error('所有者会话已切换，请回原范围确认创建请求');
  const result=await directory.activate(value.body);checkpointCreation(recovery,value,result,()=>store.get('session')?.actorId===owner);return result;
 }});
 const save=()=>{if(live())recovery.save(draft);};
 if(draft.wizardVersion!==2){draft.wizardVersion=2;draft.wizardStep=draft.activatedId||command.pending()?2:draft.worldResourceId?1:0;draft.modeDrafts={};}
 if(selectCreationWorld(draft,{resourceId:ctx.query.worldResourceId,version:ctx.query.worldVersion,pending:!!command.pending()}))draft.wizardStep=draft.activatedId?2:1;
 if(!draft.activatedId&&!command.pending()&&ctx.query.template&&ctx.query.version){draft.mode='template';draft.templateId=ctx.query.template;draft.templateVersion=ctx.query.version;draft.wizardStep=draft.worldResourceId?1:0;}
 draft.modeDrafts||={};if(draft.mode==='json'&&draft.json&&!draft.jsonValidation){try{parseCreationInput(draft.json);draft.jsonValid=true;draft.jsonValidation='已恢复并校验原 JSON 草稿';}catch(cause){draft.jsonValid=false;draft.jsonValidation=cause.message;}}save();
 const invalidate=()=>{draft.templateReviewed=false;draft.analysis=null;draft.analysisText='';draft.requestId=randomId();save();};
 const fail=cause=>{error=creationFeedback(cause).message;paint();};
 function capture(form){for(const name of ['name','bio','traits','world','groupId','timezone','json'])if(form.elements[name])draft[name]=form.elements[name].value;save();}
 function modeFields(){
  if(draft.mode==='json')return `<a class="btn" href="/templates/yaoguang-character-init-template.v2.json" download>下载 JSON 模板</a><label class="field">选择或更换 JSON 文件<input class="input" type="file" accept="application/json,.json" data-json-file></label><p class="small muted">${esc(draft.fileName||'尚未选择文件')}</p>${draft.json?`<p>${esc(draft.jsonValidation||'等待校验')}</p><details><summary>查看导入内容</summary><pre>${esc(draft.json)}</pre></details>`:''}`;
  if(draft.mode==='template')return `<p>使用精确角色模板 ${esc(draft.templateId)} · ${esc(draft.templateVersion)}</p><p class="small muted">这是从创作资源继续的模板分支。</p>${draft.analysis?CreationPreview(draft):''}`;
  if(draft.mode==='analysis')return Field({label:'当前所有者的分析预览',name:'json',value:draft.json,type:'textarea',rows:8});
  return draft.mode==='description'?Field({label:'一段完整描述',name:'bio',value:draft.bio,type:'textarea',rows:8,attrs:{required:true},hint:'身份、性格、背景和生活可以在同一段描述中说明。'}):'<p class="small muted">从姓名开始，保留空白人物的初始化语义；经历与认识在后续真实互动中形成。</p>';
 }
 function paint(){
  if(!live())return;picker?.dispose();picker=null;clearTimeout(timer);
  const pending=command.pending(),locked=busy||!!pending||!!draft.activatedId;
  if(draft.activatedId||pending)draft.wizardStep=progress?.complete?3:2;
  draft.step=draft.wizardStep;save();
  const step=draft.wizardStep;
  let body='';
  if(step===0)body=`<h2 class="page-title">选择世界</h2><p>世界是她生活的背景、地点和规则。选择一份已发布的世界设定，新角色会拥有自己的生活世界。</p><div data-world-picker></div><a class="btn primary" href="#/studio/edit/new?kind=world">创建世界设定</a><p class="small muted">填写背景和规则，保存后发布，再用「以此世界设定继续人物初始化」返回。当前人物草稿会保留。</p><a class="btn" href="#/studio?type=${encodeURIComponent('世界')}">管理世界资源</a>`;
  else if(step===1)body=`<h2 class="page-title">选择创建方式</h2><p class="small muted">已选世界：${esc(draft.worldName||draft.worldResourceId)} · ${esc(draft.worldResourceVersion)}</p>${['template','analysis'].includes(draft.mode)?'':`<div class="creation-choices">${[['blank','空白创建','从姓名开始'],['description','一段描述','分析并整理你的描述'],['json','JSON 导入','校验结构化人物定义']].map(([value,label,note])=>`<button type="button" class="creation-choice ${draft.mode===value?'on':''}" data-mode="${value}" aria-pressed="${draft.mode===value}"><b>${label}</b><span>${note}</span></button>`).join('')}</div>`}<form class="stack" data-source>${Field({label:'姓名',name:'name',value:draft.name,attrs:{required:true,maxlength:200}})}${modeFields()}<details><summary>分组与生活时区</summary>${Field({label:'角色分组',name:'groupId',type:'select',value:draft.groupId,options:[{value:'',label:'未分组'},...store.list('groups').filter(g=>g.id!=='ungrouped').map(g=>({value:g.id,label:g.name}))]})}${Field({label:'初始生活时区',name:'timezone',value:draft.timezone,attrs:{required:true}})}</details></form>`;
  else if(step===2){
   const stages=progress?.stages||[{title:'分析与整理设定',state:draft.activatedId||draft.analysis||draft.mode==='blank'?'ready':busy?'running':'unknown',note:busy?'正在处理本次创建资料':'资料与原请求已保留'},...['初始化设定图','初始化头像','初始化日程'].map(title=>({title,state:'pending',note:'等待人物资料确认'}))],done=stages.filter(s=>s.state==='ready').length;
   body=`<h2 class="page-title">正在初始化${esc(draft.name||'人物')}</h2><p>${done}/4 项已确认完成</p><progress class="creation-progress" max="4" value="${done}" aria-label="已确认完成的初始化项目"></progress><ol class="preparation-steps">${stages.map(stage=>`<li class="preparation-step"><div class="grow"><b>${esc(stage.title)}</b><p class="small muted">${esc(stage.note||'')}</p>${stage.href?`<a href="${esc(stage.href)}">查看原任务与结果</a>`:''}</div><span>${esc({ready:'完成',running:'处理中',pending:'等待',unknown:'状态待确认',failed:'需要处理'}[stage.state]||'状态待确认')}</span></li>`).join('')}</ol>${draft.activatedId?`<p class="small muted">世界：${progress?.worldConfirmed?'已读回所选独立世界':'仍待确认；已有角色和请求身份保留'}</p><div class="row"><button class="btn" data-refresh-progress>重新读取进度</button><button class="btn ghost" data-world-recover>继续原世界与分组步骤</button><button class="btn ghost" data-read-settings>查看已整理的设定</button></div>`:pending?'<p>原激活结果尚未确认，请查询原提交。</p>':'<button class="btn" data-start-again>继续本次初始化</button>'}`;
  }else body=`<h2 class="page-title">初始化完成</h2><p>${esc(draft.name)} · ${esc(draft.worldName||'所选世界')}</p><p>4/4 项已确认完成，独立世界已读回。</p><div class="row"><a class="btn primary" href="#/c/${encodeURIComponent(draft.activatedId)}/chat">开始相处</a><a class="btn" href="#/c/${encodeURIComponent(draft.activatedId)}/profile">查看角色</a><button class="btn ghost" data-new-creation>开始新的创建草稿</button></div>`;
  el.innerHTML=`<div class="creation-flow stack"><div class="wizsteps">${['选择世界','选择方式','初始化进度','完成'].map((label,i)=>`<span class="${step===i?'on':''}" ${step===i?'aria-current="step"':''}>${i+1}. ${label}</span>`).join('')}</div><section class="card stack creation-main">${body}<p role="alert" data-error>${esc(error)}</p>${pending?`<div class="banner info">原创建请求已保留，当前内容不会创建第二个人物。${JsonDetails(pending.value,'查看原提交')}<button class="btn" data-original ${busy||pending.rejection?'disabled':''}>查询或重试原激活</button>${pending.rejection?'<button class="btn" data-reanalyze>保留输入并重新整理</button>':''}</div>`:''}<div class="row"><button class="btn ghost" data-return>保留草稿并返回</button>${step===0?`<button class="btn primary" data-world-next ${draft.worldResourceId&&draft.worldResourceVersion?'':'disabled'}>下一步</button>`:''}${step===1?`<button class="btn" data-prev ${locked?'disabled':''}>上一步</button><button class="btn primary" data-start ${locked||draft.mode==='json'&&!draft.jsonValid?'disabled':''}>${draft.mode==='template'?(draft.templateReviewed?'确认并激活':'解析并审阅模板'):'开始初始化'}</button>`:''}</div></section></div>`;
  if(step===0){picker=mountCursorRecords(el.querySelector('[data-world-picker]'),{alive:live,label:'已发布世界',read:cursor=>{worldCursor=cursor;return client.listStudioResources({kind:'world',published:true,cursor});},render:w=>`<button class="card world-choice ${w.id===draft.worldResourceId&&w.published_version===draft.worldResourceVersion?'on':''}" data-world="${esc(w.id)}" data-version="${esc(w.published_version)}" data-name="${esc(w.name)}"><b>${esc(w.name)}</b><span class="small muted">发布版本 ${esc(w.published_version)}</span></button>`,empty:'还没有已发布的世界设定。请先点击下方「创建世界设定」，填写并发布后继续。',bind:(box,page)=>{if(!worldCursor&&!draft.worldResourceId&&!command.pending()&&!draft.activatedId&&page.items.length===1&&!page.next_cursor){const w=page.items[0];if(selectCreationWorld(draft,{resourceId:w.id,version:w.published_version})){draft.worldName=w.name;save();box.querySelector('[data-world]')?.classList.add('on');el.querySelector('[data-world-next]').disabled=false;}}box.querySelectorAll('[data-world]').forEach(button=>button.onclick=()=>{selectCreationWorld(draft,{resourceId:button.dataset.world,version:button.dataset.version,pending:!!command.pending()});draft.worldName=button.dataset.name;save();paint();});}});}
  el.querySelector('[data-world-next]')?.addEventListener('click',()=>{if(!draft.worldResourceId||!draft.worldResourceVersion)return;draft.wizardStep=1;save();paint();});
  el.querySelector('[data-return]').onclick=()=>{save();toast('创建草稿已保留，稍后可从角色库的创建进度卡继续','ok');ctx.navigate('#/library');};
  el.querySelector('[data-prev]')?.addEventListener('click',()=>{draft.wizardStep=0;save();paint();});
  const form=el.querySelector('[data-source]');if(form){if(draft.mode==='template'&&draft.analysis)bindCreationPreview(form,draft,{changed:()=>{draft.templateReviewed=true;save();},error:cause=>{error=typeof cause==='string'?cause:cause?.message||'';}});form.querySelectorAll('input,textarea,select,button').forEach(n=>n.disabled=locked);form.oninput=e=>{capture(form);if(!['groupId','timezone','analysisText','includeGoals'].includes(e.target.name))invalidate();};form.onsubmit=e=>{e.preventDefault();void start();};}
  el.querySelectorAll('[data-mode]').forEach(button=>{button.disabled=locked;button.onclick=()=>{if(form)capture(form);draft.modeDrafts[draft.mode]={name:draft.name,bio:draft.bio,json:draft.json,fileName:draft.fileName,jsonValid:draft.jsonValid,jsonValidation:draft.jsonValidation};draft.mode=button.dataset.mode;Object.assign(draft,{name:'',bio:'',json:'',fileName:'',jsonValid:false,jsonValidation:''},draft.modeDrafts[draft.mode]);invalidate();paint();};});
  el.querySelector('[data-json-file]')?.addEventListener('change',async e=>{const file=e.target.files?.[0];if(!file)return;const token=++epoch;try{if(file.size>60000)throw new Error('JSON 文件超过 60000 UTF-8 字节上限');const text=await file.text();if(!live()||token!==epoch)return;const value=parseCreationInput(text);draft.json=text;draft.fileName=file.name;draft.jsonValid=true;draft.jsonValidation='文件校验通过';const name=value.core_persona?.identity?.name;if(typeof name==='string')draft.name=name;invalidate();error='';paint();}catch(cause){draft.jsonValid=false;draft.fileName=file.name;draft.jsonValidation=cause.message;save();fail(cause);}});
  el.querySelector('[data-start]')?.addEventListener('click',()=>void start());el.querySelector('[data-start-again]')?.addEventListener('click',()=>void start());
  el.querySelector('[data-original]')?.addEventListener('click',()=>void activate(true));
  el.querySelector('[data-reanalyze]')?.addEventListener('click',()=>{command.releaseRejected();draft.wizardStep=1;invalidate();error='原输入已保留，请重新整理后继续。';paint();});
  el.querySelector('[data-refresh-progress]')?.addEventListener('click',()=>{reads=0;void queryProgress();});
  el.querySelector('[data-world-recover]')?.addEventListener('click',()=>void finishWorld());
  el.querySelector('[data-read-settings]')?.addEventListener('click',()=>drawer({title:'本次已整理的设定',body:JsonDetails(draft.analysis||{name:draft.name,mode:'blank_slate'},'本次资料（只读）')}));
  if(draft.activatedId&&!progress?.complete&&!busy&&!document.hidden&&reads<60)timer=setTimeout(queryProgress,5000);
  el.querySelector('[data-new-creation]')?.addEventListener('click',()=>{if(progress?.complete){recovery.clear();draft=recovery.read();draft.wizardVersion=2;draft.wizardStep=0;progress=null;save();paint();}});
 }
 async function start(){
  if(busy||!live())return;if(draft.activatedId){void queryProgress();return;}if(command.pending()){void activate(true);return;}
  const form=el.querySelector('[data-source]');if(form){if(!form.reportValidity())return;capture(form);}
  try{if(!draft.worldResourceId||!draft.worldResourceVersion)throw new Error('请先选择已发布的世界');if(!draft.name.trim())throw new Error('请填写姓名');new Intl.DateTimeFormat('zh-CN',{timeZone:draft.timezone}).format();if(draft.mode==='json'&&!draft.jsonValid)throw new Error('请先选择并校验 JSON 文件');
   busy=true;error='';draft.wizardStep=2;save();paint();
   if(draft.mode==='analysis')draft.analysis=parseCreationAnalysis(draft.json);
   else if(draft.mode!=='blank'&&!draft.analysis){const request=creationAnalysisRequest(draft);if(request.description){const world=await directory.publication(draft.worldResourceId,draft.worldResourceVersion);request.description+='\n所选世界设定：'+JSON.stringify({name:world.name,body:world.body});if(new TextEncoder().encode(request.description).length>60000)throw new Error('人物描述与所选世界内容合计超过 60000 UTF-8 字节，请缩短描述或世界设定');}const value=await directory.analyze(request);if(!live())return;draft.analysis=parseCreationAnalysis(JSON.stringify(value));draft.analysisText=JSON.stringify(draft.analysis,null,2);save();}
   if(draft.mode==='template'&&!draft.templateReviewed){draft.templateReviewed=true;draft.wizardStep=1;save();busy=false;paint();return;}
   busy=false;await activate(false);
  }catch(cause){busy=false;fail(cause);}
 }
 async function activate(retry){
  if(busy||!live()||!(await requireOwner()))return;busy=true;error='';paint();
  try{draft=recovery.read();if(!draft.activatedId){const value=retry?command.pending()?.value:{body:draft.mode==='blank'?{requestId:draft.requestId,initializationMode:'blank_slate',name:draft.name,initializationTimezone:draft.timezone}:activationFromAnalysis(draft.includeGoals===false?{...draft.analysis,initial_goals:[],initial_intentions:[]}:draft.analysis,draft.name,draft.requestId,draft.timezone),groupId:draft.groupId,worldResourceId:draft.worldResourceId,worldResourceVersion:draft.worldResourceVersion};if(!value)throw new Error('原提交身份尚未读到，请保留草稿核对');const result=retry?await command.retry():await command.submit(value);if(!live())return;draft=recovery.read();draft.activatedId=result.id;draft.activationGroupId=value.groupId;draft.activationDate=localDate(new Date().toISOString(),draft.timezone);save();}
   busy=false;await finishWorld();
  }catch(cause){busy=false;fail(cause);}finally{busy=false;if(live())paint();}
 }
 async function finishWorld(){
  if(busy||!draft.activatedId||!live())return;busy=true;error='';paint();
  try{if(!draft.groupConfirmed){await directory.move(draft.activatedId,draft.activationGroupId||'');draft.groupConfirmed=true;save();}const result=await finishCreationWorld({client,recovery,storage,owner});if(!live())return;draft=recovery.read();if(!result.configured)error=result.warning;}
  catch(cause){error='人物已经建立，原世界步骤仍可继续：'+cause.message;}
  finally{busy=false;if(live())await queryProgress();}
 }
 async function queryProgress(){
  if(!draft.activatedId||!live()||busy)return;busy=true;const token=++epoch;clearTimeout(timer);
  try{const value=await readCreationProgress(client,draft.activatedId,{localDate:draft.activationDate||localDate(new Date().toISOString(),draft.timezone),worldId:draft.worldRuntimeId});if(!live()||token!==epoch)return;value.complete=value.complete&&!!draft.groupConfirmed;progress=value;draft.wizardStep=value.complete?3:2;save();}
  catch(cause){if(live())error='进度读取失败，原人物与任务保留：'+cause.message;}
  finally{busy=false;if(live()){paint();reads++;}}
 }
 const visibility=()=>{clearTimeout(timer);if(!document.hidden&&draft.activatedId&&reads<60)void queryProgress();};document.addEventListener('visibilitychange',visibility);
 paint();if(draft.activatedId)void queryProgress();
 return()=>{active=false;epoch++;clearTimeout(timer);picker?.dispose();document.removeEventListener('visibilitychange',visibility);};
}
