import {get,subscribe} from '../store.js';

// Refresh recovery keeps submitted identities and UI drafts in this session.
// Logout/Owner switch removes them; these records are never business results.
let previous=get('session')?.actorId;
subscribe(()=>{const next=get('session')?.actorId;if(previous&&previous!==next){
 for(const key of Object.keys(sessionStorage))if(key.startsWith(`fluctlight-studio:${previous}:`)||key.startsWith(`fluctlight-world-event:${previous}:`)||key.startsWith(`fluctlight-creation:${previous}:`)||key.startsWith(`fluctlight-media:${previous}:`)||key.startsWith(`fluctlight-wardrobe:${previous}:`)||key.startsWith(`fluctlight-world-framework:${previous}:`)||key.startsWith(`fluctlight-runtime-framework:${previous}:`))sessionStorage.removeItem(key);
}previous=next;});
export const studioDraftStorage=()=>sessionStorage;
