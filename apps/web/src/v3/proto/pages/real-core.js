import {errorMessage} from '../../domain-presentation.mjs';
import { randomId } from '../../../random-id.mjs';
import * as store from '../store.js';
import { auth, directory } from '../../providers/real.ts';
import { esc, toast, requireOwner, drawer, confirm } from '../ui.js';
import { Tabs, Field } from '../components/forms.js';
import {CharacterCard} from '../components/character-card.js';
import {bindAvatars} from '../components/avatar.js';
import {creationDraft} from '../../creation-draft.mjs';
import {studioDraftStorage} from '../components/studio-draft-storage.js';
export {renderRealCreate} from './real-create.js';

export function renderRealAuth(el,ctx) {
  let alive = true;
  const showError=(target,error)=>{target.textContent=errorMessage(error);const detail=document.createElement('details'),summary=document.createElement('summary'),pre=document.createElement('pre');summary.textContent='技术详情';pre.textContent=String(error?.message||error);detail.append(summary,pre);target.append(detail);};
  const session = store.get('session');
  if (session) {
    el.innerHTML = '<div class="card stack" style="max-width:420px;margin:auto"><p>已登录所有者会话</p><a class="btn primary" href="#/library">进入角色库</a><button class="btn" data-out>退出</button><p data-result role="alert"></p></div>';
    el.querySelector('[data-out]').onclick = async () => { try { await auth.logout(); ctx.navigate('#/auth'); ctx.refresh(); } catch(error) { if(alive) showError(el.querySelector('[data-result]'),error); } };
  } else {
    el.innerHTML = '<div class="card" style="max-width:420px;margin:auto" role="status">正在读取登录配置…</div>';
    void auth.setupStatus().then(status => {
      if (!alive) return;
      el.innerHTML = `<form class="card stack" style="max-width:420px;margin:auto"><h2>${status.setupAvailable ? '设置所有者账户' : '所有者登录'}</h2>${status.setupAvailable ? Field({label:'初始化令牌',name:'token',type:'password',hint:'由部署者在服务器生成并提供，仅用于首次设置账户；拿不到令牌时请联系部署者。',attrs:{required:true,autocomplete:'off'}}) : ''}${Field({label:'密码',name:'password',type:'password',hint:status.setupAvailable?'至少 12 个字符；请妥善保存。':'请输入创建账户时设置的密码。',attrs:{required:true,autocomplete:status.setupAvailable?'new-password':'current-password',minlength:12}})}${status.setupAvailable ? Field({label:'确认密码',name:'confirmPassword',type:'password',attrs:{required:true,autocomplete:'new-password',minlength:12}}) : ''}<button class="btn primary" data-test="auth-submit">${status.setupAvailable?'设置并登录':'登录'}</button><p data-result role="alert"></p></form>`;
      el.querySelector('form').onsubmit = async event => { event.preventDefault(); const form=event.target, button=form.querySelector('button'); if(status.setupAvailable&&form.elements.password.value!==form.elements.confirmPassword.value){form.querySelector('[data-result]').textContent='两次输入的密码不一致，请重新核对。';form.elements.confirmPassword.focus();return;} button.disabled=true; try { if(status.setupAvailable) await auth.setup(form.elements.token.value,form.elements.password.value); else await auth.login(form.elements.password.value); if(alive) ctx.navigate('#/library'); } catch(error) { if(alive) showError(form.querySelector('[data-result]'),error); } finally { button.disabled=false; } };
    }).catch(error => { if(alive){el.innerHTML='<div class="statebox" role="alert"><p data-error></p><button class="btn" data-retry>重新读取登录配置</button></div>';showError(el.querySelector('[data-error]'),error);el.querySelector('[data-retry]').onclick=()=>ctx.refresh();} });
  }
  return () => {alive=false;};
}
export function renderRealLibrary(el,ctx) {
  const search=ctx.query.q||'',filter=ctx.query.s||'全部', groups=store.list('groups'), characters=store.list('characters');
  const knownGroups=new Set(groups.map(g=>g.id));
  const displayGroups=groups.some(g=>g.id==='ungrouped')?groups:[...groups,{id:'ungrouped',name:'未分组'}];
  let draft=null,draftError='',onboardingSeen=false;
  try{const owner=store.get('session')?.actorId,storage=studioDraftStorage();onboardingSeen=!!storage.getItem(`fluctlight-setup:${owner}:home-guide-dismissed`);if(owner&&storage.getItem(`fluctlight-creation:${owner}:draft`))draft=creationDraft({owner,storage}).read();}catch{draftError='创建草稿暂时无法读取，请从新建角色页面核对并继续。';}
  const creation=draft&&draft.wizardStep!==3&&(draft.name||draft.activatedId||draft.wizardStep>0)?`<section class="card stack" aria-label="正在创建的角色"><h3>${esc(draft.name||'新角色')} · ${draft.wizardStep===2?'正在诞生':'创建草稿'}</h3><p class="small muted">创建资料和处理进度已保留，点击可继续查看。</p><a class="btn primary" href="#/library/create">继续创建与查看进度</a></section>`:'';
  const onboarding=!characters.length||!onboardingSeen?`<section class="card stack" aria-label="首次使用引导" data-home-guide><h3>从这里开始相识</h3><ol><li><a href="#/settings/models">连接模型</a>，让角色能够回应与思考。</li><li><a href="#/library/create">描述你想认识的人</a>，会为角色准备日常生活世界。</li><li>资料准备好后，打开对话开始相处。</li></ol><button class="btn ghost" data-dismiss-guide>我知道了</button></section>`:'';
  const counts=status=>characters.filter(c=>status==='全部'||c.status===status).length;
  el.innerHTML=`${onboarding}${creation}${draftError?`<p role="alert">${esc(draftError)}</p>`:''}<div class="row library-toolbar"><details class="library-organize"><summary>整理角色</summary><div class="row" aria-label="按角色状态筛选">${['全部','正常','异常','已暂停','已退休','未启动'].map(s=>`<a class="chip ${filter===s?'on':''}" ${filter===s?'aria-current="page"':''} href="#/library?s=${encodeURIComponent(s)}">${s} <span>${counts(s)}</span></a>`).join('')}</div><button class="btn" data-group>新建分组</button></details><a class="btn primary" href="#/library/create" data-test="lib-create">＋ 认识新的人</a></div><form class="row" data-library-search><label class="field grow">搜索角色<input class="input" name="q" value="${esc(search)}" placeholder="按名称搜索当前角色库"></label><button class="btn">搜索</button>${search?'<a class="btn" href="#/library">清除搜索</a>':''}</form>${!characters.length?'<div class="statebox"><h2>还没有角色</h2><p>连接模型后，描述你想认识的人，就可以开始创建。</p><a class="btn primary" href="#/library/create">开始创建</a></div>':''}${displayGroups.map(group=>{const rows=characters.filter(c=>(c.groupId===group.id||group.id==='ungrouped'&&!knownGroups.has(c.groupId))&&(filter==='全部'||c.status===filter)&&(!search||c.name.toLocaleLowerCase().includes(search.toLocaleLowerCase())));return `<section class="library-group"><h3 class="row"><span class="grow">${esc(group.name)} <span class="small muted">${rows.length}</span></span>${group.id!=='ungrouped'?`<button class="btn ghost sm" data-remove-group="${esc(group.id)}">删除分组</button>`:''}</h3>${rows.length?`<div class="library-grid">${rows.map(c=>CharacterCard(c,groups)).join('')}</div>`:characters.length?'<p class="small muted">此分组下没有符合当前筛选的角色。</p>':''}</section>`;}).join('')}<p role="alert" data-error></p>`;
  el.querySelector('[data-library-search]').onsubmit=event=>{event.preventDefault();ctx.navigate(`#/library?s=${encodeURIComponent(filter)}&q=${encodeURIComponent(event.target.elements.q.value.trim())}`);};
  bindAvatars(el);
  el.querySelector('[data-dismiss-guide]')?.addEventListener('click',()=>{studioDraftStorage().setItem(`fluctlight-setup:${store.get('session')?.actorId}:home-guide-dismissed`,'true');el.querySelector('[data-home-guide]')?.remove();});
  el.querySelectorAll('[data-remove-group]').forEach(button=>button.onclick=async()=>{if(!(await requireOwner())||!(await confirm({title:'删除分组',text:'删除分组，不会删除角色或历史。',okText:'删除分组'})))return;button.disabled=true;try{await directory.removeGroup(button.dataset.removeGroup,randomId());}catch(error){el.querySelector('[data-error]').textContent=error.message;button.disabled=false;}});
  el.querySelectorAll('[data-lifecycle]').forEach(button=>button.onclick=async()=>{
    if(!(await requireOwner()))return;
    const id=button.dataset.lifecycle, character=store.get('characters',id), detail=store.get('meta').details?.[id];
    if(!Number.isSafeInteger(detail?.current_revision))return toast('当前资料版本缺失，请重新读取角色库后再试','err');
    const d=drawer({title:`${esc(character.name)} · 生命周期`,body:`<p>当前状态：${esc(character.status)}</p><p class="small muted">暂停会停止继续运行，可再次恢复；退休会停止运行并保留历史归档，退休后不能恢复。</p>${Field({label:'修改原因',name:'reason',type:'textarea',attrs:{required:true,maxlength:1000,placeholder:'例如：暂时停止运行，稍后继续'}})}<p data-error role="alert"></p>`,footer:`<button class="btn" data-close>取消</button><button class="btn" data-status="${character.status==='已暂停'?'active':'paused'}">${character.status==='已暂停'?'恢复运行':'暂停运行'}</button><button class="btn danger" data-status="retired">退休并保留归档</button>`});
    d.el.querySelector('[data-close]').onclick=()=>d.close();
    d.el.querySelectorAll('[data-status]').forEach(control=>control.onclick=async()=>{const reason=d.el.querySelector('[name=reason]').value.trim();if(!reason){d.el.querySelector('[data-error]').textContent='请填写修改原因';return;}const retiring=control.dataset.status==='retired',pausing=control.dataset.status==='paused';if(!(await confirm({title:retiring?'退休角色':pausing?'暂停运行':'恢复运行',text:retiring?`停止${esc(character.name)}继续运行并保留历史归档。退休后不能恢复运行。取消会保留当前原因。`:pausing?'暂停后角色停止继续运行，历史资料与已保存内容保留，可从角色库恢复。':'恢复后角色将继续运行，历史资料保持。',okText:retiring?'确认退休':pausing?'确认暂停':'确认恢复',...(retiring?{type:'danger'}:{})})))return;d.el.querySelectorAll('button').forEach(row=>row.disabled=true);try{await directory.lifecycle(id,control.dataset.status,detail.current_revision,reason);d.close();toast(control.dataset.status==='retired'?'角色已退休，历史归档保留':control.dataset.status==='paused'?'角色已暂停，可在角色库恢复运行':'角色已恢复运行','ok');}catch(error){d.el.querySelector('[data-error]').textContent=error.message;d.el.querySelectorAll('button').forEach(row=>row.disabled=false);}});
  });
  el.querySelector('[data-group]').onclick=()=>{
    const d=drawer({title:'新建分组',body:`<form class="stack" id="create-character-group">${Field({label:'分组名称',name:'name',hint:'用于整理角色，不会改变角色的资料和生活。',attrs:{required:true,maxlength:60,autocomplete:'off'}})}<p role="alert" data-error></p></form>`,footer:'<button class="btn" data-close>取消</button><button class="btn primary" type="submit" form="create-character-group" data-create>创建分组</button>'});
    d.el.querySelector('[data-close]').onclick=()=>d.close();
    d.el.querySelector('form').onsubmit=async event=>{event.preventDefault();const name=event.target.elements.name.value.trim(),button=d.el.querySelector('[data-create]');if(!name){d.el.querySelector('[data-error]').textContent='请填写分组名称';event.target.elements.name.focus();return;}button.disabled=true;button.textContent='正在创建…';try{await directory.createGroup(name);d.close();}catch(error){if(d.el.isConnected){d.el.querySelector('[data-error]').textContent=error.message;button.disabled=false;button.textContent='创建分组';}}};
  };
  el.querySelectorAll('[data-grp]').forEach(input=>input.onchange=async()=>{input.disabled=true;try{await directory.move(input.dataset.grp,input.value);toast('角色分组已更新','ok');}catch(error){el.querySelector('[data-error]').textContent=error.message;input.disabled=false;}});
}
