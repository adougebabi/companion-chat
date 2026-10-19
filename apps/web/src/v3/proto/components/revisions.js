import { esc, badge, icon } from './controls.js';
import { DiffView } from './forms.js';

export function RevisionHistory({ entries, renderEntry, emptyText = '尚无修订记录' }) {
  return `<div class="chain">${entries.length ? [...entries].reverse().map(entry => `<div class="lrow small"><b>v${esc(entry.version ?? entry.v)}</b><span class="grow">${renderEntry ? renderEntry(entry) : esc(entry.text || entry.source || '')}</span><span class="muted">${esc(entry.at)}</span></div>`).join('') : `<p class="small muted">${esc(emptyText)}</p>`}</div>`;
}
export function ConflictPanel({ baseVersion, currentVersion, rows = [], keepAttribute = 'keep', refreshAttribute = 'refresh' }) {
  return `<div class="banner err" role="alert" data-test="conflict">${icon('warn')} 版本冲突：打开时 v${esc(baseVersion)}，当前 v${esc(currentVersion)}。草稿尚未覆盖已保存资料。</div>${DiffView(rows)}<div class="row" style="margin:8px 0"><button type="button" class="btn" data-${esc(keepAttribute)}>保留草稿</button><button type="button" class="btn primary" data-${esc(refreshAttribute)}>读取新版本并保留草稿</button></div>`;
}
