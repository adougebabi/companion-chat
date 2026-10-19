import { esc, icon, badge } from './controls.js';
import { attributes } from './forms.js';

export function Card({ body = '', title = '', heading, tone = '', attrs = {}, footer = '' } = {}) {
  return `<div class="card${tone ? ` ${esc(tone)}` : ''}" ${attributes(attrs)}>${heading ?? (title ? `<h3>${esc(title)}</h3>` : '')}${body}${footer}</div>`;
}
export function EntityHeader({ avatar, title, subtitle = '', status = '', size = 'lg' }) {
  return `<div class="row"><img class="av ${esc(size)}" src="${esc(avatar)}" alt=""><div class="grow"><h2 style="margin:0">${esc(title)}</h2><div class="small muted">${subtitle}</div></div>${status}</div>`;
}
export function OutfitStrip({ items, link, attrs = {} }) {
  return `<div class="outfit" ${attributes(attrs)}>${items.map(item => {
    const image = item.img ? `<img src="${esc(item.img)}" alt="${esc(item.name)}"${link ? '' : ` title="${esc(item.name)}"`}>` : `<span class="small muted" style="display:inline-block;padding:12px">${esc(item.name)}</span>`;
    return link ? `<a href="${esc(link(item))}" title="${esc(item.name)}">${image}</a>` : image;
  }).join('')}</div>`;
}
export function NPCRow(npc, link) {
  return `<div class="lrow"><img class="av sm" src="${esc(npc.avatar)}" alt="">${link(npc.id)}<span class="small muted">${esc(npc.role)}</span></div>`;
}
export function PreviewCard(preview, { resourceName = '', includeResource = false } = {}) {
  return `<div class="card" data-test="preview-item"><div class="row">${badge('创作预览·不进入生活')}<span class="small muted grow">${includeResource ? `${esc(resourceName)} · ${esc(preview.at)}` : `${esc(preview.id)} · ${esc(preview.at)}`}</span><button class="btn ghost sm" data-del="${esc(preview.id)}">丢弃</button></div>${preview.img ? `<img src="${esc(preview.img)}" alt="虚构创作图片预览" data-preview-image style="width:100%;border-radius:10px;margin:6px 0"><p class="small muted" data-preview-image-error hidden>图片读取失败，请核对素材权限与任务状态后刷新。</p>` : ''}${preview.text ? `<p class="small">${esc(preview.text)}</p>` : ''}</div>`;
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
