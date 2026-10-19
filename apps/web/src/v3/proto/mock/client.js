import * as store from './store.js';

// UI-facing Mock operations. This module deliberately has no HTTP transport,
// BrowserClient, real authentication, provider or database dependencies.
export class MockConflict extends Error {
  constructor(currentVersion) { super('版本冲突，草稿已保留'); this.status = 409; this.currentVersion = currentVersion; }
}
const copy = value => structuredClone(value);
const keyOf = (characterId, actorId) => `${characterId}:${actorId}`;
const clock = () => new Date().toTimeString().slice(0, 5);
function assertScope(characterId) { if (!store.get('characters', characterId)) throw new Error('当前范围未找到角色'); }
export function assertWritableCharacter(characterId) { assertScope(characterId); if (store.get('characters', characterId).status === '已退休') throw new Error('该角色已退休，只能查看归档'); }
function metadata(data) { data.meta.actorVersions ||= {}; data.meta.actorHistory ||= {}; data.actorProfiles ||= {}; }
function actorValue(data, characterId, actorId) {
  if (characterId === 'c1' && actorId === data.user.id) return data.user.background;
  return data.actorProfiles?.[keyOf(characterId, actorId)] || {};
}

export function belongsToWorld(characterId, npc) {
  const worldId = store.get('characters', characterId)?.worldId;
  if (npc.charId && npc.charId !== characterId) return false;
  return npc.worldId ? npc.worldId === worldId : store.get('places', npc.placeId)?.worldId === worldId;
}
export const mockClient = {
  actors() { return [{ id: store.get('user').id, name: '你', kind: 'human' }, { id: 'u_guest', name: '另一位交流参与者', kind: 'human' }]; },
  async aboutYou(characterId, actorId) {
    assertScope(characterId); if (!this.actors().some(actor => actor.id === actorId)) throw new Error('当前范围未找到交流参与者'); const data = store.get(), key = keyOf(characterId, actorId);
    return copy({ characterId, actorId, version: data.meta.actorVersions?.[key] || 0, fields: actorValue(data, characterId, actorId), history: data.meta.actorHistory?.[key] || [],
      impression: characterId === 'c1' && actorId === data.user.id ? { text: '你今天关心她正在画的作品。', source: 'msg_003', kind: '角色印象（Mock）' } : null });
  },
  async saveAboutYou(characterId, actorId, { fields, expectedVersion, operation = 'change' }) {
    assertWritableCharacter(characterId);
    assertScope(characterId); if (!this.actors().some(actor => actor.id === actorId)) throw new Error('当前范围未找到交流参与者'); const data = store.get(), key = keyOf(characterId, actorId), current = data.meta.actorVersions?.[key] || 0;
    if (current !== expectedVersion) throw new MockConflict(current);
    for (const value of Object.values(fields)) if (typeof value !== 'string' || value.length > 8000) throw new Error('请检查资料字段长度');
    store.update(d => { metadata(d); const before = copy(actorValue(d, characterId, actorId));
      if (characterId === 'c1' && actorId === d.user.id) d.user.background = copy(fields);
      else d.actorProfiles[key] = copy(fields);
      d.meta.actorVersions[key] = current + 1;
      (d.meta.actorHistory[key] ||= []).push({ version: current + 1, at: clock(), operation, before, after: copy(fields), source: '本人填写（Mock）' });
    }, { identity: '所有者修改', summary: `保存关于你 · ${actorId}（Mock）`, target: actorId });
    return this.aboutYou(characterId, actorId);
  },
  async simulateActorConflict(characterId, actorId) {
    const current = await this.aboutYou(characterId, actorId);
    return this.saveAboutYou(characterId, actorId, { expectedVersion: current.version, fields: { ...current.fields, 可让她知道: '另一处刚补充了资料（Mock 冲突演示）' } });
  },
  async npcs(characterId, { query = '', archived = false } = {}) {
    assertScope(characterId); 
    return copy(store.list('npcs', npc => belongsToWorld(characterId, npc) && (archived || !npc.archived) && (!query || `${npc.name} ${npc.role}`.includes(query))));
  },
  async npc(characterId, id) {
    const npc = (await this.npcs(characterId, { archived: true })).find(item => item.id === id);
    if (!npc) throw new Error('当前运行世界未找到该 NPC');
    const relations = store.list('relations', item => item.charId === characterId && item.npcId === id);
    const references = [...relations.map(item => item.id), ...store.list('memories', item => item.charId === characterId && item.relatedNpc === id).map(item => item.id), ...store.list('items', item => item.charId === characterId && item.owner === id).map(item => item.id), ...store.list('events', event => event.npcId === id || event.participants?.includes(id)).map(event => event.id)];
    return { ...npc, version: npc.version || 1, routine: npc.routine || [], relations: copy(relations), references };
  },
  async saveNPC(characterId, id, value) {
    assertWritableCharacter(characterId);
    const current = await this.npc(characterId, id);
    if (value.expectedVersion !== current.version) throw new MockConflict(current.version);
    if (!value.name?.trim()) throw new Error('请填写姓名');
    const worldId = store.get('characters', characterId).worldId;
    for (const slot of value.routine) if (!['每天', '工作日', '周末'].includes(slot.day) || !/^([01]\d|2[0-3]):[0-5]\d$/.test(slot.start) || !/^([01]\d|2[0-3]):[0-5]\d$/.test(slot.end) || slot.start === slot.end || !slot.placeId || !slot.activity?.trim()) throw new Error('请完整填写规律的日期、开始结束、地点和活动');
    for (const placeId of [value.placeId, ...value.routine.map(slot => slot.placeId)].filter(Boolean)) if (store.get('places', placeId)?.worldId !== worldId) throw new Error('地点不属于当前运行世界');
    store.update(d => { const inst = d.instances[characterId]?.[current.resourceId]; if (inst) for (const [field, key] of [['name', '名称'], ['role', '简介'], ['bio', '背景']]) { if (value[field] !== current[field]) inst.overrides[key] = value[field]; } Object.assign(d.npcs[id], { name: value.name.trim(), role: value.role, bio: value.bio, placeId: value.placeId, positionMode: value.positionMode || '默认地点', routine: copy(value.routine), enabled: value.enabled, archived: value.archived, version: current.version + 1 }); },
      { identity: '所有者修改', summary: `编辑 NPC ${value.name}（Mock，关系未改变）`, target: id });
    return this.npc(characterId, id);
  },
  async createNPCDraft(characterId, value) {
    assertScope(characterId); if (!value.name?.trim()) throw new Error('请填写姓名');
    const id = store.uid('r_npc_');
    store.update(d => { d.resources[id] = { id, type: 'NPC', name: value.name.trim(), published: null, draft: { 名称: value.name.trim(), 简介: value.role || '', 背景: value.bio || '' }, versions: {} }; }, { identity: '所有者修改', summary: '创建 NPC 定义草稿（Mock，尚未放入世界）', target: id });
    return copy(store.get('resources', id));
  },
  async publishNPCDraft(resourceId) {
    const resource = store.get('resources', resourceId); if (!resource || resource.type !== 'NPC' || !resource.draft) throw new Error('先保存 NPC 草稿');
    const version = `v${Object.keys(resource.versions).length + 1}`;
    store.update(d => { const r = d.resources[resourceId]; r.versions[version] = copy(r.draft); r.published = version; r.draft = null; }, { identity: '所有者修改', summary: '发布 NPC 定义（Mock，运行世界未改变）', target: resourceId });
    return copy(store.get('resources', resourceId));
  },
  async saveNPCDraft(resourceId, value) {
    const resource = store.get('resources', resourceId);
    if (!resource || resource.type !== 'NPC' || !value.name?.trim()) throw new Error('NPC 定义或名称无效');
    store.update(d => { d.resources[resourceId].name = value.name.trim(); d.resources[resourceId].draft = { 名称: value.name.trim(), 简介: value.role || '', 背景: value.bio || '' }; }, { identity: '所有者修改', summary: '保存 NPC 定义草稿（Mock，尚未发布）', target: resourceId });
    return copy(store.get('resources', resourceId));
  },
  async applyNPC(characterId, resourceId, placeId) {
    assertWritableCharacter(characterId);
    assertScope(characterId); const resource = store.get('resources', resourceId), worldId = store.get('characters', characterId).worldId;
    if (resource?.type !== 'NPC' || !resource?.published) throw new Error('先发布 NPC 定义');
    if (!store.get('worlds', worldId)) throw new Error('请先配置目标角色的运行世界');
    if (placeId && store.get('places', placeId)?.worldId !== worldId) throw new Error('地点不属于当前运行世界');
    const existing = store.list('npcs', npc => npc.charId === characterId && npc.resourceId === resourceId)[0];
    if (existing) throw new Error('此定义已应用，请从人物详情查看或编辑');
    const id = store.uid('npc_'), version = resource.versions[resource.published];
    store.update(d => { d.npcs[id] = { id, charId: characterId, worldId, resourceId, appliedVersion: resource.published, version: 1, full: false, name: version.名称, role: version.简介, bio: version.背景, avatar: '/v3/assets/avatars/user.svg', placeId, routine: [], enabled: true, archived: false };
      (d.instances[characterId] ||= {})[resourceId] = { applied: resource.published, overrides: {}, npcId: id };
    }, { identity: '所有者修改', summary: '将 NPC 定义应用到独立运行世界（Mock，未建立关系）', target: id });
    return this.npc(characterId, id);
  },
  async recordEncounter(characterId, id, { placeId, summary, establishRelation = false }) {
    assertWritableCharacter(characterId);
    const npc = await this.npc(characterId, id), worldId = store.get('characters', characterId).worldId;
    if (store.get('places', placeId)?.worldId !== worldId) throw new Error('请选择当前世界的地点');
    if (!summary?.trim()) throw new Error('请填写接触经过');
    const eventId = store.uid('evt_encounter_'), existing = npc.relations[0], relationId = establishRelation ? existing?.id || store.uid('rel_') : null;
    store.update(data => { data.events[eventId] = { id: eventId, charId: characterId, worldId, npcId: id, placeId, title: summary.trim(), state: '已确认', at: clock(), participants: [characterId, id], impacts: relationId ? [{ type: 'relation', id: relationId, text: '来自这次 Mock 接触的关系记录' }] : [] };
      if (relationId) { const relation = data.relations[relationId] ||= { id: relationId, charId: characterId, npcId: id, stage: '熟人', intimacy: { value: 10, state: '推测', evidence: [eventId] }, version: 0, revisions: [] }; relation.version++; relation.revisions.push({ v: relation.version, at: clock(), text: 'Mock 接触确认', source: eventId, by: '运行时' }); }
    }, { identity: '系统运维', summary: '演示有来源的 NPC 接触（Mock，角色未移动）', target: eventId }); return { eventId, relationId };
  },
  async updateNPCDefinition(characterId, id, { expectedVersion, choices = {} }) {
    assertWritableCharacter(characterId);
    const npc = await this.npc(characterId, id), resource = store.get('resources', npc.resourceId);
    if (npc.version !== expectedVersion) throw new MockConflict(npc.version);
    if (resource?.type !== 'NPC' || !resource?.published) throw new Error('定义尚未发布');
    const target = resource.versions[resource.published];
    store.update(data => { const current = data.npcs[id], inst = data.instances[characterId][resource.id];
      for (const [field, key] of [['name', '名称'], ['role', '简介'], ['bio', '背景']]) {
        if (Object.hasOwn(inst.overrides, key) && choices[key] !== 'replace') continue;
        current[field] = target[key]; delete inst.overrides[key];
      }
      inst.applied = resource.published; current.appliedVersion = resource.published; current.version++;
    }, { identity: '所有者修改', summary: '应用 NPC 新定义（Mock，保留关系、规律与位置）', target: id });
    return this.npc(characterId, id);
  },
  async removeNPC(characterId, id) {
    assertWritableCharacter(characterId);
    const npc = await this.npc(characterId, id);
    if (npc.references.length) throw new Error('人物仍被关系或记忆引用，请归档并保留历史');
    store.update(d => { delete d.npcs[id]; if (npc.resourceId && d.instances[characterId]?.[npc.resourceId]?.npcId === id) delete d.instances[characterId][npc.resourceId]; }, { identity: '所有者修改', summary: '移除无历史引用的 NPC（Mock）', target: id });
  },
};
