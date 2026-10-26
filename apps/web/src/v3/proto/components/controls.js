// Shared presentation components. Output preserves the reference v3 DOM and classes.
export const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
export const icon = (name, cls = '') => `<svg class="icon ${cls}" aria-hidden="true"><use href="/v3/assets/icons.svg#i-${name}"></use></svg>`;

const BADGE_CLASS = { '生成图·非事实': 'b-nonfact', '创作预览·不进入生活': 'b-preview' };
/** badge('已确认') / badge('交流') / badge('生成图·非事实') / badge('角色范围') / badge('任意文字','plain') */
export function badge(label, kind) {
  const cls = kind ? 'b-' + kind : (BADGE_CLASS[label] || 'b-' + label);
  return `<span data-domain-ui="badge" class="badge ${esc(cls)}" data-badge="${esc(label)}">${esc(label)}</span>`;
}
export const LABELS = {
  identity: ['交流', '行动请求', '所有者修改', '系统运维'],
  fact: ['计划', '候选', '推测', '已确认', '失败', '取消', '未知'],
  nonfact: ['生成图·非事实', '创作预览·不进入生活'],
  scope: ['全局定义', '角色范围', '运行世界'],
};

