// One explicit route inventory for object-level tools inside the workspace.
// These modules own the original domain commands, never another app or router.
const routes = [];
const add = (path, page, title, load, render, extra = {}) => {
  const keys = [];
  const pattern = new RegExp('^' + path.replace(/:([a-zA-Z]+)/g, (_, key) => { keys.push(key); return '([^/]+)'; }) + '$');
  routes.push({path, pattern, keys, page, title, load, render, ...extra});
};
add('/studio/edit/:resId', '创作中心', '编辑创作资源', () => import('../react-app/pages/StudioEditorPage.tsx').then(module => module.default), 'StudioEditorPage', {native:'studio-edit'});
add('/studio/preview', '创作中心', '创作预览', () => import('../react-app/pages/StudioPreviewPage.tsx').then(module => module.default), 'StudioPreviewPage', {native:'studio-preview'});
add('/studio/material/:materialId', '创作中心', '创作素材', () => import('../react-app/pages/StudioMaterialPage.tsx').then(module => module.default), 'StudioMaterialPage', {native:'studio-material'});
add('/studio/npc/:resId', '创作中心', '编辑人物定义', () => import('../react-app/pages/StudioEditorPage.tsx').then(module => module.default), 'StudioEditorPage', {native:'studio-edit'});
add('/media/:mdId', '媒体', '素材详情', () => import('../react-app/pages/MediaPage.tsx').then(module => module.default), 'MediaPage', {native:'media'});
add('/c/:charId/media/avatar', '媒体', '显示头像', () => import('../react-app/pages/MediaPage.tsx').then(module => module.default), 'MediaPage', {native:'media',initialTab:'avatar'});
add('/c/:charId/media/identity/edit', '媒体', '编辑视觉身份', () => import('../react-app/pages/MediaPage.tsx').then(module => module.default), 'MediaPage', {native:'media',initialTab:'identity',initialEdit:true});
add('/c/:charId/media/identity', '媒体', '视觉身份', () => import('../react-app/pages/MediaPage.tsx').then(module => module.default), 'MediaPage', {native:'media',initialTab:'identity'});
add('/c/:charId/media/generate', '媒体', '生成画面', () => import('../react-app/pages/MediaPage.tsx').then(module => module.default), 'MediaPage', {native:'media',initialTab:'generate'});
add('/c/:charId/media/:mdId', '媒体', '素材详情', () => import('../react-app/pages/MediaPage.tsx').then(module => module.default), 'MediaPage', {native:'media'});
add('/c/:charId/governance/:governanceKind/:governanceId', '角色详情', '治理记录', () => import('../react-app/pages/GovernancePage.tsx').then(module => module.default), 'GovernancePage', {native:'governance'});
add('/c/:charId/governance', '角色详情', '身份与能力治理', () => import('../react-app/pages/GovernancePage.tsx').then(module => module.default), 'GovernancePage', {native:'governance'});
add('/c/:charId/growth/self/claim/:claimId', '记忆', '认识的来源与修订', () => import('../react-app/pages/SelfPage.tsx').then(module => module.default), 'SelfPage', {native:'self'});
add('/c/:charId/growth/self', '记忆', '自我认知', () => import('../react-app/pages/SelfPage.tsx').then(module => module.default), 'SelfPage', {native:'self'});
add('/c/:charId/growth/memory/:memId', '记忆', '记忆详情', () => import('../v3-domain/memory.js').then(module => module.renderRealMemory), 'renderRealMemory');
add('/c/:charId/growth/reflection/:refId', '记忆', '反思详情', () => import('../v3-domain/reflection.js').then(module => module.renderRealReflection), 'renderRealReflection');
add('/c/:charId/growth/reflection', '记忆', '反思记录', () => import('../v3-domain/reflection.js').then(module => module.renderRealReflections), 'renderRealReflections');
add('/c/:charId/life/goal/:goalId', '目标', '目标详情', () => import('../v3-domain/goal-editor.js').then(module => module.renderRealGoalEditor), 'renderRealGoalEditor');
add('/c/:charId/life/planning', '目标', '目标规划', () => import('../v3-domain/goal-planning.js').then(module => module.renderRealGoalPlanning), 'renderRealGoalPlanning');
add('/c/:charId/life/schedule/planning', '日程', '安排与调整日程', () => import('../v3-domain/life-schedule.js').then(module => module.renderRealSchedule), 'renderRealSchedule');
add('/c/:charId/life/wardrobe/item/:itemId', '衣柜', '衣物与物品记录', () => import('../v3-domain/life-wardrobe.js').then(module => module.renderRealInventoryItem), 'renderRealInventoryItem');
add('/c/:charId/life/diary/:entryId', '日记', '日记详情', () => import('../v3-domain/diary.js').then(module => module.renderRealDiaryEntry), 'renderRealDiaryEntry');
add('/c/:charId/life/shared', '日记', '共同经历', () => import('../v3-domain/diary.js').then(module => module.renderRealShared), 'renderRealShared');
add('/c/:charId/life/timeline', '此刻', '生活时间线', () => import('../v3-domain/life-timeline.js').then(module => module.renderLifeTimeline), 'renderLifeTimeline');
add('/c/:charId/chat/history', '聊天', '会话历史', () => import('../react-app/pages/ChatPage.tsx').then(module => module.default), 'ChatPage', {native:'chat-history'});
add('/feed/:postId', '动态', '动态详情', () => import('../react-app/pages/FeedPage.jsx').then(module => module.default), 'FeedPage', {native:'feed'});
add('/c/:charId/worlds/:worldId/place/:placeId/history', '世界', '地点修订历史', () => import('../v3/proto/components/place-defaults.js').then(module => module.renderPlaceHistory), 'renderPlaceHistory');
add('/c/:charId/worlds/:worldId/place/:placeId/edit', '世界', '编辑地点', () => import('../v3-domain/world-management.js').then(module => module.renderPlaceEditor), 'renderPlaceEditor');
add('/c/:charId/worlds/:worldId/place/new', '世界', '添加地点', () => import('../v3-domain/world-management.js').then(module => module.renderPlaceEditor), 'renderPlaceEditor');
add('/c/:charId/worlds/:worldId/edit', '世界', '编辑世界', () => import('../v3-domain/world-management.js').then(module => module.renderWorldEditor), 'renderWorldEditor');
add('/c/:charId/world/events', '世界', '世界事件', () => import('../v3-domain/world-events.js').then(module => module.renderRealWorldEvents), 'renderRealWorldEvents');
add('/c/:charId/world/place/:placeId', '世界', '地点详情', () => import('../v3-domain/world-details.js').then(module => module.renderPlace), 'renderPlace');
add('/c/:charId/world/scene', '世界', '当前场景', () => import('../v3-domain/world-details.js').then(module => module.renderScene), 'renderScene');
add('/c/:charId/world/settings', '世界', '世界事件设置', () => import('../v3-domain/world-details.js').then(module => module.renderWorldSettings), 'renderWorldSettings');
add('/c/:charId/world/event/:evId', '世界', '世界事件详情', () => import('../v3-domain/world-events.js').then(module => module.renderRealWorldEvent), 'renderRealWorldEvent');
add('/c/:charId/world/items/:itemId', '世界', '世界物品详情', () => import('../v3-domain/world-items.js').then(module => module.renderWorldItem), 'renderWorldItem');
add('/c/:charId/world/items', '世界', '世界物品目录', () => import('../v3-domain/world-items.js').then(module => module.renderWorldItems), 'renderWorldItems');
add('/c/:charId/people/npc/:npcId/edit', '人物关系', '编辑世界人物', () => import('../react-app/pages/PeoplePage.jsx').then(module => module.default), 'PeoplePage', {native:'people',initialEdit:true});
add('/c/:charId/people/npc/new', '人物关系', '创建世界人物', () => import('../react-app/pages/PeoplePage.jsx').then(module => module.default), 'PeoplePage', {native:'people',initialCreate:true});
add('/c/:charId/people/npcs', '人物关系', '管理世界人物', () => import('../react-app/pages/PeoplePage.jsx').then(module => module.default), 'PeoplePage', {native:'people'});
add('/c/:charId/people/list', '人物关系', '人物目录', () => import('../react-app/pages/PeoplePage.jsx').then(module => module.default), 'PeoplePage', {native:'people'});
add('/c/:charId/people/npc/:npcId', '人物关系', '人物详情与接触记录', () => import('../react-app/pages/PeoplePage.jsx').then(module => module.default), 'PeoplePage', {native:'people'});
add('/c/:charId/people/relation/:relId', '人物关系', '关系修订', () => import('../v3-domain/world-details.js').then(module => module.renderRelEdit), 'renderRelEdit');
add('/c/:charId/people/owner', '人物关系', '关于你', () => import('../react-app/pages/PeoplePage.jsx').then(module => module.default), 'PeoplePage', {native:'people',initialTab:'you'});
add('/diag/task/:taskId', '管理中心', '原任务与恢复', () => import('../react-app/pages/TaskDetailPage.tsx').then(module => module.default), 'TaskDetailPage', {native:'task'});
add('/diag/request/:requestId', '管理中心', '请求处理详情', () => import('../v3-domain/operations.js').then(module => module.renderRealRequest), 'renderRealRequest');
add('/diag/workflow/:workflowId', '管理中心', '工作流执行与恢复', () => import('../react-app/shared/WorkflowRecovery.tsx').then(module => module.default), 'WorkflowRecovery', {native:'workflow'});
add('/diag/record/:kind/:recordRef', '管理中心', '完整诊断记录', () => import('../v3-domain/diagnostics.js').then(module => module.renderRealDiagnosticRecord), 'renderRealDiagnosticRecord');
add('/diag/pending', '通知与待处理', '完整待处理事项', () => import('../v3-domain/operations.js').then(module => module.renderRealPending), 'renderRealPending');
add('/diag/:section', '管理中心', '运行诊断', () => import('../v3-domain/diagnostics.js').then(module => module.renderRealDiagnosticRecords), 'renderRealDiagnosticRecords');
add('/data/backup/:backupId', '管理中心', '备份与恢复记录', () => import('../v3-domain/backups.js').then(module => module.renderRealBackup), 'renderRealBackup');
add('/data/pack/:packId', '管理中心', '导入草稿', () => import('../v3-domain/data-tools.js').then(module => module.renderRealPack), 'renderRealPack');
add('/data/revisions', '管理中心', '对象修订记录', () => import('../v3-domain/data-tools.js').then(module => module.renderRealRevisions), 'renderRealRevisions');
add('/data/rollback', '管理中心', '时间线回档说明', () => import('../v3-domain/data-tools.js').then(module => module.renderRealRollback), 'renderRealRollback');
add('/data', '管理中心', '数据管理', () => import('../v3-domain/data-tools.js').then(module => module.renderRealData), 'renderRealData');
export const featureRouteInventory = routes.map(({path,page,title,native}) => ({path,page,title,native}));
export function featureRoute(path, query) {
  for (const route of routes) {
    const match = route.pattern.exec(path);
    if (!match) continue;
    const params = Object.fromEntries(route.keys.map((key,index) => [key,decodeURIComponent(match[index+1])]));
    return {...route,params,fid:params.charId || query.get('fid') || null};
  }
  return null;
}
