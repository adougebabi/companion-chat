import {get,subscribe} from '../proto/store.js';

let latestError='',started=false;
export const browserNotificationState=()=>({supported:typeof Notification!=='undefined',permission:typeof Notification==='undefined'?'unsupported':Notification.permission,error:latestError});
function instant(value){const at=Date.parse(value);if(!Number.isFinite(at))throw new Error('通知时间不可读取');const fraction=String(value).match(/\.(\d+)(?:Z|[+-]\d\d:\d\d)$/)?.[1]||'';return BigInt(Math.floor(at/1000))*1000000000n+BigInt(fraction.padEnd(9,'0').slice(0,9));}

export function installBrowserNotifications(client){
 if(started)return;started=true;let owner='',epoch=0,busy=false,watermark='';
 function sync(){const next=get('session')?.actorId||'';if(next!==owner){owner=next;epoch++;watermark='';latestError='';}void poll();}
 async function poll(){
  if(busy||!owner||document.hidden||typeof Notification==='undefined'||Notification.permission!=='granted')return;
  busy=true;const actor=owner,token=epoch,live=()=>owner===actor&&epoch===token&&get('session')?.actorId===actor;
  try{
   let page=await client.listNotifications({unread:true,limit:100});if(!live())return;
   if(!watermark){watermark=page.as_of;latestError='';return;}
   const nextWatermark=page.as_of,cutoff=instant(watermark),pending=[];
   for(;;){if(!live())return;let older=false;for(const notice of page.items){if(instant(notice.created_at)<=cutoff){older=true;continue;}if(!notice.quiet&&!notice.read_at)pending.push(notice);}if(older||!page.next_cursor)break;page=await client.listNotifications({unread:true,cursor:page.next_cursor,limit:100});}
   for(const notice of pending.reverse()){
    const show=()=>{if(!live())return;const key=`fluctlight-notice-shown:${actor}:${notice.id}`;if(localStorage.getItem(key))return;const popup=new Notification(notice.title,{body:notice.body,tag:actor+':'+notice.id});localStorage.setItem(key,'1');popup.onclick=()=>{if(live()&&notice.link?.startsWith('#/')){window.focus();location.hash=notice.link;}popup.close();};};
    if(navigator.locks)await navigator.locks.request(`fluctlight-notice:${actor}:${notice.id}`,show);else show();
   }
   if(live()){watermark=nextWatermark;latestError='';}
  }catch(error){if(live())latestError=error.message;}finally{busy=false;}
 }
 subscribe(sync);document.addEventListener('visibilitychange',()=>void poll());setInterval(()=>void poll(),30000);sync();
}
