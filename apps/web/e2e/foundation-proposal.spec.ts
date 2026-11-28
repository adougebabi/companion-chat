import {test,expect} from '@playwright/test';
import {expectNoLayoutDefects,expectTabsReachable} from './support/layout';
import {readManifest,verifyBaselines} from './support/baselines.mjs';
const origin='http://127.0.0.1:5197',fid='fluctlight-fixture-one';
test.beforeEach(async({page})=>{
 await page.clock.setFixedTime(new Date('2026-11-23T10:30:00+08:00'));
 await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
});
test('foundation proposal uses shared profile fields and retains advanced data without applying identity',{tag:['@foundation','@visual']},async({page})=>{
 const writes:{url:string;body:any}[]=[];
 let release:()=>void=()=>{};const pending=new Promise<void>(resolve=>{release=resolve;});
 await page.route(/\/foundation-revisions$/,async route=>{if(route.request().method()==='POST')await pending;await route.continue();});
 page.on('request',request=>{if(request.method()==='POST'&&request.url().includes('/foundation-revisions'))writes.push({url:request.url(),body:request.postDataJSON()});});
 await page.goto(origin+'/#/c/'+fid+'/governance?tab=foundation');
 await page.getByRole('button',{name:'创建修订提议',exact:true}).click();
 const form=page.getByRole('dialog',{name:'创建身份核心修订提议',exact:true});
 await expect(form.getByRole('textbox',{name:'名字',exact:true})).toBeVisible();
 await expect(form.getByRole('button',{name:'操作打开时的原始记录（诊断）',exact:true})).toBeVisible();
 await expectTabsReachable(form.getByRole('tablist'));
 await expectNoLayoutDefects(page,form,{minTarget:page.viewportSize()!.width<640?40:0});
 const name='foundation-proposal-form.png',file=test.info().project.name+'/'+name;
 if(!process.env.UI_BASELINE_APPROVAL_REASON&&(verifyBaselines().status!=='ok'||!readManifest()?.files?.[file]))throw Error('UI_BASELINE_UNVERIFIED: '+file);
 await expect(form).toHaveScreenshot(name);
 await form.getByRole('textbox',{name:'名字',exact:true}).fill('林青的提议称呼');
 await form.getByRole('textbox',{name:'提议理由',exact:true}).fill('想先一起核对这个称呼');
 for(const label of ['性格与说话方式','生活','更多设定','基本资料']){
  const tab=form.getByRole('tab',{name:label,exact:true});
  if(await tab.count()){await tab.scrollIntoViewIfNeeded();await tab.click();await expect(tab).toHaveAttribute('aria-selected','true');await expectNoLayoutDefects(page,form,{minTarget:page.viewportSize()!.width<640?40:0});}
 }
 await form.getByRole('button',{name:'使用高级 JSON',exact:true}).click();
 const json=form.getByRole('textbox',{name:'字段变更（JSON）',exact:true});
 expect(JSON.parse(await json.inputValue())).toEqual({identity:{name:'林青的提议称呼'}});
 const changes={identity:{name:'林青的提议称呼'},personality:{openness:0},provenance:{future:{sample:[null,false,0]}}};
 await json.fill(JSON.stringify(changes));
 await form.getByRole('button',{name:'使用常用字段',exact:true}).click();
 await expect(form.getByRole('textbox',{name:'名字',exact:true})).toHaveValue('林青的提议称呼');
 const save=form.getByRole('button',{name:'保存提议（不立即应用）',exact:true});await expect(save).toBeInViewport({ratio:1});await save.click();
 await expect(save).toBeDisabled();await page.keyboard.press('Escape');await expect(form).toBeVisible();release();
 await expect(form).toBeHidden();expect(writes).toHaveLength(1);
 expect(writes[0].url).toMatch(/\/foundation-revisions$/);expect(writes[0].body.changes).toEqual(changes);
 expect(writes[0].body.reason).toBe('想先一起核对这个称呼');expect(writes[0].body.idempotencyKey).toBeTruthy();expect(Number.isInteger(writes[0].body.expectedRevision)).toBe(true);
 const detail=await (await page.request.get(origin+'/api/fluctlights/'+fid+'/detail')).json();
 expect(detail.identity.name).not.toBe('林青的提议称呼');
});
test('invalid advanced JSON retains its exact input and can recover into normal fields',{tag:'@foundation'},async({page})=>{
 await page.goto(origin+'/#/c/'+fid+'/governance?tab=foundation');await page.getByRole('button',{name:'创建修订提议',exact:true}).click();
 const form=page.getByRole('dialog',{name:'创建身份核心修订提议',exact:true});
 await form.getByRole('button',{name:'使用高级 JSON',exact:true}).click();
 const input=form.getByRole('textbox',{name:'字段变更（JSON）',exact:true});await input.fill('{ unfinished');
 await form.getByRole('button',{name:'使用常用字段',exact:true}).click();
 await expect(input).toHaveValue('{ unfinished');await expect(form.getByText('字段变更的 JSON 无法解析，请修正当前输入。',{exact:true})).toBeVisible();
 await input.fill('{"identity":{"name":"待核对的名字"}}');await form.getByRole('button',{name:'使用常用字段',exact:true}).click();
 await expect(form.getByRole('textbox',{name:'名字',exact:true})).toHaveValue('待核对的名字');
 await form.getByRole('button',{name:'保存提议（不立即应用）',exact:true}).click();
 await expect(form.locator('div[tabindex="-1"]').filter({hasText:'请填写提议理由'})).toBeFocused();
 await expect(form.getByText('请填写提议理由',{exact:true})).toBeVisible();
 await expectNoLayoutDefects(page,form,{minTarget:page.viewportSize()!.width<640?40:0});
});

test('unknown proposal receipt retries the original body and key while preserving newer form input',{tag:'@foundation'},async({page})=>{
 const writes:any[]=[];
 await page.route(/\/foundation-revisions$/,async route=>{
  if(route.request().method()!=='POST'){await route.continue();return;}
  writes.push(route.request().postDataJSON());
  if(writes.length===1){await route.fulfill({status:503,json:{code:'unavailable',message:'原提交结果暂时未确认。'}});return;}
  await route.continue();
 });
 await page.goto(origin+'/#/c/'+fid+'/governance?tab=foundation');await page.getByRole('button',{name:'创建修订提议',exact:true}).click();
 const form=page.getByRole('dialog',{name:'创建身份核心修订提议',exact:true});
 const name=form.getByRole('textbox',{name:'名字',exact:true});await name.fill('原提议称呼');
 await form.getByRole('textbox',{name:'提议理由',exact:true}).fill('原提议理由');
 const save=form.getByRole('button',{name:'保存提议（不立即应用）',exact:true});await save.click();
 const retry=form.getByRole('button',{name:'核对或重试原提交',exact:true});await expect(retry).toBeVisible();await expect(retry).toBeInViewport({ratio:1});await expect(save).toBeDisabled();
 await name.fill('等待期间的新草稿');await retry.click();
 await expect.poll(()=>writes.length).toBe(2);expect(writes[1]).toEqual(writes[0]);
 await expect(name).toHaveValue('等待期间的新草稿');await expect(form).toBeVisible();
});
