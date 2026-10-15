// 分区：生活成长（BP-05~09、BP-13）
import { register } from './registry.js';
import { renderNow } from '../pages/life-now.js';
import { renderWardrobe } from '../pages/wardrobe.js';
import { guard } from '../pages/common.js';
import * as L from '../pages/life-extra.js';
const Z = '生活成长';
register({ route: '/c/:charId/life/now', key: 'life.now', title: '生活 · 此刻', bp: 'BP-05 / BP-06', zone: Z, coverage: '可操作', scope: '运行世界',
  iface: [['GET /characters/:id/now', '待核验'], ['推送更新', '待核验']], source: '工作区“生活”默认 tab；对话页状态条；通知深链',
  render: (el, ctx) => renderNow(el, ctx), renderState: (el, ctx, s) => { if (['空', '失败', '未接入'].includes(s)) { renderNow(el, ctx, s); return true; } return false; } });
register({ route: '/c/:charId/life', key: 'life.now', title: '生活 · 此刻', bp: 'BP-05 / BP-06', zone: Z, coverage: '可操作', scope: '运行世界', render: (el, ctx) => renderNow(el, ctx) });
register({ route: '/c/:charId/life/wardrobe', key: 'items.wardrobe.view', title: '衣柜与物品', bp: 'BP-08', zone: Z, coverage: '可操作',
  iface: [['物品列表', '待核验'], ['换装请求', '新增'], ['拍照生成', '新增']], source: '生活 tab；此刻页当前穿着；行动完成卡', render: guard(renderWardrobe, '衣柜') });
register({ route: '/c/:charId/life/schedule', key: 'life.schedule', title: '生活 · 日程', bp: 'BP-06', zone: Z, coverage: '可操作', scope: '运行世界', iface: [['日程（计划/实际）', '待核验'], ['日程建议', '新增']], source: '生活 tab；通知 nt_1；地点详情「建议她来这里」', render: guard(L.renderSchedule, '日程') });
register({ route: '/c/:charId/life/goals', key: 'goals.list.view', title: '目标 · 意向 · 约定', bp: 'BP-07', zone: Z, coverage: '可操作', iface: [['目标列表', '待核验'], ['提出约定', '新增']], render: guard(L.renderGoals, '目标') });
register({ route: '/c/:charId/life/diary', key: 'diary.list.view', title: '日记', bp: 'BP-09', zone: Z, coverage: '仅展示', render: guard(L.renderDiaryList, '日记') });
register({ route: '/c/:charId/life/diary/:entryId', key: 'diary.entry.detail', title: '日记条目', bp: 'BP-09', zone: Z, coverage: '可操作', render: guard(L.renderDiaryEntry, '日记') });
register({ route: '/c/:charId/life/shared', key: 'diary.shared.timeline', title: '共同经历时间线', bp: 'BP-09.3', zone: Z, coverage: '仅展示', render: guard(L.renderShared, '共同经历') });
register({ route: '/c/:charId/growth', key: 'growth.memory.list', title: '记忆库', bp: 'BP-13', zone: Z, coverage: '可操作', render: guard(L.renderMemList, '记忆') });
register({ route: '/c/:charId/growth/memory/:memId', key: 'growth.memory.item', title: '记忆条目', bp: 'BP-13', zone: Z, coverage: '可操作', iface: [['记忆修订', '新增']], render: guard(L.renderMemItem, '记忆') });
register({ route: '/c/:charId/growth/reflection', key: 'growth.reflection.timeline', title: '反思与成长时间线', bp: 'BP-13', zone: Z, coverage: '仅展示', render: guard(L.renderReflections, '反思') });
register({ route: '/c/:charId/growth/reflection/:refId', key: 'growth.reflection.detail', title: '反思详情', bp: 'BP-13.4', zone: Z, coverage: '可操作', render: guard(L.renderReflection, '反思') });
register({ route: '/c/:charId/growth/self', key: 'growth.self.view', title: '自我认知', bp: 'BP-13', zone: Z, coverage: '可操作', render: guard(L.renderSelf, '自我认知') });
register({ route: '/c/:charId/profile', key: 'profile.char.view', title: '角色资料', bp: 'BP-05', zone: Z, coverage: '可操作', source: '每个角色显示自己的资料与状态', render: L.renderProfile });
