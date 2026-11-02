import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {SidebarProvider,Sidebar,SidebarTrigger,SidebarRail}=await vite.ssrLoadModule('/src/components/ui/sidebar.tsx');
test.after(async()=>{await vite.close();env.restore();});
test('r34c every sidebar toggle exposes Chinese visible, sr-only, aria and title text',async()=>{
 const Component=()=>React.createElement(SidebarProvider,null,React.createElement(Sidebar,null,React.createElement(SidebarRail)),React.createElement(SidebarTrigger));
 const v=await mountReact(vite,Component);
 try{
  const trigger=v.host.querySelector('[data-slot=sidebar-trigger]'),rail=v.host.querySelector('[data-slot=sidebar-rail]');
  assert.equal(trigger.querySelector('.sr-only').textContent,'切换侧栏');
  assert.equal(rail.getAttribute('aria-label'),'切换侧栏');assert.equal(rail.title,'切换侧栏');
  const sidebar=v.host.querySelector('[data-state]');assert.equal(sidebar.dataset.state,'expanded');
  await v.click(trigger);assert.equal(sidebar.dataset.state,'collapsed');
  await v.click(rail);assert.equal(sidebar.dataset.state,'expanded');
  for(const node of v.host.querySelectorAll('.sr-only,[aria-label],[title]'))assert.doesNotMatch(node.textContent+' '+node.getAttribute('aria-label')+' '+node.getAttribute('title'),/Toggle Sidebar/);
 }finally{await v.dispose();}
});
