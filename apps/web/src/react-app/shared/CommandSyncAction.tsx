import {MoreHorizontal} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {DropdownMenu,DropdownMenuTrigger,DropdownMenuContent,DropdownMenuItem} from '@/components/ui/dropdown-menu';

// Synchronization is secondary until a real conflict requires review.
export default function CommandSyncAction({state,onReview,placement='footer'}:{state:any;onReview:()=>unknown;placement?:'footer'|'recovery'}){
 if(state.review||state.readbackPending||(state.pending&&!state.pending.rejection))return null;
 const required=state.needsRebase||!!state.pending?.rejection;
 if(required)return placement==='recovery'?<Button type="button" variant="outline" disabled={state.busy} onClick={()=>void onReview()}>同步资料，保留输入</Button>:null;
 if(placement!=='footer')return null;
 return <DropdownMenu><DropdownMenuTrigger asChild><Button type="button" variant="ghost" size="icon" aria-label="更多表单操作" disabled={state.busy}><MoreHorizontal size={16}/></Button></DropdownMenuTrigger><DropdownMenuContent align="start"><DropdownMenuItem onSelect={()=>void onReview()}>同步资料，保留输入</DropdownMenuItem></DropdownMenuContent></DropdownMenu>;
}
