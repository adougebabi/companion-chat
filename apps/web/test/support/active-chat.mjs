import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact,act} from '../helpers/react-dom.mjs';
export async function activeChatFixture(){
 const environment=installDOM(),calls=[];
 const vite=await createServer({root:fileURLToPath(new URL('../../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 const {default:Chat}=await vite.ssrLoadModule('/src/react-app/pages/ChatPage.tsx'),{createConversationController}=await vite.ssrLoadModule('/src/state/conversation.ts');
 const store=createConversationController();store.fluctlightId='role';store.conversation={id:'conversation'};store.messages=[];
 for(const [key,fn] of Object.entries({send:async(text,_a,_b,accepted)=>{calls.push(['send',text]);accepted();},cancel:()=>calls.push(['cancel']),retryMessage:async id=>calls.push(['retry',id]),refreshActiveTurn:async force=>calls.push(['status',force]),invalidateRequest:()=>calls.push(['invalidate'])}))Object.defineProperty(store,key,{configurable:true,value:fn});
 const state=proxy({session:{actorId:'owner'},selectedId:'role',characters:[{id:'role',name:'周屿',initial:'屿',color:'#aabbcc',schedule:[],wearing:{},garments:[],_directory:{},timezone:'Asia/Shanghai'}],errors:{},loading:{}});
 const workspace={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),conversation:()=>store,current:{get value(){return state.characters[0];}},currentScope:()=> 'owner-role',clock:()=> '12:00',services:{mediaClient:{url:id=>'/api/media/'+id}},openConversation:async(id,live)=>calls.push(['open',id,live()]),navigatePath:path=>calls.push(['navigate',path]),detail:async()=>{},loadWardrobe:async()=>{}};
 const view=await mountReact(vite,Chat,{workspace,props:{preview:false,onGo(){}}});
 const button=text=>[...view.host.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
 return {store,calls,view,button,async update(fn){await act(async()=>fn());await view.flush();},async dispose(){await view.dispose();await vite.close();environment.restore();}};
}
