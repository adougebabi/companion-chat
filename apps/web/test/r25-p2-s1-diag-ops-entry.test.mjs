import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact} from './helpers/react-dom.mjs';
import {approvedRoute} from '../src/approved-ui/routing.mjs';

const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const styles=await Promise.all(['replica-management','replica-diagnostics'].map(name=>readFile(new URL(`../src/styles/${name}.css`,import.meta.url),'utf8')));
test.after(async()=>{await vite.close();env.restore();});

test('运行记录入口在桌面、窄屏和滚动紧凑态可见，点击进入已注册的事件路由',async()=>{
  const {default:Diagnostics}=await vite.ssrLoadModule('/src/react-app/shared/DiagnosticsWorkspace.jsx');
  const state=proxy({session:{actorId:'diagnostic-entry-owner'},characters:[]}),reads=[];
  const workspace={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{diagnosticsClient:{page:async(kind,options)=>{
    reads.push([kind,options]);
    return {items:[],next_cursor:''};
  }}}};
  const v=await mountReact(vite,Diagnostics,{workspace,props:{view:{openTaskDiagnostics(){}}}});
  const content=v.host.firstElementChild;
  const style=document.createElement('style');
  document.head.append(style);
  try{
    assert.ok(reads.some(([kind])=>kind==='workflow-intents'));
    v.host.className='react-workspace replica-workspace';
    const shell=document.createElement('main');
    shell.className='replica-page replica-page--managed';
    shell.innerHTML='<header class="replica-page-heading"></header><div class="replica-management"><div class="replica-management-layout"><div class="replica-management-main"></div></div></div>';
    v.host.append(shell);
    shell.querySelector('.replica-management-main').append(content);
    const heading=shell.querySelector('.replica-page-heading');
    const entry=content.querySelector('.replica-settings-head .replica-diagnostic-environment');
    assert.equal(entry.tagName,'A');
    assert.equal(entry.getAttribute('data-slot'),'button','复用 shadcn Button asChild');
    assert.equal(entry.textContent,'查看运行记录');
    assert.equal(entry.getAttribute('href'),'#/diag/events');
    assert.equal(entry.closest('button'),null);
    style.textContent=styles.join('\n');
    const rules=[...style.sheet.cssRules];
    // jsdom does not apply viewport media queries; select the matching CSSOM
    // rules, as in the existing narrow sticky header regression test.
    const atWidth=(rules,width)=>rules.flatMap(rule=>{
      if(!rule.conditionText)return [rule.cssText];
      const min=Number(rule.conditionText.match(/min-width:\s*(\d+)px/)?.[1]||0);
      const max=Number(rule.conditionText.match(/max-width:\s*(\d+)px/)?.[1]||Infinity);
      return width>=min&&width<=max?atWidth([...rule.cssRules],width):[];
    });
    for(const width of [320,390,700,701,1280]){
      style.textContent=atWidth(rules,width).join('\n');
      for(const scrolled of [false,true]){
        heading.classList.toggle('is-scrolled',scrolled);
        for(let node=entry;node&&node!==v.host;node=node.parentElement){
          const computed=window.getComputedStyle(node);
          assert.notEqual(computed.display,'none',`${width}px scrolled=${scrolled}: ${node.className}`);
          assert.notEqual(computed.visibility,'hidden');
        }
        assert.equal(window.getComputedStyle(entry).minHeight,'32px');
      }
    }
    await v.click(entry);
    assert.equal(window.location.hash,'#/diag/events');
    const route=approvedRoute(window.location.hash);
    assert.equal(route.notFound,undefined);
    assert.equal(route.feature.params.section,'events');
  }finally{
    // Return the rendered subtree to React before unmounting.
    v.host.prepend(content);
    v.host.querySelector('main')?.remove();
    style.remove();
    await v.dispose();
  }
});
