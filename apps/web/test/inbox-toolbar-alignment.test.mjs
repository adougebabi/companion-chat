import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import {JSDOM} from 'jsdom';

const read=path=>readFile(new URL(path,import.meta.url),'utf8');
const [page,css,tabs,select,button,controls]=await Promise.all([
  read('../src/react-app/pages/InboxPage.jsx'),
  read('../src/styles/replica-management.css'),
  read('../src/components/ui/tabs.tsx'),
  read('../src/components/ui/select.tsx'),
  read('../src/components/ui/button.tsx'),
  read('../src/react-app/controls.tsx'),
]);
const toolbar=page.match(/<div className="replica-inbox-toolbar">([\s\S]*?)<\/div>/)[1];

function withRules(check){
  const dom=new JSDOM('<!doctype html><style></style>');
  try{
    const style=dom.window.document.querySelector('style');
    style.textContent=css;
    check([...style.sheet.cssRules]);
  }finally{dom.window.close();}
}

test('inbox toolbar centers desktop controls without wrapping and uses a centered three-column grid on narrow screens',()=>withRules(rules=>{
  const base=rules.find(rule=>rule.selectorText==='.replica-inbox-toolbar').style;
  assert.equal(base.getPropertyValue('display'),'flex');
  assert.equal(base.getPropertyValue('align-items'),'center');
  assert.equal(base.getPropertyValue('flex-wrap'),'nowrap');
  const mobile=rules.find(rule=>rule.conditionText==='(max-width:700px)').cssRules;
  const mobileToolbar=[...mobile].find(rule=>rule.selectorText==='.replica-inbox-toolbar').style;
  assert.equal(mobileToolbar.getPropertyValue('display'),'grid');
  assert.equal(mobileToolbar.getPropertyValue('grid-template-columns'),'auto minmax(0,1fr) 36px');
  assert.equal(mobileToolbar.getPropertyValue('align-items'),'center');
  assert.equal(mobileToolbar.getPropertyValue('gap'),'8px');
  for(const media of rules.filter(rule=>rule.cssRules)){
    for(const rule of media.cssRules){
      if(rule.selectorText!=='.replica-inbox-toolbar')continue;
      for(const property of ['display','grid-template-columns','align-items','flex-wrap','gap']){
        const mobileValues={display:'grid','grid-template-columns':'auto minmax(0,1fr) 36px','align-items':'center','flex-wrap':'nowrap',gap:'8px'};
        const expected=media.conditionText==='(max-width:700px)'?mobileValues[property]:base.getPropertyValue(property);
        assert.ok(!rule.style.getPropertyValue(property)||rule.style.getPropertyValue(property)===expected,`${media.conditionText} preserves ${property}`);
      }
    }
  }
}));

test('inbox tabs, role select and refresh reuse shadcn sizes with matching h-9 height',()=>{
  assert.match(toolbar,/<TabsList variant="line" className="group-data-horizontal\/tabs:h-9">/);
  assert.match(toolbar,/<ChoiceSelect size="default" className="data-\[size=default\]:h-9" aria-label="通知角色范围"/);
  assert.match(toolbar,/<Button variant="ghost" size="icon" aria-label="重新读取通知"/);
  assert.match(tabs,/group-data-horizontal\/tabs:h-8/,'local override uses the existing horizontal height variant');
  assert.match(select,/data-\[size=default\]:h-8/,'local override uses the existing select size variant');
  assert.match(controls,/<SelectTrigger\s+\{\.\.\.props\}[\s\S]*?className=\{cn\("w-full min-w-0", className\)\}/,'ChoiceSelect forwards size and className to its trigger');
  assert.match(button,/icon: "size-9 p-0"/,'refresh has the same 9 spacing units of height');
});

test('inbox tab offsets and mobile minimum heights cannot override the common control height',()=>withRules(rules=>{
  const list=rules.find(rule=>rule.selectorText==='.replica-inbox-toolbar [role=tablist]').style;
  assert.equal(list.getPropertyValue('height'),'','TabsList owns its h-9 height');
  assert.equal(list.getPropertyValue('margin-bottom'),'0px');
  assert.equal(list.getPropertyValue('padding'),'0px');
  const tab=rules.find(rule=>rule.selectorText==='.replica-inbox-toolbar [role=tab]').style;
  assert.equal(tab.getPropertyValue('height'),'100%');
  assert.equal(tab.getPropertyValue('padding'),'0px 2px');
  function checkOverrides(rules){
    for(const rule of rules){
      if(rule.cssRules)checkOverrides([...rule.cssRules]);
      if(!rule.selectorText?.includes('.replica-inbox-toolbar'))continue;
      const min=rule.style.getPropertyValue('min-height');
      if(rule.selectorText!=='.replica-inbox-toolbar')assert.ok(!min||min==='0px',`${rule.selectorText} must not enlarge controls`);
      if(rule.selectorText.includes('[role=tablist]')&&rule.style.getPropertyValue('height'))assert.fail('CSS must not override TabsList height');
    }
  }
  checkOverrides(rules);
}));
