import {get} from '../store.js';
import {studioDraftStorage} from './studio-draft-storage.js';

// Presentation drafts only. Acknowledgement removes exactly the submitted
// snapshot, so an old replay cannot erase newer edits on the same object.
export function wardrobeDraft(scope,initial){
 const key=`fluctlight-wardrobe:${get('session')?.actorId}:draft:${JSON.stringify(scope)}`,storage=studioDraftStorage();
 const raw=storage.getItem(key);let value=raw?JSON.parse(raw):structuredClone(initial);
 return {get:()=>structuredClone(value),set(next){value=structuredClone(next);storage.setItem(key,JSON.stringify(value));},acknowledge(submitted){const current=storage.getItem(key);if(current&&JSON.stringify(JSON.parse(current))===JSON.stringify(submitted))storage.removeItem(key);}};
}
