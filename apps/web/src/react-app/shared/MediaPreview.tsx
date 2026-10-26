import {useEffect,useState} from 'react';
import {Button} from '@/components/ui/button';
import {Alert,AlertDescription} from '@/components/ui/alert';
export default function MediaPreview({src,alt='图片素材',className='max-h-96 w-full object-contain'}:{src:string;alt?:string;className?:string}){const [failed,setFailed]=useState(false),[attempt,retry]=useState(0);useEffect(()=>setFailed(false),[src]);return failed?<Alert variant="destructive"><AlertDescription><p>图片读取失败，请核对素材状态或重新读取。</p><Button type="button" variant="outline" onClick={()=>{setFailed(false);retry(value=>value+1);}}>重新读取图片</Button></AlertDescription></Alert>:<img key={attempt} src={src} alt={alt} className={className} onError={()=>setFailed(true)}/>;}
