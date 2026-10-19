import * as store from './store.js';
import { createMockSeed } from './seed.js';
import { MockConflict, assertWritableCharacter } from './client.js';
const clone = value => structuredClone(value);
export const sceneOptions = ['cafe', 'oldstreet', 'flowershop', 'seaside', 'apartment', 'shop'].map(name => ({ value: `/v3/assets/scenes/${name}.svg`, label: name }));
export const mockWorld = {
  worlds(characterId) { const originalWorldId = createMockSeed().characters[characterId]?.worldId; return clone(store.list('worlds', world => world.charId === characterId || store.get('characters', characterId)?.worldId === world.id || world.id === originalWorldId)); },
  world(characterId, id) { const world = this.worlds(characterId).find(item => item.id === id); if (!world) throw new Error('当前角色范围未找到世界'); return { ...world, version: world.version || 1 }; },
  create(characterId, name) { assertWritableCharacter(characterId);
    if (!name.trim()) throw new Error('请填写世界名称'); const id = store.uid('w_');
    store.update(data => { data.worlds[id] = { id, charId: characterId, name: name.trim(), version: 1, description: '', flavor: '', itemPool: '' }; data.worldSettings[id] = { randomEvents: false, freq: '适中', cats: { 日常小波折: true, 新发现: true }, harmLocked: true, intercepts: [] }; }, { identity: '所有者修改', summary: '创建空白世界（Mock，尚未切换）', target: id }); return id;
  },
  save(characterId, id, fields, version) { assertWritableCharacter(characterId);
    const world = this.world(characterId, id); if (world.version !== version) throw new MockConflict(world.version);
    if (!fields.name.trim()) throw new Error('请填写世界名称');
    store.update(data => { Object.assign(data.worlds[id], fields, { version: version + 1 }); }, { identity: '所有者修改', summary: '保存世界定义（Mock）', target: id });
  },
  select(characterId, id) { assertWritableCharacter(characterId);
    this.world(characterId, id);
    store.update(data => { const character = data.characters[characterId]; character.worldId = id; if (data.places[character.locationId]?.worldId !== id) { character.locationId = null; character.doing = '新世界的初始位置尚未确认'; } }, { identity: '所有者修改', summary: '显式切换运行世界（Mock，位置待确认）', target: id });
  },
  references(id) {
    return [...store.list('characters', item => item.worldId === id || item.locationId === id).map(item => item.id), ...store.list('places', item => item.worldId === id).map(item => item.id), ...store.list('npcs', item => item.placeId === id || item.worldId === id || item.routine?.some(slot => slot.placeId === id)).map(item => item.id), ...['schedule', 'events', 'media'].flatMap(col => store.list(col, item => item.placeId === id).map(item => item.id))];
  },
  removeWorld(characterId, id) { assertWritableCharacter(characterId); this.world(characterId, id); if (createMockSeed().worlds[id] || this.references(id).length) throw new Error('内置或被引用的世界不能移除'); store.update(data => { delete data.worlds[id]; delete data.worldSettings[id]; }, { identity: '所有者修改', summary: '移除空白自定义世界（Mock）', target: id }); },
  place(characterId, worldId, id) { this.world(characterId, worldId); const place = store.get('places', id); if (!place || place.worldId !== worldId) throw new Error('当前世界未找到地点'); return clone({ ...place, version: place.version || 1 }); },
  savePlace(characterId, worldId, id, fields, version) { assertWritableCharacter(characterId);
    this.world(characterId, worldId); if (!fields.name?.trim()) throw new Error('请填写地点名称');
    if (!sceneOptions.some(option => option.value === fields.scene)) throw new Error('请选择原型素材');
    if (id && this.place(characterId, worldId, id).version !== version) throw new MockConflict(this.place(characterId, worldId, id).version);
    const target = id || store.uid('p_'); store.update(data => { data.places[target] = { ...data.places[target], ...clone(fields), id: target, worldId, charId: characterId, version: (version || 0) + 1 }; }, { identity: '所有者修改', summary: '保存世界地点（Mock，角色未移动）', target }); return target;
  },
  removePlace(characterId, worldId, id) { assertWritableCharacter(characterId); this.place(characterId, worldId, id); if (this.references(id).length) throw new Error('地点被角色、人物、规律或历史引用，不能移除'); store.update(data => { delete data.places[id]; }, { identity: '所有者修改', summary: '移除无引用地点（Mock）', target: id }); },
  restorePlace(characterId, worldId, id) { assertWritableCharacter(characterId); const current = this.place(characterId, worldId, id), original = createMockSeed().places[id]; if (!original) throw new Error('自定义地点没有内置默认值'); this.savePlace(characterId, worldId, id, original, current.version); },
};
