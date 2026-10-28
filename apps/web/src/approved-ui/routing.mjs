import {featureRoute} from './feature-routes.mjs';
export const lifePages = [
  "此刻",
  "日程",
  "目标",
  "衣柜",
  "日记",
  "记忆",
  "角色详情",
];
export const workspacePages = [
  "聊天",
  ...lifePages,
  "世界",
  "动态",
  "人物关系",
  "媒体",
  "通知与待处理",
  "角色库",
  "创作中心",
  "初始化角色",
  "管理中心",
];
const modules = {
  chat: "聊天",
  "life/now": "此刻",
  "life/schedule": "日程",
  "life/goals": "目标",
  "life/wardrobe": "衣柜",
  "life/diary": "日记",
  growth: "记忆",
  world: "世界",
  people: "人物关系",
  media: "媒体",
  profile: "角色详情",
};
export function approvedRoute(hash) {
  const value=String(hash||'').replace(/^#/,'');
  let [path,search='']=value.split('?');
  const query=new URLSearchParams(search);
  if(query.getAll('legacy').includes('1'))return {removed:true,page:'页面未找到',path,query,fid:null};
  try { decodeURIComponent(path); }
  catch { return {page:'页面未找到',notFound:true,path,query,fid:null}; }
  const aliases={'/instances':'/library','/moments':'/feed','/diagnostics':'/diag','/settings/diagnostics':'/diag','/backup':'/data','/setup':'/library/create','/studio/media':'/media'};
  path=aliases[path]||path;
  if(path==='/studio/template'){path='/studio';query.set('kind','event_template');}
  if(path==='/studio/apply')path=query.get('res')?'/studio/edit/'+encodeURIComponent(query.get('res')):'/studio';
  const autonomy=/^\/c\/([^/]+)\/settings\/autonomy$/.exec(path);
  if(autonomy){path='/settings';query.set('fid',decodeURIComponent(autonomy[1]));query.set('section','角色自主与主动联系');}
  if(path.startsWith('/studio/media/'))path='/media/'+path.slice('/studio/media/'.length);
  if(path.startsWith('/settings/')){
    const section=path.slice('/settings/'.length);
    query.set('section',({models:'服务地址','reply-vision':'聊天回复','media-workflows':'图片（ComfyUI）',runtime:'角色自主与主动联系',outreach:'角色自主与主动联系',notifications:'通知',owner:'所有者与会话',capabilities:'能力与权限',kev:'高级'})[section]||section);
    path='/settings';
  }
  if(path==='/auth')return {page:'登录',auth:true,path,query};
  if(path==='/media')return {page:'媒体',path,query,fid:null,global:true};
  if(path==='/worlds')return {page:'世界',path,query,fid:null,global:true};
  if (!path || workspacePages.includes(path))
    return { page: path || "角色库", fid: null, path, query, alias: true };
  const match = /^\/c\/([^/]+)\/(.*)$/.exec(path);
  if(match&&match[2]==='chat/media')return {page:'媒体',fid:null,path,query:new URLSearchParams({...Object.fromEntries(query),fid:decodeURIComponent(match[1])}),global:true};
  if(match&&['chat/call','chat/corrections'].includes(match[2]))return {page:'聊天',fid:decodeURIComponent(match[1]),path,query,information:match[2]==='chat/call'?{title:'文字交流',body:'当前版本支持文字和已有媒体消息，暂未提供实时通话。',page:'聊天',label:'返回聊天'}:{title:'纠正与修订记录',body:'可以从记忆详情查看原始来源和每一版修订；聊天原文保留。',page:'记忆',label:'查看记忆与来源'}};
  if(match&&['life','worlds'].includes(match[2])){
    const suffix=match[2]==='life'?'life/now':'world';
    return {page:modules[suffix],fid:decodeURIComponent(match[1]),path,query};
  }
  if (match && modules[match[2]])
    return { page: modules[match[2]], fid: decodeURIComponent(match[1]), path, query };
  const global = {
    "/feed": "动态",
    "/inbox": "通知与待处理",
    "/library": "角色库",
    "/library/create": "初始化角色",
    "/studio": "创作中心",
    "/settings": "管理中心",
    "/diag": "管理中心",
  };
  if (global[path])
    return {
      page: global[path],
      path,
      fid: query.get("fid"),
      query,
      diagnostics: path === "/diag",
    };
  try {
    const feature=featureRoute(path,query);
    if(feature)return {page:feature.page,title:feature.title,path,query,fid:feature.fid,feature};
  }catch { /* Invalid encoded selectors stay inside the same not-found view. */ }
  return {page:'页面未找到',notFound:true,path,query,fid:null};
}
export function approvedHref(page, fid) {
  if(page==='媒体')return '#/media';
  if(page==='世界'&&!fid)return '#/worlds';
  const module = Object.entries(modules).find(
    ([, label]) => label === page,
  )?.[0];
  if (module)
    return fid ? "#/c/" + encodeURIComponent(fid) + "/" + module : "#/library";
  return (
    {
      动态: "#/feed" + (fid ? "?fid=" + encodeURIComponent(fid) : ""),
      通知与待处理: "#/inbox",
      角色库: "#/library",
      初始化角色: "#/library/create",
      创作中心: "#/studio",
      管理中心: "#/settings",
    }[page] || "#/library"
  );
}
export function bookmarkedPath(pathname,search='') {
  const path=String(pathname||'').replace(/\/$/,'')||'/';
  if(path==='/')return null;
  return ({'/chat':'/library','/world':'/worlds','/people':'/library'})[path]||path+search;
}

// UI reading position belongs to an Owner and the complete route/filter context.
export function workspaceReadingKey(owner, route) {
  const query = [...(route.query || new URLSearchParams()).entries()];
  query.sort(([left], [right]) => left.localeCompare(right));
  return JSON.stringify([owner, route.path || route.page, route.fid || null, query]);
}
