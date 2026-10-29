import {useState} from 'react';
import Combobox from './Combobox';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {RefreshCw} from 'lucide-react';
import {Feedback} from './Presentation';
import {diagnosticInstant} from '@/approved-ui/task-presentation.mjs';
export default function ModelDirectory({models=[],status,error,onRetry,readAt}){
 const [selected,setSelected]=useState(''),names=[...new Set(models.map(model=>typeof model==='string'?model:model.id||model.name).filter(Boolean))];
 return <div className="replica-model-directory"><div className="replica-model-directory-head"><span>{readAt?'读取于 '+diagnosticInstant(readAt):'尚未同步模型目录'}</span>{typeof onRetry==='function'&&<Button type="button" variant="outline" size="sm" disabled={status==='loading'} onClick={onRetry}><RefreshCw size={13}/>{status==='loading'?'同步中…':'同步模型目录'}</Button>}</div>
 {status==='loading'&&<p role="status" data-feedback-kind="progress">正在读取已保存连接的模型目录…</p>}
 {error&&status!=='loading'&&<Feedback kind="warning" title="模型目录未同步" className="replica-feedback-compact" details={<pre>{error}</pre>}>{error} 已保存的连接保留，用途分配仍支持手动填写模型 ID。</Feedback>}
 {names.length>0&&<><div className="replica-model-tags">{names.slice(0,6).map(name=><Badge key={name} variant="secondary">{name}</Badge>)}</div><Combobox label={status==='stale'?'上次已保存连接的模型目录':'可用模型目录'} placeholder={'浏览模型目录 · '+names.length+' 项'} value={selected} options={names} onChange={setSelected}/></>}
 {!names.length&&status!=='loading'&&!error&&<p>{status==='ready'?'服务没有返回模型目录。':status==='stale'?'连接已修改，保存后可重新同步目录。':'同步后可以浏览服务返回的模型。'}可在用途分配手动填写模型 ID。</p>}
 </div>;
}
