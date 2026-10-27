// Read status is UI metadata; domain data and command identity remain authoritative.
const families={此刻:['detail','life','schedule','promises'],日程:['detail','life','schedule','promises'],目标:['detail','life','goals','promises'],日记:['detail','life','diary'],记忆:['detail','life','memory'],衣柜:['wardrobe'],世界:['world'],人物关系:['人物关系'],动态:['动态'],通知与待处理:['通知与待处理']};
const primary={此刻:'detail',日程:'schedule',日记:'diary',记忆:'memory',衣柜:'wardrobe',世界:'world',人物关系:'人物关系',动态:'动态',通知与待处理:'通知与待处理'};
const names={detail:'角色资料',life:'生活记录',schedule:'日程',goals:'目标',diary:'日记',memory:'记忆',promises:'约定',wardrobe:'衣柜',world:'世界'};
export function currentReadError(state,key,scope){return state.errors?.[key]&&(!state.errorScopes?.[key]||state.errorScopes[key]===scope)?state.errors[key]:'';}
/** @returns {{errors:Array<{key:string,label:string,message:string}>,initialFailure:boolean,loading:boolean}} */
export function workspaceReadFeedback(state,page,scope){const keys=families[page]||[],errors=keys.flatMap(key=>{const message=currentReadError(state,key,scope);return message?[{key,label:names[key]||key,message}]:[];}),key=primary[page];return{errors,initialFailure:!!key&&errors.some(error=>error.key===key)&&!state.readSuccesses?.[scope]?.[key],loading:keys.some(key=>state.loading?.[key])};}
