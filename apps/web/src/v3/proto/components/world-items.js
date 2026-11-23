import {esc,badge} from '../ui.js';
import {RawRecordDownload} from './records.js';
import {isInternalId} from '../../internal-id.mjs';
import {userInstant} from '../../presentation-time.mjs';
import {wardrobeSlotLabel} from '../../../lib/wardrobe-form.ts';

function readableName(value, id) {
  const name = typeof value === 'string' ? value.trim() : '';
  return name === id || isInternalId(name) ? '' : name;
}

export function WorldItemPlaceName(place) {
  return readableName(place?.name, place?.id) || readableName(place?.alias, place?.id);
}

export function WorldItemSummary(item, {world, place, heading = 'h2'} = {}) {
  const name = readableName(item.name, item.id) || '未命名的世界物品';
  const {description, item_kind, slot} = item.definition ?? {};
  const status = item.available === true ? '可提出取得请求' : item.available === false ? '当前不可取得' : '';
  const part = item_kind === 'wearable' && slot ? readableName(wardrobeSlotLabel(slot)) : '';
  const type = item_kind === 'wearable' ? ['服饰', part].filter(Boolean).join(' · ') : item_kind ? '普通物品' : '';
  const worldName = readableName(world?.name, item.world_id) || '当前运行世界';
  const placeName = WorldItemPlaceName(place);
  const scope = item.place_id ? (placeName ? '取得地点：' + placeName : '仅限指定地点取得') : '取得地点不限';
  const updated = typeof item.updated_at === 'string' && Number.isFinite(Date.parse(item.updated_at));
  return `<div class="row"><${heading} class="grow">${esc(name)}</${heading}>${status ? badge(status, 'plain') : ''}</div>
    ${typeof description === 'string' && description.trim() ? `<p>${esc(description)}</p>` : ''}
    ${type ? `<p class="small muted">${esc(type)}</p>` : ''}
    <p class="small muted">所属世界：${esc(worldName)} · ${esc(scope)}</p>
    ${updated ? `<p class="small muted">更新时间：<time datetime="${esc(item.updated_at)}">${esc(userInstant(item.updated_at))}</time></p>` : ''}`;
}

export function WorldItemDownload(item) {
  return RawRecordDownload(item, '原始世界物品记录（诊断）');
}

export function WorldItemCard(item, fid, {world, place} = {}) {
  return `<div data-domain-ui="card" class="card">
    ${WorldItemSummary(item, {world, place, heading: 'h3'})}
    <p class="small muted">目录记录不证明角色已经持有</p>
    <a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${encodeURIComponent(fid)}/world/items/${encodeURIComponent(item.id)}">详情与取得范围</a>
    ${WorldItemDownload(item)}
  </div>`;
}

export function WorldItemSource(item) {
  const overrides = item.local_overrides;
  const retained = [['name', '物品名称'], ['definition', '物品定义']]
    .filter(([key]) => overrides?.[key] != null && overrides[key] !== '')
    .map(([, label]) => label);
  return `${item.source_resource_id ? `<p>来自 <a href="#/studio/edit/${encodeURIComponent(item.source_resource_id)}">物品定义</a></p>` : ''}
    ${retained.length ? `<p class="small muted">此世界保留了自己的${retained.join('、')}。</p>` : ''}`;
}
export function WorldItemSuggestion(item){return `你愿意在虚拟生活中尝试取得「${item.name}」吗？目录引用：${item.id}。请先核对这件目录物品的真实资料、取得地点和可用状态，再根据你的意愿、目标与时间安排决定是否以及何时行动。这只是世界目录，不代表已经在你的衣柜里；尚未完成取得时，请不要说已经拥有、穿上或使用。`;}
