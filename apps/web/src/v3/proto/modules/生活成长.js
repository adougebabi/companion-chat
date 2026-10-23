import { renderGoalEditor, renderGoalPlanning } from '../pages/goal-management.js';
// 分区：生活成长（BP-05~09、BP-13）
import { register } from './registry.js';
import { renderNow } from '../pages/life-now.js';
import { renderWardrobe } from '../pages/wardrobe.js';
import { guard } from '../pages/common.js';
import * as L from '../pages/life-extra.js';
import { isMock } from '../store.js';
import { providerPage } from '../pages/provider-page.js';
const liveLife=(load,name,fixture)=>(el,ctx)=>isMock?fixture(el,ctx):providerPage(load,name,el,ctx);
const timeline=liveLife(()=>import('../pages/real-life-timeline.js'),'renderLifeTimeline',renderNow);
const now=liveLife(()=>import('../pages/real-life-now.js'),'renderRealNow',renderNow);
const schedule=liveLife(()=>import('../pages/real-life-schedule.js'),'renderRealSchedule',guard(L.renderSchedule,'日程'));
const wardrobe=liveLife(()=>import('../pages/real-life-wardrobe.js'),'renderRealWardrobe',guard(renderWardrobe,'衣柜'));
const goals=liveLife(()=>import('../pages/real-life-goals.js'),'renderRealGoals',guard(L.renderGoals,'目标'));
const goalEditor=liveLife(()=>import('../pages/real-goal-editor.js'),'renderRealGoalEditor',renderGoalEditor);
const goalPlanning=liveLife(()=>import('../pages/real-goal-planning.js'),'renderRealGoalPlanning',renderGoalPlanning);
const memories=liveLife(()=>import('../pages/real-memory.js'),'renderRealMemoryList',guard(L.renderMemList,'记忆'));
const memory=liveLife(()=>import('../pages/real-memory.js'),'renderRealMemory',guard(L.renderMemItem,'记忆'));
const reflections=liveLife(()=>import('../pages/real-reflection.js'),'renderRealReflections',guard(L.renderReflections,'反思'));
const reflection=liveLife(()=>import('../pages/real-reflection.js'),'renderRealReflection',guard(L.renderReflection,'反思'));
const Z = '生活成长';
register({ route: '/c/:charId/life/now', key: 'life.now', title: '生活 · 此刻', bp: 'BP-05 / BP-06', zone: Z, coverage: '可操作', scope: '运行世界',
  iface: [['GET /characters/:id/now', '待核验'], ['推送更新', '待核验']], source: '工作区“生活”默认 tab；对话页状态条；通知深链',
  render: now, manualRefresh:true, renderState: (el, ctx, s) => { if (isMock && ['空', '失败', '未接入'].includes(s)) { renderNow(el, ctx, s); return true; } return false; } });
register({ route: '/c/:charId/life', key: 'life.timeline', title: '她的生活', bp: 'BP-05 / BP-06', zone: Z, coverage: '可操作', scope: '角色生活', render: timeline, manualRefresh:true });
register({ route: '/c/:charId/life/wardrobe', key: 'items.wardrobe.view', title: '衣柜与物品', bp: 'BP-08', zone: Z, coverage: '可操作',
  iface: [['物品列表', '待核验'], ['换装请求', '新增'], ['拍照生成', '新增']], source: '生活 tab；此刻页当前穿着；行动完成卡', render: wardrobe, manualRefresh:true });
