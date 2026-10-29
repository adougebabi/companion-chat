import WorkspaceDialog from './WorkspaceDialog';
import TaskDetailPage from '../pages/TaskDetailPage';
export default function TaskRecordDialog({taskId,returnTo,onClose,returnFocus}:{taskId:string;returnTo?:string;onClose:()=>void;returnFocus?:()=>HTMLElement|null}){
 return <WorkspaceDialog open title="任务记录" description="失败原因、实际执行记录和原业务恢复入口会分别保留。" returnFocus={returnFocus} onOpenChange={open=>!open&&onClose()}><TaskDetailPage key={taskId} embedded route={{feature:{params:{taskId}},query:new URLSearchParams(returnTo?{returnTo}:{})}}/></WorkspaceDialog>;
}
