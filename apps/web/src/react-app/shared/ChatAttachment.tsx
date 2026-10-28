import {useState} from 'react';
import {ImageIcon} from 'lucide-react';
import {Attachment,AttachmentMedia,AttachmentTrigger} from '@/components/ui/attachment';
import {Spinner} from '@/components/ui/spinner';

export default function ChatAttachment({src,alt,onOpen}:{src:string;alt:string;onOpen:(source:HTMLButtonElement)=>void}) {
  const [state,setState]=useState<'loading'|'done'|'error'>('loading');
  const [attempt,setAttempt]=useState(0);
  return <Attachment orientation="vertical" className="companion-message-image" state={state==='loading'?'processing':state}>
    <AttachmentMedia variant="image" className="companion-message-image-preview">
      <img key={src+attempt} src={src} alt={alt} onLoad={()=>setState('done')} onError={()=>setState('error')}/>
      {state==='loading'&&<span className="companion-image-status" role="status"><Spinner/>正在读取图片…</span>}
      {state==='error'&&<span className="companion-image-status" role="alert">图片暂时无法读取</span>}
    </AttachmentMedia>
    <span className={state==='error'?'companion-image-retry':undefined}>
      {state==='error'?'重新读取图片':<><ImageIcon size={12}/>查看这个片刻</>}
    </span>
    <AttachmentTrigger aria-label={state==='error'?'重新读取图片':'查看对话附件'} onClick={event=>{
      if(state==='error'){setState('loading');setAttempt(value=>value+1);}
      else onOpen(event.currentTarget);
    }}/>
  </Attachment>;
}
