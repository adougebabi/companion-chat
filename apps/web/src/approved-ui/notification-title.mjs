import {domainName} from '../v3/domain-presentation.mjs';

export function notificationTitle(row, characters = []) {
  if (row?.source_kind === 'conversation.reply.published') {
    const name = characters.find(character => character.id === row.fluctlight_id)?.name?.trim();
    return name ? `${name} · 新回复` : / · 新回复$/.test(row.title || '') ? row.title : '角色 · 新回复';
  }
  return domainName(row?.title, '') || row?.title || '新通知';
}
