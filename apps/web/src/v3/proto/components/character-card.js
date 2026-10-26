import {esc, badge} from './controls.js';
import {Avatar} from './avatar.js';

export function CharacterCard(character, groups) {
  const id = encodeURIComponent(character.id), active = character.status === '正常';
  return `<article class="card library-card" data-test="character-card">
    <a class="library-person" href="#/c/${id}/${active ? 'chat' : 'profile'}" aria-label="${esc(active ? `与${character.name}对话` : `查看${character.name}的资料`)}">
      ${Avatar({src:character.avatar,name:character.name})}
      <div class="grow"><h3>${esc(character.name)}</h3><span class="small muted">${active ? '开始聊天 →' : '查看角色资料 →'}</span></div>
    </a>
    <div class="library-card-meta">${badge(character.status,active ? '已确认' : character.status === '异常' ? '失败' : 'plain')}${character.bio ? `<p class="small muted">${esc(character.bio)}</p>` : ''}</div>
    <div class="library-card-actions">
      <details data-domain-ui="disclosure" class="library-management"><summary aria-label="${esc(character.name)}的更多操作">更多 ···</summary><div class="stack"><a data-domain-ui="button" class="btn ghost sm" href="#/c/${id}/profile">查看资料</a>
        <label class="field">所属分组<select data-domain-ui="field" class="select" data-grp="${esc(character.id)}" aria-label="为${esc(character.name)}选择分组"><option value="" ${character.groupId === 'ungrouped' ? 'selected' : ''}>未分组</option>${groups.filter(group=>group.id !== 'ungrouped').map(group=>`<option value="${esc(group.id)}" ${group.id === character.groupId ? 'selected' : ''}>${esc(group.name)}</option>`).join('')}</select></label>
        ${character.status !== '已退休' ? `<button data-domain-ui="button" class="btn sm" data-lifecycle="${esc(character.id)}">生命周期管理</button>` : ''}
      </div></details>
    </div>
  </article>`;
}
