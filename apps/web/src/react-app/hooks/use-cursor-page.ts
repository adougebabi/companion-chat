import {useCallback,useEffect,useRef,useState} from 'react';
import {errorRecord,readErrorMessage} from '@/v3/domain-presentation.mjs';

export function useCursorPage<T=any>(read:(cursor:string)=>Promise<{items:T[];next_cursor?:string}>,scope:string){
 const reader=useRef(read),scopeRef=useRef(scope),active=useRef(false),epoch=useRef(0);
 reader.current=read;
 scopeRef.current=scope;
 const [page,setPage]=useState<{items:T[];next_cursor:string;cursors:string[];index:number}>({items:[],next_cursor:'',cursors:[''],index:0});
 const [loading,setLoading]=useState(false);
 const [problem,setProblem]=useState<{scope:string;error:string;failure:unknown}|null>(null);
 const load=useCallback(async(cursor='',index=0,cursors=[''])=>{
  const token=++epoch.current,identity=scopeRef.current;
  const current=()=>active.current&&token===epoch.current&&identity===scopeRef.current;
  setLoading(true);
  setProblem(null);
  try{
   const result=await reader.current(cursor);
   if(!current())return;
   if(!Array.isArray(result.items))throw Error('记录列表格式无法读取');
   setPage({items:result.items,next_cursor:result.next_cursor||'',index,cursors});
  }catch(cause){
   if(current())setProblem({scope:identity,error:readErrorMessage(cause),failure:errorRecord(cause)});
  }finally{
   if(current())setLoading(false);
  }
 },[]);
 useEffect(()=>{active.current=true;void load();return()=>{active.current=false;epoch.current++;};},[scope,load]);
 const currentProblem=problem?.scope===scope?problem:null;
 return {
  ...page,loading,error:currentProblem?.error||'',failure:currentProblem?.failure,
  reload:()=>load(),
  retry:()=>load(page.cursors[page.index],page.index,page.cursors),
  next:()=>page.next_cursor&&load(page.next_cursor,page.index+1,[...page.cursors.slice(0,page.index+1),page.next_cursor]),
  previous:()=>page.index>0&&load(page.cursors[page.index-1],page.index-1,page.cursors),
 };
}
