import {useState} from 'react';
import Combobox from './Combobox';
export default function ModelDirectory({models=[]}){
 const [selected,setSelected]=useState(''),names=[...new Set(models.map(model=>typeof model==='string'?model:model.id||model.name).filter(Boolean))];
 return <div className="grid min-w-0 gap-2"><Combobox label="可用模型目录" placeholder={'浏览可用模型 · '+names.length+' 项'} value={selected} options={names} onChange={setSelected}/><p className="text-sm text-muted-foreground">目录仅供浏览；在「模型用途」选择并保存对应模型。{!names.length&&' 尚未同步目录时，也可以手动填写模型 ID。'}</p></div>;
}
