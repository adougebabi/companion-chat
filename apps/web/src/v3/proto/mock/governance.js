import * as store from './store.js';
import { MockConflict, assertWritableCharacter } from './client.js';
const clone = value => structuredClone(value);
const initial = characterId => ({ version: 1, autoPlan: true, order: store.list('goals', goal => goal.charId === characterId).map(goal => goal.id), requests: [], candidates: [],
  capabilities: [{ id: 'cap_media', title: '生成画面', description: '在你同意后，生成标明非事实的画面。', status: '待审核', version: 1, reason: '' }],
  foundation: { version: 1, bio: store.get('characters', characterId)?.bio || '', history: [] },
  developing: { version: 1, claims: [{ id: 'claim_share', text: '愿意更主动分享日常', evidence: store.list('reflections', reflection => reflection.charId === characterId).slice(0, 1).map(reflection => reflection.id), status: '待复核' }], history: [] } });
export const mockGovernance = {
  read(characterId) { if (!store.get('characters', characterId)) throw new Error('当前范围未找到角色'); return clone(store.get('meta').governance?.[characterId] || initial(characterId)); },
  change(characterId, version, summary, mutator) {
    assertWritableCharacter(characterId);
    const current = this.read(characterId); if (current.version !== version) throw new MockConflict(current.version);
    mutator(current); current.version++;
    store.update(data => { (data.meta.governance ||= {})[characterId] = current; }, { identity: '所有者修改', summary: summary + '（Mock）', target: characterId }); return this.read(characterId);
  },
  goal(characterId, id) { const goal = store.get('goals', id); if (!goal || goal.charId !== characterId) throw new Error('当前范围未找到目标'); return clone({ ...goal, version: goal.version || 1, revisions: goal.revisions || [] }); },
  saveGoal(characterId, id, value, expectedVersion) {
    assertWritableCharacter(characterId);
    const current = id ? this.goal(characterId, id) : null; if (current && current.version !== expectedVersion) throw new MockConflict(current.version);
    if (!value.title?.trim()) throw new Error('请填写目标标题');
    const target = id || store.uid('g_');
    store.update(data => { const before = current ? clone(current) : null; data.goals[target] = { ...current, ...clone(value), id: target, charId: characterId, version: (current?.version || 0) + 1, revisions: [...(current?.revisions || []), { v: (current?.version || 0) + 1, at: '刚刚', text: value.note || '所有者编辑', before }] }; }, { identity: '所有者修改', summary: '保存目标治理（Mock，不伪造执行结果）', target }); return this.goal(characterId, target);
  },
  commandGoal(characterId, id, command, expectedVersion) {
    const current = this.goal(characterId, id), states = { 暂停: '已暂停', 恢复: '计划', 取消: '取消', 放弃: '已放弃' };
    return this.saveGoal(characterId, id, { ...current, state: states[command] || current.state, note: command === '重新评估' ? '已提交 Mock 重新评估请求，等待演示结果' : `所有者${command}` }, expectedVersion);
  },
  async plan(characterId, version) {
    const value = this.change(characterId, version, '提交立即规划', current => { current.requests.unshift({ id: store.uid('plan_'), state: '执行中', text: 'Mock 规划任务已排队' }); });
    await new Promise(resolve => setTimeout(resolve, 350));
    // Read latest version after async work: do not overwrite intervening edits.
    const latest = this.read(characterId);
    if (!latest.requests.some(item => item.id === value.requests[0].id)) return latest;
    return this.change(characterId, latest.version, '完成 Mock 规划演示', current => { const request = current.requests.find(item => item.id === value.requests[0].id); if (request) { request.state = '完成'; request.text = 'Mock 规划产生 1 条候选，尚未成为目标'; } current.candidates.push({ id: store.uid('candidate_'), title: '整理本周创作素材', reason: 'Mock 演示候选 · 待所有者审核', state: '候选' }); });
  },
};
