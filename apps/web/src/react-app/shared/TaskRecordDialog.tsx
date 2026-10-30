import {useState} from 'react';
import {useWorkspace} from '../page';
import {diagnosticDateLabel,taskDisplayTitle} from '@/approved-ui/task-presentation.mjs';
import {Sheet,SheetContent,SheetHeader,SheetTitle,SheetDescription,SheetFooter} from '@/components/ui/sheet';
import WorkspaceDialog from './WorkspaceDialog';
import TaskDetailPage from '../pages/TaskDetailPage';
export default function TaskRecordDialog({taskId,returnTo,onClose,returnFocus,presentation='dialog'}:{taskId:string;returnTo?:string;onClose:()=>void;returnFocus?:()=>HTMLElement|null;presentation?:'dialog'|'sheet'}){
 const [task,setTask]=useState<any>(null),[controlTarget,setControlTarget]=useState<HTMLDivElement|null>(null);
 const w=useWorkspace(),title=taskDisplayTitle(task,w.state.characters);
 const description=task?`${task.task_id||taskId}${task.created_at?' · '+diagnosticDateLabel(task.created_at):''}`:'正在读取任务…';
 const detail=<TaskDetailPage key={taskId} embedded controlTarget={controlTarget} onTaskLoaded={setTask} route={{feature:{params:{taskId}},query:new URLSearchParams(returnTo?{returnTo}:{})}}/>;
 if(presentation==='sheet')return <Sheet open onOpenChange={open=>!open&&onClose()}><SheetContent className="react-workspace replica-overlay replica-creation-task-sheet" onCloseAutoFocus={event=>{
   const source=returnFocus?.(),target=source?.isConnected&&!source.matches(':disabled,[aria-disabled="true"]')?source:document.querySelector<HTMLElement>('.replica-page-heading h1');
   if(target?.isConnected){event.preventDefault();target.focus({preventScroll:true});}
 }}><SheetHeader><SheetTitle>{title}</SheetTitle><SheetDescription>{description}</SheetDescription></SheetHeader><div className="creation-task-sheet-body">{detail}</div><SheetFooter><div ref={setControlTarget} className="replica-task-footer-target"/></SheetFooter></SheetContent></Sheet>;
 return <WorkspaceDialog open wide footer={<div ref={setControlTarget} className="replica-task-footer-target"/>} title={title} description={description} returnFocus={returnFocus} onOpenChange={open=>!open&&onClose()}>{detail}</WorkspaceDialog>;
}
