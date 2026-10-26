import {get} from '../store.js';
import {inventoryClient} from '../services.js';
import {drawer} from '../ui.js';
import {mountOwnerMediaRequest} from './media-generation.js';

export function openOwnerMediaDrawer({fid,itemId=null,prompt='',place=null,title='图片请求'}){
 const owner=get('session')?.actorId;let panel=null,active=true;
 const d=drawer({title,body:'<div data-photo-panel role="status">正在读取图片请求来源…</div>',footer:'<button data-domain-ui="button" class="btn" data-close>关闭</button>',onClose:()=>{active=false;panel?.dispose();}});
 d.el.querySelector('[data-close]').onclick=()=>d.close();const alive=()=>active&&d.el.isConnected&&get('session')?.actorId===owner;
 void(async()=>{try{const item=itemId?await inventoryClient.read(fid,itemId):null;if(!alive())return;panel=mountOwnerMediaRequest(d.el.querySelector('[data-photo-panel]'),{fid,fixedItem:item,initialPrompt:prompt,initialPlace:place});}catch(error){if(alive())d.el.querySelector('[data-photo-panel]').textContent=error.message;}})();return d;
}
