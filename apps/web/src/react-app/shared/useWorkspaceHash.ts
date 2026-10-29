import {useSyncExternalStore} from 'react';
const read=()=>globalThis.location?.hash||'';
const subscribe=(notify:()=>void)=>{window.addEventListener('hashchange',notify);return()=>window.removeEventListener('hashchange',notify);};
export default function useWorkspaceHash(){return useSyncExternalStore(subscribe,read,()=>'');}
