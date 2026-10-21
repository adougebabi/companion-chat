import { randomId } from '../../../random-id.mjs';
import * as store from '../store.js';
import { auth, directory } from '../../providers/real.ts';
import { esc, toast, requireOwner, drawer, confirm } from '../ui.js';
import { Tabs, Field } from '../components/forms.js';
import {CharacterCard} from '../components/character-card.js';
import {bindAvatars} from '../components/avatar.js';
export {renderRealCreate} from './real-create.js';

export function renderRealAuth(el,ctx) {
  let alive = true;
  const session = store.get('session');
  if (session) {
    el.innerHTML = '<div class="card stack" style="max-width:420px;margin:auto"><p>已登录所有者会话</p><a class="btn primary" href="#/library">进入角色库</a><button class="btn" data-out>退出</button><p data-result role="alert"></p></div>';
    el.querySelector('[data-out]').onclick = async () => { try { await auth.logout(); ctx.navigate('#/auth'); ctx.refresh(); } catch(error) { if(alive) el.querySelector('[data-result]').textContent = error.message; } };
  } else {
    el.innerHTML = '<div class="card" style="max-width:420px;margin:auto" role="status">正在读取登录配置…</div>';
    void auth.setupStatus().then(status => {
      if (!alive) return;
      el.innerHTML = `<form class="card stack" style="max-width:420px;margin:auto"><h2>${status.setupAvailable ? '设置所有者账户' : '所有者登录'}</h2>${status.setupAvailable ? Field({label:'初始化令牌',name:'token',type:'password',attrs:{required:true,autocomplete:'off'}}) : ''}${Field({label:'密码',name:'password',type:'password',attrs:{required:true,autocomplete:status.setupAvailable?'new-password':'current-password',minlength:12}})}<button class="btn primary" data-test="auth-submit">${status.setupAvailable?'设置并登录':'登录'}</button><p data-result role="alert"></p></form>`;
      el.querySelector('form').onsubmit = async event => { event.preventDefault(); const form=event.target, button=form.querySelector('button'); button.disabled=true; try { if(status.setupAvailable) await auth.setup(form.elements.token.value,form.elements.password.value); else await auth.login(form.elements.password.value); if(alive) ctx.navigate('#/library'); } catch(error) { if(alive) form.querySelector('[data-result]').textContent=error.message; } finally { button.disabled=false; } };
    }).catch(error => { if(alive) el.textContent=error.message; });
  }
  return () => {alive=false;};
}
export function renderRealLibrary(el,ctx) {
  const filter=ctx.query.s||'全部', groups=store.list('groups'), characters=store.list('characters');
  const counts=status=>characters.filter(c=>status==='全部'||c.status===status).length;
  el.innerHTML=`<div class="row library-toolbar"><div class="row grow" aria-label="按角色状态筛选">${['全部','正常','异常','已暂停','已退休','未启动'].filter(s=>counts(s)||s===filter).map(s=>`<a class="chip ${filter===s?'on':''}" ${filter===s?'aria-current="page"':''} href="#/library?s=${encodeURIComponent(s)}">${s} <span>${counts(s)}</span></a>`).join('')}</div><button class="btn" data-group>新建分组</button><a class="btn primary" href="#/setup" data-test="lib-create">新建角色</a></div>${!characters.length?'<div class="statebox"><h2>还没有角色</h2><p>从服务连接和世界底稿开始，创建第一个角色。</p><a class="btn primary" href="#/setup">开始创建</a></div>':''}${groups.map(group=>{const rows=characters.filter(c=>c.groupId===group.id&&(filter==='全部'||c.status===filter));return `<section class="library-group"><h3 class="row"><span class="grow">${esc(group.name)} <span class="small muted">${rows.length}</span></span>${group.id!=='ungrouped'?`<button class="btn ghost sm" data-remove-group="${esc(group.id)}">删除分组</button>`:''}</h3>${rows.length?`<div class="library-grid">${rows.map(c=>CharacterCard(c,groups)).join('')}</div>`:characters.length?'<p class="small muted">此分组下没有符合当前筛选的角色。</p>':''}</section>`;}).join('')}<p role="alert" data-error></p>`;
  bindAvatars(el);
  el.querySelectorAll('[data-remove-group]').forEach(button=>button.onclick=async()=>{if(!(await requireOwner())||!(await confirm({title:'删除分组',text:'删除分组，不会删除角色或历史。',okText:'删除分组'})))return;button.disabled=true;try{await directory.removeGroup(button.dataset.removeGroup,randomId());}catch(error){el.querySelector('[data-error]').textContent=error.message;button.disabled=false;}});
  el.querySelectorAll('[data-lifecycle]').forEach(button=>button.onclick=async()=>{
    if(!(await requireOwner()))return;
    const id=button.dataset.lifecycle, character=store.get('characters',id), detail=store.get('meta').details?.[id];
    if(!Number.isSafeInteger(detail?.current_revision))return toast('当前资料版本缺失，请重新读取角色库后再试','err');
    const d=drawer({title:`${esc(character.name)} · 生命周期`,body:`<p>当前状态：${esc(character.status)}</p>${Field({label:'修改原因',name:'reason',type:'textarea',attrs:{required:true,maxlength:1000}})}<p data-error role="alert"></p>`,footer:`<button class="btn" data-close>取消</button><button class="btn primary" data-status="${character.status==='已暂停'?'active':'paused'}">${character.status==='已暂停'?'恢复运行':'暂停运行'}</button><button class="btn danger" data-status="retired">退休并保留归档</button>`});
    d.el.querySelector('[data-close]').onclick=()=>d.close();
    d.el.querySelectorAll('[data-status]').forEach(control=>control.onclick=async()=>{const reason=d.el.querySelector('[name=reason]').value.trim();if(!reason){d.el.querySelector('[data-error]').textContent='请填写修改原因';return;}if(control.dataset.status==='retired'&&!(await confirm({title:'退休角色',text:'停止继续运行并保留历史归档。退休后不能恢复运行。',okText:'确认退休'})))return;d.el.querySelectorAll('button').forEach(row=>row.disabled=true);try{await directory.lifecycle(id,control.dataset.status,detail.current_revision,reason);d.close();}catch(error){d.el.querySelector('[data-error]').textContent=error.message;d.el.querySelectorAll('button').forEach(row=>row.disabled=false);}});
  });
  el.querySelector('[data-group]').onclick=()=>{
    const d=drawer({title:'新建分组',body:`<form class="stack" id="create-character-group">${Field({label:'分组名称',name:'name',hint:'用于整理角色，不会改变角色的资料和生活。',attrs:{required:true,maxlength:60,autocomplete:'off'}})}<p role="alert" data-error></p></form>`,footer:'<button class="btn" data-close>取消</button><button class="btn primary" type="submit" form="create-character-group" data-create>创建分组</button>'});
    d.el.querySelector('[data-close]').onclick=()=>d.close();
    d.el.querySelector('form').onsubmit=async event=>{event.preventDefault();const name=event.target.elements.name.value.trim(),button=d.el.querySelector('[data-create]');if(!name){d.el.querySelector('[data-error]').textContent='请填写分组名称';event.target.elements.name.focus();return;}button.disabled=true;button.textContent='正在创建…';try{await directory.createGroup(name);d.close();}catch(error){if(d.el.isConnected){d.el.querySelector('[data-error]').textContent=error.message;button.disabled=false;button.textContent='创建分组';}}};
  };
  el.querySelectorAll('[data-grp]').forEach(input=>input.onchange=async()=>{input.disabled=true;try{await directory.move(input.dataset.grp,input.value);}catch(error){el.querySelector('[data-error]').textContent=error.message;input.disabled=false;}});
}
