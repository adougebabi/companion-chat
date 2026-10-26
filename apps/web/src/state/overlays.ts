// One application-owned surface. No API calls or durable command identity live here.
export type OverlayRequest={id:number;kind:'sheet'|'dialog'|'confirm';title:string;text?:string;okText?:string;destructive?:boolean;node?:HTMLElement;wide?:boolean;owner?:string;trigger?:HTMLElement|null;onClose?:(accepted:boolean)=>void};
let sequence=0,requests:readonly OverlayRequest[]=Object.freeze([]);
const listeners=new Set<()=>void>();
export const overlaySnapshot=()=>requests;
export const subscribeOverlays=(listener:()=>void)=>{listeners.add(listener);return()=>{listeners.delete(listener);};};
export function openOverlay(value:Omit<OverlayRequest,'id'>){if(!listeners.size)throw Error('弹层宿主尚未就绪，请重新打开页面');const request={...value,id:++sequence};requests=Object.freeze([...requests,request]);for(const listener of listeners)listener();return request.id;}
export function closeOverlay(id:number,accepted=false){const found=requests.find(request=>request.id===id);if(!found)return;requests=Object.freeze(requests.filter(request=>request.id!==id));for(const listener of listeners)listener();found.onClose?.(accepted);}
export function closeAllOverlays(){for(const request of [...requests].reverse())closeOverlay(request.id);}
