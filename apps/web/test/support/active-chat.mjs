import {readFileSync} from 'node:fs';
import * as Vue from 'vue';
import {renderToString} from 'vue/server-renderer';
import {renderChatPage} from '../../src/approved-ui/renderers/ChatPage.mjs';
import {chatWaitingPresentation,chatDiagnosticTurn,chatFailureText} from '../../src/v3/chat-feedback.mjs';
import {userInstant} from '../../src/v3/presentation-time.mjs';
import {memoryStorage,tick} from './sfc-harness.mjs';
export function activeChatFixture(){
 const mounted=[],unmounted=[],calls=[],storage=memoryStorage();let project;
 const store=Vue.reactive({messages:[],fluctlightId:'role',sending:false,hasPendingTurn:false,hasSettlingTurn:false,canCancel:false,error:'',attemptStartedAt:Date.now()-120000,
  $subscribe(fn){project=fn;return()=>calls.push(['unsubscribe']);},send:async(text,_a,_b,accepted)=>{calls.push(['send',text]);accepted();},cancel(){calls.push(['cancel']);},retryMessage:async id=>calls.push(['retry',id]),refreshActiveTurn:async force=>calls.push(['status',force]),invalidateRequest(){calls.push(['invalidate']);}});
 const character=Vue.ref({id:'role',name:'周屿',initial:'屿',color:'#aabbcc',messages:[],schedule:[],timezone:'Asia/Shanghai'});
 const workspace={conversation:()=>store,current:character,currentScope:()=> 'owner-role',clock:()=> '12:00',state:{errors:{}},services:{mediaClient:{url:id=>'/api/media/'+id}},openConversation:async(id,live)=>calls.push(['open',id,live()]),navigatePath:path=>calls.push(['navigate',path])};
 const imports={defineComponent:x=>x,ref:Vue.ref,computed:Vue.computed,watch:Vue.watch,nextTick:Vue.nextTick,onMounted:fn=>mounted.push(fn),onUnmounted:fn=>unmounted.push(fn),useWorkspace:()=>workspace,renderChatPage:x=>x,CurrentOutfit:()=>null,joined:(...v)=>v.join(''),chatWaitingPresentation,chatDiagnosticTurn,chatFailureText,userInstant,sessionStorage:storage,setInterval:()=>1,clearInterval(){}};
 const source=readFileSync(new URL('../../src/approved-ui/pages/ChatPage.mjs',import.meta.url),'utf8').replace(/^import\s[\s\S]*?from\s*['"][^'"]+['"];?\s*$/gm,'').replace('export default','return');
 const component=Function(...Object.keys(imports),source)(...Object.values(imports)),bindings=component.setup({preview:false},{emit(){}});
 return {bindings,store,character,calls,storage,project:()=>project(),async mount(){for(const fn of mounted)await fn();},dispose(){for(const fn of unmounted)fn();},html(){return renderToString(Vue.createSSRApp({render:renderChatPage(bindings)}));}};
}
