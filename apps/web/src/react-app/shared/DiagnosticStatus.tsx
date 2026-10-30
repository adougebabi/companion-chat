import {Badge} from '../controls';
import {stateLabel} from '@/v3/domain-presentation.mjs';
import '../../styles/replica-diagnostics.css';
type Tone='neutral'|'green'|'amber'|'red'|'blue';
export default function DiagnosticStatus({status,label,tone}:{status?:string;label?:string;tone?:Tone}){
 const resolved=tone||(['completed','succeeded','success'].includes(status||'')?'green':['failed','error','dead_letter'].includes(status||'')?'red':['paused','blocked'].includes(status||'')?'amber':['running','processing'].includes(status||'')?'blue':'neutral');
 return <Badge className="replica-diagnostic-status" data-tone={resolved}><span aria-hidden="true"/>{label||stateLabel(status,'状态未记录')}</Badge>;
}
