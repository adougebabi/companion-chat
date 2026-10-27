import {Disclosure} from '../controls';
import {Button} from '@/components/ui/button';
import RecordView from './RecordView';
export default function JsonDetails({value,title='技术详情'}:{value:any;title?:string}){const empty=value==null||typeof value==='object'&&!Object.keys(value).length;return <Disclosure title={title}><RecordView value={value} emptyText="尚未记录相关详情" technical/>{!empty&&<Button type="button" variant="outline" onClick={()=>{const url=URL.createObjectURL(new Blob([JSON.stringify(value,null,2)],{type:'application/json'})),link=document.createElement('a');link.href=url;link.download='fluctlight-record.json';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}}>下载原始记录</Button>}</Disclosure>;}
