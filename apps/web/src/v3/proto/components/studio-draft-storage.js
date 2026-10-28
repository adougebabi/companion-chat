import {get,subscribe} from '../store.js';

// Refresh recovery keeps submitted identities and UI drafts in this session.
// Logout/Owner switch removes them; these records are never business results.
let previous=get('session')?.actorId;
subscribe(()=>{const next=get('session')?.actorId;if(previous&&previous!==next){
 for(const key of Object.keys(sessionStorage))if(key.startsWith('fluctlight:approved-config-command:['+JSON.stringify(previous)+',')||key.startsWith('fluctlight:approved-policy:['+JSON.stringify(previous)+',')||key.startsWith(`fluctlight:management-view:${previous}:`)||key.startsWith('fluctlight.chat-draft:['+JSON.stringify(previous)+',')||key.startsWith('fluctlight:pending-schedule:['+JSON.stringify(previous)+',')||key.startsWith(`fluctlight-setup:${previous}`)||key.startsWith(`fluctlight-studio:${previous}:`)||key.startsWith(`fluctlight-world-event:${previous}:`)||key.startsWith(`fluctlight-creation:${previous}:`)||key.startsWith(`fluctlight-media:${previous}:`)||key.startsWith(`fluctlight-wardrobe:${previous}:`)||key.startsWith(`fluctlight-world-framework:${previous}:`)||key.startsWith(`fluctlight-runtime-framework:${previous}:`)||key.startsWith(`fluctlight-goals:${previous}:`)||key.startsWith(`fluctlight-diary:${previous}:`)||key.startsWith(`fluctlight-memory:${previous}:`)||key.startsWith(`fluctlight-profile:${previous}:`)||key.startsWith(`fluctlight-people:${previous}:`))sessionStorage.removeItem(key);
}previous=next;});
export const studioDraftStorage=()=>sessionStorage;
