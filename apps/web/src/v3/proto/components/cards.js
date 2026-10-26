import {userInstant} from '../../presentation-time.mjs';
import { esc, icon, badge } from './controls.js';
import { attributes } from './forms.js';
import {JsonDetails} from './records.js';
import {Avatar} from './avatar.js';

export function Card({ body = '', title = '', heading, tone = '', attrs = {}, footer = '' } = {}) {
  return `<div class="card${tone ? ` ${esc(tone)}` : ''}" ${attributes(attrs)}>${heading ?? (title ? `<h3>${esc(title)}</h3>` : '')}${body}${footer}</div>`;
}
export function EntityHeader({ avatar, title, subtitle = '', status = '', size = 'lg' }) {
  return `<div class="row">${Avatar({src:avatar,name:title,size})}<div class="grow"><h2 style="margin:0">${esc(title)}</h2><div class="small muted">${subtitle}</div></div>${status}</div>`;
}
export function OutfitStrip({ items, link, attrs = {} }) {
  return `<div class="outfit" ${attributes(attrs)}>${items.map(item => {
    const image = item.img ? `<img src="${esc(item.img)}" alt="${esc(item.name)}"${link ? '' : ` title="${esc(item.name)}"`}>` : `<span class="small muted" style="display:inline-block;padding:12px">${esc(item.name)}</span>`;
    return link ? `<a href="${esc(link(item))}" title="${esc(item.name)}">${image}</a>` : image;
  }).join('')}</div>`;
}
export function NPCRow(npc, link) {
  return `<div class="lrow">${Avatar({src:npc.avatar,name:npc.name,size:'sm'})}${link(npc.id)}<span class="small muted">${esc(npc.role)}</span></div>`;
}
export function PreviewCard(preview, { resourceName = '', includeResource = false } = {}) {
  return `<div class="card" data-test="preview-item"><div class="row">${badge('创作预览·不进入生活')}<span class="small muted grow">${includeResource ? `${esc(resourceName)} · ${esc(userInstant(preview.at))}` : `${esc(preview.id)} · ${esc(userInstant(preview.at))}`}</span><button data-domain-ui="button" class="btn ghost sm" data-del="${esc(preview.id)}">丢弃</button></div>${preview.img ? `<img src="${esc(preview.img)}" alt="虚构创作图片预览" data-preview-image style="width:100%;border-radius:10px;margin:6px 0"><p class="small muted" data-preview-image-error hidden>图片读取失败，请核对素材权限与任务状态后刷新。</p>` : ''}${preview.text ? `<p class="small">${esc(preview.text)}</p>` : ''}</div>`;
}
export function ResourceCard(resource, { pending = false } = {}) {
  return `<a class="card" href="#/studio/${resource.type === 'NPC' ? 'npc' : 'edit'}/${esc(resource.id)}" data-test="res-${esc(resource.id)}"><div class="row"><b class="grow">${esc(resource.name)}</b>${badge(resource.type, 'plain')}</div><div class="small muted">已发布 ${esc(resource.published || '尚未发布')}${resource.draft ? ' · 有草稿' : ''}${pending ? ' · 有实例待应用' : ''}</div></a>`;
}
export function EmptyState({ title, text = '', actions = '' }) {
  return `<div class="statebox"><h2>${esc(title)}</h2>${text ? `<p class="muted">${esc(text)}</p>` : ''}${actions ? `<div class="row" style="justify-content:center">${actions}</div>` : ''}</div>`;
}
export function ImageCard({ as = 'div', className = 'icard', img, imageHref, caption, attrs = {} }) {
  if (!['div', 'button', 'a'].includes(as)) throw new Error('Invalid image-card element');
  if (as !== 'div' && imageHref) throw new Error('Interactive image cards cannot contain another link');
  const image = img ? `<img src="${esc(img)}" alt="">` : '<div class="small muted" style="min-height:96px;display:grid;place-items:center">尚无图片</div>';
  return `<${as} class="${esc(className)}" ${attributes(attrs)}>${imageHref ? `<a href="${esc(imageHref)}">${image}</a>` : image}<div class="cap">${caption}</div></${as}>`;
}

export function GoalRow(goal, link, { compact = false, showDue = false, attrs = {} } = {}) {
  return `<div class="lrow${compact ? ' small' : ''}" ${attributes(attrs)}>${badge(goal.type, 'plain')}<span class="grow">${link(goal.id, goal.title)}${showDue ? ` ${goal.due ? `<span class="small muted">${esc(goal.due)}</span>` : ''}` : ''}</span>${badge(goal.state)}</div>`;
}

const intentionLabels={candidate:'候选意向',qualified:'已具备执行条件',due:'待执行',in_progress:'正在执行',paused:'已暂停',completed:'意向已完成',expired:'已过期',cancelled:'已取消'};
/** Read-only intention, shared by goals, current life and promise details. */
export function IntentionRow(intention,{fid,goals=[],profiles=[]}={}){
 const value=intention&&typeof intention==='object'?intention:{};
 const action=[value.action_intent,value.action,value.description].find(v=>typeof v==='string'&&v.trim())||'尚未记录意向内容';
 const goal=goals.find(g=>g.id===value.goal_id),profile=profiles.find(p=>p.id===value.profile_id);
 const confidence=typeof value.confidence==='number'&&Number.isFinite(value.confidence)&&value.confidence>=0&&value.confidence<=1?`把握 ${Math.round(value.confidence*100)}%`:'';
 return `<div class="lrow intention-row"><div class="grow"><div class="row"><b class="grow">${esc(action)}</b>${badge(intentionLabels[value.status]||'状态待确认','plain')}</div><div class="row small muted">${value.goal_id&&fid?`<a href="#/c/${encodeURIComponent(fid)}/life/goal/${encodeURIComponent(value.goal_id)}">${esc(goal?.desired_outcome||goal?.description||goal?.title||'查看关联目标')}</a>`:''}${profile?.name?`<span>人格：${esc(profile.name)}</span>`:''}${confidence?`<span>${esc(confidence)}</span>`:''}</div>${JsonDetails(value,'意向来源与技术记录')}</div></div>`;
}
export function IntentionList(values,options={}){
 const rows=Array.isArray(values)?values:[];
 return rows.length?rows.map(row=>IntentionRow(row,options)).join(''):'<p class="small muted">尚未记录意向。可以从对话提出计划，或请求独立规划。</p>';
}

// One action per person; browsing never selects a character or creates a relation.
export function PersonCard({name,avatar='',description='',status='',href,label='查看人物',attrs={}}) {
  return `<article class="person-card" ${attributes(attrs)}><div class="person-portrait">${Avatar({src:avatar,name,size:'lg'})}</div><div class="person-card-body"><h3>${esc(name)}</h3><p class="small muted">${esc(description)}</p><p class="small">${esc(status)}</p><a data-domain-ui="button" class="btn" href="${esc(href)}">${esc(label)}</a></div></article>`;
}
