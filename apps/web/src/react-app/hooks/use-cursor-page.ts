import {useCallback,useEffect,useRef,useState} from 'react';
import {errorMessage} from '@/v3/domain-presentation.mjs';
export function useCursorPage<T=any>(read:(cursor:string)=>Promise<{items:T[];next_cursor?:string}>,scope:string){
 const reader=useRef(read),scopeRef=useRef(scope),active=useRef(false),epoch=useRef(0);reader.current=read;scopeRef.current=scope;
 const [page,setPage]=useState<{items:T[];next_cursor:string;cursors:string[];index:number}>({items:[],next_cursor:'',cursors:[''],index:0}),[loading,setLoading]=useState(false),[error,setError]=useState('');
 const load=useCallback(async(cursor='',index=0,cursors=[''])=>{const token=++epoch.current,identity=scopeRef.current;setLoading(true);setError('');try{const result=await reader.current(cursor);if(!active.current||token!==epoch.current||identity!==scopeRef.current)return;if(!Array.isArray(result.items))throw Error('记录列表格式无法读取');setPage({items:result.items,next_cursor:result.next_cursor||'',index,cursors});}catch(cause){if(active.current&&token===epoch.current&&identity===scopeRef.current)setError(errorMessage(cause));}finally{if(active.current&&token===epoch.current&&identity===scopeRef.current)setLoading(false);}},[]);
 useEffect(()=>{active.current=true;void load();return()=>{active.current=false;epoch.current++;};},[scope,load]);
 return {...page,loading,error,reload:()=>load(),retry:()=>load(page.cursors[page.index],page.index,page.cursors),next:()=>page.next_cursor&&load(page.next_cursor,page.index+1,[...page.cursors.slice(0,page.index+1),page.next_cursor]),previous:()=>page.index>0&&load(page.cursors[page.index-1],page.index-1,page.cursors)};
}
