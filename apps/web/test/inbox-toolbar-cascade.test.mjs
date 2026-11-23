import test from 'node:test';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {fileURLToPath} from 'node:url';
import {build,createServer} from 'vite';
import {installDOM,mountReact} from './helpers/react-dom.mjs';

const require=createRequire(import.meta.url);
const postcss=createRequire(require.resolve('vite'))('postcss');
const Specificity=createRequire(require.resolve('jsdom'))('@bramus/specificity').default;
const root=fileURLToPath(new URL('../',import.meta.url));
const geometry=['height','min-height','max-height','box-sizing','padding-top','padding-bottom','border-top-width','border-bottom-width'];
function mediaMatches(query,width,height){
  if(/prefers-|forced-colors/.test(query))return false;
  const matches=[...query.matchAll(/(min|max)-(width|height):\s*([\d.]+)(px|rem)/g)].every(([,bound,axis,n,unit])=>{
    const actual=axis==='width'?width:height,target=Number(n)*(unit==='rem'?16:1);
    return bound==='min'?actual>=target:actual<=target;
  });
  return query.startsWith('not all')?!matches:matches;
}
const greater=(a,b)=>{for(let i=0;i<a.length;i++){if(a[i]!==b[i])return a[i]>b[i];}return false;};
// jsdom has no layout / complete layer cascade. Resolve declarations explicitly:
// importance, layer, Selectors 4 specificity (:where=0, :is=max), source order.
// Normal unlayered rules outrank ALL named layers; !important reverses layers.
function cascade(css,element,width,height){
  const layers=['theme','base','components','utilities'],winners={};let order=0;
  function visit(container,layer=null){
    for(const node of container.nodes||[]){
      if(node.type==='atrule'){
        if(node.name==='layer'&&node.nodes)visit(node,node.params);
        else if(node.name==='media'&&mediaMatches(node.params,width,height))visit(node,layer);
        else if(node.name==='supports')visit(node,layer);
        continue;
      }
      if(node.type!=='rule')continue;
      for(const selector of postcss.list.comma(node.selector)){
        if(selector.includes('::'))continue; // Pseudo-elements own separate boxes.
        let matches=false;try{matches=element.matches(selector);}catch{continue;}
        if(!matches)continue;
        const {a,b,c}=Specificity.calculate(selector)[0].value;
        for(const d of node.nodes.filter(n=>n.type==='decl')){
          const style=element.ownerDocument.createElement('div').style;
          style.setProperty(d.prop,d.value);
          for(const property of geometry){
            const value=d.prop===property?d.value:style.getPropertyValue(property);
            if(!value)continue;
            const index=layer===null?layers.length:layers.indexOf(layer);
            const rank=[Number(Boolean(d.important)),d.important?-index:index,a,b,c,order++];
            if(!winners[property]||greater(rank,winners[property].rank))winners[property]={value,selector,layer,rank};
          }
        }
      }
    }
  }
  visit(css);return winners;
}
function pixels(value,spacing){
  if(value===undefined||value==='none'||value==='0')return 0;
  const calc=value.match(/^calc\(var\(--spacing\) \* ([\d.]+)\)$/);
  if(calc)return pixels(spacing)*Number(calc[1]);
  const length=value.match(/^([\d.]+)(px|rem)$/);
  assert.ok(length,`unsupported geometry value: ${value}`);
  return Number(length[1])*(length[2]==='rem'?16:1);
}
function borderBox(styles,spacing){
  assert.equal(styles['box-sizing'].value,'border-box');
  const height=Math.max(pixels(styles.height?.value,spacing),pixels(styles['min-height']?.value,spacing));
  const inset=['padding-top','padding-bottom','border-top-width','border-bottom-width'].reduce((sum,key)=>sum+pixels(styles[key]?.value,spacing),0);
  assert.ok(inset<=height,'padding and borders fit inside the border box');
  assert.ok(!styles['max-height']||styles['max-height'].value==='none'||pixels(styles['max-height'].value,spacing)>=height);
  return height;
}

test('inbox actual toolbar resolves to three 36px controls at 1280×800 and 390×844 using production CSS cascade',async()=>{
  // Real production Vite/Tailwind pipeline, all actual CSS imports. No stale
  // dist dependency, .env reads, disk output or backend/model connections.
  const previousMode=process.env.NODE_ENV;
  let result;
  try{result=await build({root,envFile:false,logLevel:'silent',build:{write:false}});}
  finally{if(previousMode===undefined)delete process.env.NODE_ENV;else process.env.NODE_ENV=previousMode;}
  const css=postcss.parse(result.output.filter(x=>x.fileName.endsWith('.css')).map(x=>x.source).join('\n'));
  let spacing;css.walkDecls('--spacing',d=>{spacing=d.value;});assert.equal(pixels(spacing),4);
  const env=installDOM(),vite=await createServer({root,envFile:false,logLevel:'silent',server:{middlewareMode:true},appType:'custom'});
  let view;
  try{
    const {default:Inbox}=await vite.ssrLoadModule('/src/react-app/pages/InboxPage.jsx');
    const w={state:{session:{actorId:'toolbar-test'},characters:[],notificationCount:{count:0}},subscribe:()=>()=>{},getSnapshot:()=>w.state,current:{value:{}},client:{},services:{operationsClient:{notifications:async()=>({items:[],next_cursor:''}),pending:async()=>({groups:[]})}}};
    view=await mountReact(vite,Inbox,{workspace:w});view.host.className='react-workspace replica-workspace';
    const toolbar=view.host.querySelector('.replica-inbox-toolbar'),trigger=toolbar.querySelector('[data-slot="select-trigger"]'),list=toolbar.querySelector('[role="tablist"]'),refresh=toolbar.querySelector('button[aria-label="重新读取通知"]');
    assert.equal(trigger.parentElement.tagName,'SPAN','actual ChoiceSelect wrapper is retained');
    assert.equal(trigger.getAttribute('role'),'combobox');assert.equal(trigger.dataset.size,'default');assert.equal(trigger.textContent.trim(),'所有角色');
    for(const [width,height] of [[1280,800],[390,844]]){
      const styles=cascade(css,trigger,width,height);
      assert.equal(styles.height.layer,'utilities','h-9 reaches the real trigger');assert.equal(pixels(styles.height.value,spacing),36);
      assert.equal(styles['min-height'].layer,null,'local fix is unlayered');
      assert.equal(styles['min-height'].selector,'.replica-inbox-toolbar [data-slot=select-trigger]');assert.equal(pixels(styles['min-height'].value,spacing),0);
      assert.deepEqual([trigger,list,refresh].map(el=>borderBox(cascade(css,el,width,height),spacing)),[36,36,36],`${width}×${height}`);
      // Remove only the fix: identical DOM and compiled CSS reproduce 39px,
      // proving h-9 alone cannot beat the shared replica min-height.
      const before=css.clone();before.walkRules(r=>{if(r.selector===styles['min-height'].selector)r.remove();});
      const old=cascade(before,trigger,width,height);
      assert.equal(old['min-height'].layer,null);assert.equal(old['min-height'].value,'39px');assert.match(old['min-height'].selector,/^\.replica-workspace/);assert.equal(borderBox(old,spacing),39);
    }
    toolbar.classList.remove('replica-inbox-toolbar');assert.equal(borderBox(cascade(css,trigger,1280,800),spacing),39,'other consumers retain the shared minimum');
  }finally{if(view)await view.dispose();await vite.close();env.restore();}
});
