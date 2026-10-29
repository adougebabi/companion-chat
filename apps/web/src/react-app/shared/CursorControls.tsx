import {Button} from '@/components/ui/button';
import {Pagination,PaginationContent,PaginationItem} from '@/components/ui/pagination';
import {ChevronLeftIcon,ChevronRightIcon} from 'lucide-react';
import {InlineFeedback,Feedback} from './Presentation';
type AppendControls={loading:boolean;hasMore:boolean;hasContent?:boolean;loadMore:()=>unknown;error?:string;retry:()=>unknown};
type Props={label?:string}&({page:any;append?:never}|{page?:never;append:AppendControls});
export default function CursorControls({page,append,label='记录'}:Props){
 if(append){
  if(!append.hasMore&&!append.error&&!append.loading)return null;
  return <div className="grid min-w-0 gap-3">{append.loading&&!append.hasMore&&<Feedback kind="progress">正在读取{label}…</Feedback>}{append.hasMore&&<Pagination aria-label={label+'追加读取'} className="justify-start"><PaginationContent className="flex-wrap">{append.hasMore&&<PaginationItem><Button className="touch-target" type="button" variant="outline" disabled={append.loading} onClick={()=>void append.loadMore()}>{append.loading?'正在读取更多'+label+'…':'加载更多'+label}</Button></PaginationItem>}</PaginationContent></Pagination>}{append.error&&!append.loading&&<Feedback kind={append.hasContent?'warning':'error'} title={label+'暂未读全'} actions={<Button className="touch-target" type="button" variant="outline" onClick={()=>void append.retry()}>重新读取{label}</Button>}>{append.error}{append.hasContent?' 此前读取的记录仍保留。':''}</Feedback>}</div>;
 }
return <div className="grid min-w-0 gap-3"><Pagination aria-label={label+'分页'} className="justify-start"><PaginationContent className="flex-wrap"><PaginationItem><Button type="button" variant="outline" disabled={page.loading||page.index===0} onClick={()=>void page.previous()}><ChevronLeftIcon/>上一页{label}</Button></PaginationItem><PaginationItem><Button type="button" variant="outline" disabled={page.loading||!page.next_cursor} onClick={()=>void page.next()}>下一页{label}<ChevronRightIcon/></Button></PaginationItem></PaginationContent></Pagination>{page.error&&<InlineFeedback>{page.error}，此前读取的记录仍保留。<Button type="button" variant="outline" disabled={page.loading} onClick={()=>void page.retry()}>重新读取本页</Button></InlineFeedback>}</div>;}