register({ route: '/c/:charId/life/schedule', key: 'life.schedule', title: '生活 · 日程', bp: 'BP-06', zone: Z, coverage: '可操作', scope: '运行世界', iface: [['日程（计划/实际）', '待核验'], ['日程建议', '新增']], source: '生活 tab；通知 nt_1；地点详情「建议她来这里」', render: schedule, manualRefresh:true });
register({ route: '/c/:charId/life/goals', key: 'goals.list.view', title: '目标 · 意向 · 约定', bp: 'BP-07', zone: Z, coverage: '可操作', iface: [['目标列表', '待核验'], ['提出约定', '新增']], render: goals,manualRefresh:true });
register({ route: '/c/:charId/life/diary', key: 'diary.list.view', title: '日记', bp: 'BP-09', zone: Z, coverage: '仅展示', render: liveLife(()=>import('../pages/real-diary.js'),'renderRealDiaryList',guard(L.renderDiaryList,'日记')), manualRefresh:true });
register({ route: '/c/:charId/life/diary/:entryId', key: 'diary.entry.detail', title: '日记条目', bp: 'BP-09', zone: Z, coverage: '可操作', render: liveLife(()=>import('../pages/real-diary.js'),'renderRealDiaryEntry',guard(L.renderDiaryEntry,'日记')), manualRefresh:true });
register({ route: '/c/:charId/life/shared', key: 'diary.shared.timeline', title: '共同经历时间线', bp: 'BP-09.3', zone: Z, coverage: '仅展示', render: liveLife(()=>import('../pages/real-diary.js'),'renderRealShared',guard(L.renderShared,'共同经历')), manualRefresh:true });
register({ route: '/c/:charId/growth', key: 'growth.memory.list', title: '记忆库', bp: 'BP-13', zone: Z, coverage: '可操作', render: memories, manualRefresh:true });
register({ route: '/c/:charId/growth/memory/:memId', key: 'growth.memory.item', title: '记忆条目', bp: 'BP-13', zone: Z, coverage: '可操作', iface: [['记忆修订', '新增']], render: memory, manualRefresh:true });
register({ route: '/c/:charId/growth/reflection', key: 'growth.reflection.timeline', title: '反思与成长时间线', bp: 'BP-13', zone: Z, coverage: '可操作', render: reflections,manualRefresh:true });
register({ route: '/c/:charId/growth/reflection/:refId', key: 'growth.reflection.detail', title: '反思详情', bp: 'BP-13.4', zone: Z, coverage: '可操作', render: reflection,manualRefresh:true });
register({ route: '/c/:charId/growth/self', key: 'growth.self.view', title: '自我认知', bp: 'BP-13', zone: Z, coverage: '可操作', render: liveLife(()=>import('../pages/real-self.js'),'renderRealSelf',guard(L.renderSelf,'自我认知')),manualRefresh:true });
register({ route: '/c/:charId/growth/self/claim/:claimId', key: 'growth.self.claim', title: '认识详情与修订', bp: 'BP-13', zone: Z, coverage: '可操作', render: (el,ctx)=>providerPage(()=>import('../pages/real-self.js'),'renderRealSelfClaim',el,ctx),manualRefresh:true });
register({ route: '/c/:charId/profile', key: 'profile.char.view', title: '角色资料', bp: 'BP-05', zone: Z, coverage: '可操作', source: '角色资料、身份核心、人格档案与受控切换、有效状态', manualRefresh: true, render: L.renderProfile });

register({ route: '/c/:charId/life/goal/new', key: 'goals.create', title: '新建目标', bp: 'BP-07', zone: Z, coverage: '可操作', render: goalEditor, manualRefresh: true });
register({ route: '/c/:charId/life/goal/:goalId', key: 'goals.detail', title: '目标详情与治理', bp: 'BP-07', zone: Z, coverage: '可操作', render: goalEditor, manualRefresh: true });
register({ route: '/c/:charId/life/planning', key: 'goals.planning', title: '独立目标规划', bp: 'BP-07', zone: Z, coverage: '可操作', render: goalPlanning, manualRefresh: true });

register({route:'/c/:charId/life/wardrobe/item/:itemId',key:'life.wardrobe.item.detail',title:'物品详情与历史',bp:'BP-08',zone:Z,coverage:'可操作',manualRefresh:true,render:(el,ctx)=>providerPage(()=>import('../pages/real-life-wardrobe.js'),'renderRealInventoryItem',el,ctx)});
