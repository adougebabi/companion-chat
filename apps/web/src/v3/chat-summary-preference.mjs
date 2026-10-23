// UI preference only; scoped to the Owner and character, across conversations.
export function chatSummaryPreference({owner,fid,storage=globalThis.localStorage,defaultFolded=!(globalThis.matchMedia?.('(min-width:1000px)').matches??true)}){
 const key='fluctlight:chat-summary:'+JSON.stringify([owner,fid]);
 return {read(){try{const value=storage?.getItem(key);return value==='collapsed'?true:value==='expanded'?false:defaultFolded;}catch{return defaultFolded;}},save(folded){try{storage?.setItem(key,folded?'collapsed':'expanded');}catch{/* Mounted choice remains available. */}}};
}
