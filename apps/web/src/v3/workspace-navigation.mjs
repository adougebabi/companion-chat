export const exchangeNavigation = [
 {k:'chat',label:'对话',icon:'chat',path:'chat'},
 {k:'life',label:'生活',icon:'life',path:'life/now',subs:[['此刻','life/now','now'],['日程','life/schedule','calendar'],['目标','life/goals','target'],['衣柜','life/wardrobe','shirt'],['日记','life/diary','book'],['共同经历','life/shared','heart']]},
 {k:'world',label:'世界',icon:'globe',path:'world'}, {k:'people',label:'人物关系',icon:'people',path:'people'},
 {k:'growth',label:'记忆成长',icon:'brain',path:'growth'}, {k:'media',label:'媒体',icon:'image',path:'media'}, {k:'profile',label:'角色资料',icon:'user',path:'profile'},
];
export const managementNavigation=[
 {k:'profile',label:'角色资料',icon:'user',path:'profile'}, {k:'worlds',label:'世界管理',icon:'globe',path:'worlds'},
 {k:'people',label:'人物管理',icon:'people',path:'people/npcs'}, {k:'governance',label:'身份与能力治理',icon:'brain',path:'governance'},
 {k:'autonomy',label:'自治设置',icon:'settings',path:'settings/autonomy'},
];
export function sidebarSectionPath(path){
 return path
  .replace(/^(\/c\/[^/]+)\/worlds(?=\/|$)/,'$1/world')
  .replace(/^(\/c\/[^/]+)\/life\/(?:goal|planning)(?=\/|$)/,'$1/life/goals')
  .replace(/^(\/c\/[^/]+)\/life$/,'$1/life/now')
  .replace(/^(\/c\/[^/]+)\/governance(?=\/|$)/,'$1/growth')
  .replace(/^\/(diag|data|setup)(?=\/|$)/,'/settings/$1');
}
export function workspaceForPath(path){
 if(path==='/library/create')return '管理';
 if(/^\/(settings|diag|data|studio|setup)(\/|$)/.test(path))return '管理';
 if(/^\/c\/[^/]+\/(worlds|governance|settings)(\/|$)/.test(path)||/\/people\/(manage|npcs|npc\/new|npc\/[^/]+\/edit)(\/|$)/.test(path))return '管理';
 if(path==='/auth'||path==='/inbox'||path==='/library'||/\/profile$/.test(path))return null;
 return '交流';
}
export function globalNavigation(mode){return mode==='管理'
 ? [['角色库','#/library','home'],['初始化向导','#/setup','route'],['创作中心','#/studio','studio'],['系统设置','#/settings','settings'],['运行诊断','#/diag','diag'],['数据工具','#/data','book'],['通知','#/inbox','inbox']]
 : [['角色库','#/library','home'],['动态','#/feed','feed'],['通知','#/inbox','inbox']];}

// Only module/list paths migrate; object, conversation and source identities do not.
export function characterSwitchHref(raw, characterId) {
 const [path,search='']=String(raw||'').replace(/^#/,'').split('?');
 const section=path.replace(/^\/c\/[^/]+\//,'');
 const portable=new Set(['chat','chat/history','life','life/now','life/schedule','life/goals','life/wardrobe','life/diary','life/shared','life/planning','world','world/events','people','people/npcs','growth','growth/reflection','growth/self','governance','media','profile','settings/autonomy']);
 if(!/^\/c\/[^/]+\//.test(path))return '#'+path+(search?'?'+search:'');
 const moduleFallback=[['growth/','growth'],['world/','world'],['media/','media'],['people/','people'],['life/diary/','life/diary'],['life/goal/','life/goals'],['life/','life/now'],['governance/','governance']].find(([prefix])=>section.startsWith(prefix))?.[1];
 const target=portable.has(section)?section:moduleFallback||'chat';
 const query=new URLSearchParams(search);
 for(const key of [...query.keys()])if(!['q','mode','aboutOwner','type','status','state','s','month','date','view','tab','kind','scope','hidden','fromDate','toDate'].includes(key))query.delete(key);
 return '#/c/'+encodeURIComponent(characterId)+'/'+target+(query.size?'?'+query:'');
}
