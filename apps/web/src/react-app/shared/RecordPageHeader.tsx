import {ArrowLeft, RefreshCw} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Tooltip, TooltipContent, TooltipProvider, TooltipTrigger} from '@/components/ui/tooltip';
import WorkspacePageHeader from './WorkspacePageHeader';
import './record-page-header.css';

export default function RecordPageHeader({title, description, eyebrow='身份与成长', back, backLabel='返回', read}:any) {
  const reloadLabel = '重新读取' + title;
  return <div className="record-page-header">
    {back && <nav aria-label="返回上级" className="mb-3"><Button asChild variant="link" className="h-auto max-w-full justify-start whitespace-normal p-0"><a href={'#'+String(back).replace(/^#/, '')}><ArrowLeft aria-hidden="true" size={14}/>{backLabel}</a></Button></nav>}
    <WorkspacePageHeader eyebrow={eyebrow} title={title} description={description} actions={read && <TooltipProvider><Tooltip><TooltipTrigger asChild><Button type="button" variant="ghost" size="icon" aria-label={reloadLabel} disabled={read.loading} onClick={()=>void Promise.resolve().then(()=>read.reload()).catch(()=>{})}><RefreshCw aria-hidden="true" size={16} className={read.loading?'animate-spin':undefined}/></Button></TooltipTrigger><TooltipContent>{reloadLabel}</TooltipContent></Tooltip></TooltipProvider>}/>
  </div>;
}
