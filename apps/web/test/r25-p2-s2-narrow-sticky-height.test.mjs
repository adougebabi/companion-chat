import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import {JSDOM} from 'jsdom';

const css=await readFile(new URL('../src/styles/replica-management.css',import.meta.url),'utf8');
const compact='.replica-page--managed:has(.replica-page-heading.is-scrolled)';
function fixture(width,variant='diagnostics'){
  // Match the shared management shell and the three rendered header variants.
  // This layout regression needs no API responses or service calls.
  const data=variant==='data';
  const tabs=variant==='models'?['模型服务','用途分配']:['任务中心','模型调用记录','服务诊断'];
  const dom=new JSDOM(`<!doctype html><style></style>
    <div class="react-workspace replica-workspace"><main class="replica-page replica-page--managed">
      <div class="replica-management">
        <header class="replica-page-heading"><div><h1>管理中心</h1></div></header>
        <div class="replica-management-layout">
          <nav class="replica-management-nav"><button>模型服务</button><button>诊断与任务</button></nav>
          <div class="${data?'replica-data-main':'replica-management-main'}">
            ${data?'':'<div class="replica-management-section-header">'}
              <div class="replica-settings-head">
                <div><h2>${data?'数据管理':variant==='models'?'模型服务':'诊断与任务'}</h2><p>查看和管理工作区。</p></div>
                <small>服务与用途</small><span class="replica-diagnostic-environment">实际运行记录</span><svg class="replica-settings-section-icon"></svg>
              </div>
              ${data?'':'<div data-slot="tabs-list" role="tablist">'+tabs.map(label=>'<button role="tab" data-slot="tabs-trigger">'+label+'</button>').join('')+'</div></div>'}
          </div>
        </div>
      </div>
    </main></div>`);
  const style=dom.window.document.querySelector('style');
  style.textContent=css;
  const rules=[...style.sheet.cssRules];
  // jsdom does not evaluate viewport media queries. Select matching CSSOM rules
  // explicitly, then let its selector engine compute the compact-state styles.
  const atWidth=rules=>rules.flatMap(rule=>{
    if(!rule.conditionText)return [rule.cssText];
    const condition=rule.conditionText||'';
    const min=Number(condition.match(/min-width:\s*(\d+)px/)?.[1]||0);
    const max=Number(condition.match(/max-width:\s*(\d+)px/)?.[1]||Infinity);
    return width>=min&&width<=max?atWidth([...rule.cssRules]):[];
  });
  style.textContent=atWidth(rules).join('\n');
  return {dom,rules,document:dom.window.document,computed:selector=>dom.window.getComputedStyle(dom.window.document.querySelector(selector))};
}

test('compact height overrides use the existing 700px breakpoint and leave desktop sticky geometry/divider intact',()=>{
  const f=fixture(1280);
  try{
    const narrow=f.rules.find(rule=>rule.conditionText==='(max-width:700px)'&&[...rule.cssRules].some(child=>child.selectorText===compact+' .replica-management-section-header'));
    assert.ok(narrow,'compact styles belong only to the existing narrow-screen breakpoint');
    const value=(selector,property)=>[...narrow.cssRules].find(rule=>rule.selectorText===selector)?.style.getPropertyValue(property);
    assert.equal(value('.react-workspace.replica-workspace '+compact,'padding-top'),'4px');
    assert.equal(value(compact+' .replica-management-layout','gap'),'4px');
    assert.equal(value(compact+' .replica-management-nav','padding-block'),'0px');
    assert.equal(value(compact+' .replica-management-section-header','gap'),'4px');
    assert.equal(value(compact+' .replica-management-section-header [role=tab][data-slot=tabs-trigger]','min-height'),'36px');
    assert.equal(value(compact+' .replica-management-section-header [role=tab][data-slot=tabs-trigger]','padding'),'4px 2px');
    const header=f.computed('.replica-management-section-header');
    assert.equal(header.position,'sticky');
    assert.equal(header.top,'0px');
    assert.equal(header.gap,'20px');
    assert.equal(header.boxShadow,'inset 0 -1px 0 var(--border)');
    f.document.querySelector('.replica-page-heading').classList.add('is-scrolled');
    assert.equal(f.computed('.replica-management-section-header').gap,'20px');
    assert.notEqual(f.computed('.replica-settings-head p').display,'none');
    assert.equal(f.computed('.replica-management-layout').gap,'28px');
  }finally{f.dom.window.close();}
});

test('narrow compact state hides only header explanations and decorations; titles, tabs and navigation remain usable',()=>{
  for(const variant of ['diagnostics','models','data']){
    const f=fixture(390,variant);
    try{
      assert.notEqual(f.computed('.replica-settings-head p').display,'none','explanations are present before scrolling');
      const heading=f.document.querySelector('.replica-page-heading');
      heading.classList.add('is-scrolled');
      for(const selector of ['p','small','.replica-diagnostic-environment','.replica-settings-section-icon']){
        assert.equal(f.computed('.replica-settings-head '+selector).display,'none',variant+' hides '+selector);
      }
      assert.equal(f.computed('.replica-settings-head').flexDirection,'row');
      assert.equal(f.computed('.replica-settings-head h2').lineHeight,'24px');
      assert.equal(f.computed('.replica-page-heading h1').lineHeight,'24px');
      assert.notEqual(f.computed('.replica-settings-head h2').display,'none');
      assert.notEqual(f.computed('.replica-management-nav').display,'none');
      assert.equal(f.document.querySelectorAll('.replica-management-nav button').length,2);
      if(variant!=='data'){
        assert.equal(f.computed('.replica-management-section-header').gap,'4px');
        assert.equal(f.computed('[role=tab]').minHeight,'36px');
        assert.notEqual(f.computed('[role=tablist]').display,'none');
        assert.equal(f.document.querySelectorAll('[role=tab]').length,variant==='models'?2:3);
      }
      const sticky=f.computed(variant==='data'?'.replica-data-main>.replica-settings-head':'.replica-management-section-header');
      assert.equal(sticky.position,'sticky');
      assert.equal(sticky.boxShadow,'inset 0 -1px 0 var(--border)');
      heading.classList.remove('is-scrolled');
      assert.notEqual(f.computed('.replica-settings-head p').display,'none','scrolling back restores explanations');
    }finally{f.dom.window.close();}
  }
});

test('compact rules stop at the narrow-screen boundary',()=>{
  for(const width of [700,701]){
    const f=fixture(width);
    try{
      f.document.querySelector('.replica-page-heading').classList.add('is-scrolled');
      assert.equal(f.computed('.replica-settings-head p').display,width===700?'none':'block');
      assert.equal(f.computed('.replica-management-section-header').gap,width===700?'4px':'20px');
    }finally{f.dom.window.close();}
  }
});
