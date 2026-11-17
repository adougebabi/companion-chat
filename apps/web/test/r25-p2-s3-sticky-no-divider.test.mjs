import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import {JSDOM} from 'jsdom';

test('management sticky headers have a bottom divider without changing layout height on desktop or narrow screens', async()=>{
  const css=await readFile(new URL('../src/styles/replica-management.css',import.meta.url),'utf8');
  const dom=new JSDOM('<!doctype html><style></style>');
  try{
    const style=dom.window.document.querySelector('style');
    style.textContent=css;
    const rules=[...style.sheet.cssRules];
    const selectors=['.replica-management-section-header','.replica-page--managed .replica-data-main>.replica-settings-head'];
    const sticky=rules.find(rule=>rule.style?.getPropertyValue('position')==='sticky'&&selectors.every(selector=>rule.selectorText.split(',').map(value=>value.trim()).includes(selector)));
    assert.ok(sticky,'both management header variants share a viewport-independent sticky rule');
    assert.deepEqual(sticky.selectorText.split(',').map(value=>value.trim()),selectors,'divider stays scoped to management headers');
    assert.equal(sticky.style.getPropertyValue('box-shadow'),'inset 0 -1px 0 var(--border)','one-pixel bottom divider uses the existing border token without occupying layout space');
    for(const property of ['height','min-height','padding','padding-bottom','border','border-bottom']){
      assert.equal(sticky.style.getPropertyValue(property),'',`${property} must not change sticky header geometry`);
    }
    // Later rules, including narrow-screen media rules, must retain the divider.
    const checkOverrides=rules=>{
      for(const rule of rules){
        if(rule.cssRules)checkOverrides([...rule.cssRules]);
        if(rule.selectorText?.split(',').some(selector=>selectors.includes(selector.trim()))&&rule.style.getPropertyValue('box-shadow')){
          assert.equal(rule.style.getPropertyValue('box-shadow'),'inset 0 -1px 0 var(--border)');
        }
      }
    };
    checkOverrides(rules);
  }finally{dom.window.close();}
});
