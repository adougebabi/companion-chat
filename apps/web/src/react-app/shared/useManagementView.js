import {useEffect,useState} from 'react';
import {useWorkspace} from '../page';
// Owner-scoped display preferences only. Never stores records or command bodies.
export default function useManagementView(name,defaults){
 const w=useWorkspace(),owner=w.state.session?.actorId,key=`fluctlight:management-view:${owner}:${name}`;
 const [value,setValue]=useState(()=>{try{const saved=JSON.parse(sessionStorage.getItem(key)||'null');return Object.fromEntries(Object.entries(defaults).map(([field,fallback])=>[field,typeof saved?.[field]===typeof fallback?saved[field]:fallback]));}catch{return defaults;}});
 useEffect(()=>{try{if(w.state.session?.actorId===owner)sessionStorage.setItem(key,JSON.stringify(value));else sessionStorage.removeItem(key);}catch{/* Display preferences are optional. */}},[key,value,w,owner]);
 useEffect(()=>()=>{if(w.state.session?.actorId!==owner)try{sessionStorage.removeItem(key);}catch{}},[key,w,owner]);
 return [value,patch=>setValue(old=>({...old,...patch}))];
}
