import {useState} from 'react';
import {useWorkspace} from '../page';
import {diagnosticInstant,taskDisplayTitle} from '@/approved-ui/task-presentation.mjs';
import WorkspaceDialog from './WorkspaceDialog';
import TaskDetailPage from '../pages/TaskDetailPage';
export default function TaskRecordDialog({taskId,returnTo,onClose,returnFocus}:{taskId:string;returnTo?:string;onClose:()=>void;returnFocus?:()=>HTMLElement|null}){
 const [task,setTask]=useState<any>(null);
 const w=useWorkspace(),title=taskDisplayTitle(task,w.state.characters);
 return <WorkspaceDialog open wide title={title} description={task?`${task.task_id||taskId}${task.created_at?' · '+diagnosticInstant(task.created_at):''}`:'正在读取任务与关联模型调用…'} returnFocus={returnFocus} onOpenChange={open=>!open&&onClose()}><TaskDetailPage key={taskId} embedded onTaskLoaded={setTask} route={{feature:{params:{taskId}},query:new URLSearchParams(returnTo?{returnTo}:{})}}/></WorkspaceDialog>;
}
