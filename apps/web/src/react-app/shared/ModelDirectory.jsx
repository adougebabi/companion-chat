import {useState} from 'react';
import Combobox from './Combobox';
import {Button} from '@/components/ui/button';
export default function ModelDirectory({models=[],status,error,onRetry}){
 const [selected,setSelected]=useState(''),names=[...new Set(models.map(model=>typeof model==='string'?model:model.id||model.name).filter(Boolean))];
 return <div className="grid min-w-0 gap-2">{status==='loading'?<p role="status">正在同步模型目录…</p>:error?<div role="alert" className="grid gap-2"><p>{error}</p><Button type="button" variant="outline" onClick={onRetry}>重新读取模型目录</Button></div>:names.length?<Combobox label="可用模型目录" placeholder={'浏览可用模型 · '+names.length+' 项'} value={selected} options={names} onChange={setSelected}/>:<p className="text-sm text-muted-foreground">{status==='ready'?'该服务没有返回可用模型。':status==='stale'?'连接已修改，保存后重新同步目录。':'尚未同步模型目录。'}可在「模型用途」手动填写模型 ID。</p>}</div>;
}
