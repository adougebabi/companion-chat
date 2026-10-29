import {useState} from 'react';
import Combobox from './Combobox';
import {Button} from '@/components/ui/button';
import {Feedback} from './Presentation';
export default function ModelDirectory({models=[],status,error,onRetry}){
 const [selected,setSelected]=useState(''),names=[...new Set(models.map(model=>typeof model==='string'?model:model.id||model.name).filter(Boolean))];
 return <div className="grid min-w-0 gap-2">{status==='loading'&&<Feedback kind="progress">正在同步模型目录…</Feedback>}{error&&status!=='loading'&&<Feedback kind={names.length?'warning':'error'} title="模型目录暂未读全" details={<pre>{error}</pre>} actions={typeof onRetry==='function'&&<Button type="button" variant="outline" onClick={onRetry}>重新读取模型目录</Button>}>{names.length?'保留上次读取的目录，可以重读原连接。':'需要重新读取已保存连接的模型目录。'}</Feedback>}{names.length?<Combobox label={status==='stale'?'上次已保存连接的模型目录':'可用模型目录'} placeholder={'浏览可用模型 · '+names.length+' 项'} value={selected} options={names} onChange={setSelected}/>:status!=='loading'&&!error&&<p className="text-sm text-muted-foreground">{status==='ready'?'该服务没有返回可用模型。':status==='stale'?'连接已修改，保存后重新同步目录。':'尚未同步模型目录。'}可在「模型用途」手动填写模型 ID。</p>}</div>;
}
