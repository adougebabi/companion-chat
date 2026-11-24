import {useEffect,useState} from 'react';
import {useWorkspace} from '../page';
import {taskSubtitle,taskDisplayTitle} from '@/approved-ui/task-presentation.mjs';
import {ResponsiveSheet} from '@/components/ui/responsive-sheet';
import WorkspaceDialog from './WorkspaceDialog';
import TaskDetailPage from '../pages/TaskDetailPage';
export default function TaskRecordDialog({taskId,returnTo,onClose,returnFocus,presentation='sheet'}:{taskId:string;returnTo?:string;onClose:()=>void;returnFocus?:()=>HTMLElement|null;presentation?:'dialog'|'sheet'}){
 const [task,setTask]=useState<any>(null),[currentTaskId,setCurrentTaskId]=useState(taskId),[controlTarget,setControlTarget]=useState<HTMLDivElement|null>(null);
 useEffect(()=>{setCurrentTaskId(taskId);},[taskId]);
 const w=useWorkspace(),title=taskDisplayTitle(task,w.state.characters);
 const description=task?taskSubtitle(task):'正在读取任务…';
 const detail=<TaskDetailPage key={currentTaskId} embedded controlTarget={controlTarget} onTaskLoaded={setTask} onTaskRecovered={id=>{setTask(null);setCurrentTaskId(id);}} route={{feature:{params:{taskId:currentTaskId}},query:new URLSearchParams(returnTo?{returnTo}:{})}}/>;
 // Layout is owned by the shared ResponsiveSheet (wide); the task footer target
 // is the sheet footer, outside the scrolling body.
 if(presentation==='sheet')return <ResponsiveSheet open onOpenChange={open=>!open&&onClose()} size="wide" title={title} description={description} className="react-workspace replica-overlay replica-creation-task-sheet" bodyClassName="creation-task-sheet-body" footer={<div ref={setControlTarget} className="replica-task-footer-target"/>} onCloseAutoFocus={event=>{
   const source=returnFocus?.(),target=source?.isConnected&&!source.matches(':disabled,[aria-disabled="true"]')?source:document.querySelector<HTMLElement>('.replica-page-heading h1');
   if(target?.isConnected){event.preventDefault();target.focus({preventScroll:true});}
 }}>{detail}</ResponsiveSheet>;
 return <WorkspaceDialog open wide footer={<div ref={setControlTarget} className="replica-task-footer-target"/>} title={title} description={description} returnFocus={returnFocus} onOpenChange={open=>!open&&onClose()}>{detail}</WorkspaceDialog>;
}
