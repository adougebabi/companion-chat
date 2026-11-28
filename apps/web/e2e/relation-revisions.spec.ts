import {test,expect} from '@playwright/test';
import {expectNoLayoutDefects} from './support/layout';
const origin='http://127.0.0.1:5197',fid='fluctlight-fixture-one';
test('relation editor keeps the draft and version on a delayed rejection, with one busy submission',{tag:'@relation'},async({page})=>{
 await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
 let release:()=>void=()=>{};const pending=new Promise<void>(resolve=>{release=resolve;});const writes:any[]=[];
 await page.route(/\/api\/fluctlights\/[^/]+\/relationships\/[^/]+$/,async route=>{
  if(route.request().method()!=='PUT'){await route.continue();return;}
  writes.push(route.request().postDataJSON());await pending;await route.fulfill({status:409,json:{code:'revision_conflict',message:'关系版本已变化，请核对当前资料。'}});
 });
 await page.goto(origin+'/#/c/'+fid+'/people/relation/'+fid+'-owner-relation');
 const workspace=page.getByTestId('domain-workspace');
 const summary=workspace.getByRole('textbox',{name:'关系描述',exact:true});
 await expect(summary).toBeVisible({timeout:20000});
 await summary.fill('想补充书店里的相处');
 await workspace.getByRole('textbox',{name:'修正原因',exact:true}).fill('核对最近的实际交流');
 await workspace.getByRole('textbox',{name:'证据引用（逗号分隔）',exact:true}).fill('owner:reading-note');
 await expectNoLayoutDefects(page,workspace,{minTarget:page.viewportSize()!.width<640?40:0});
 const save=workspace.getByRole('button',{name:'保存修正',exact:true});await save.click();
 await expect(save).toBeDisabled();await expect(summary).toBeDisabled();
 expect(writes).toHaveLength(1);expect(Number.isInteger(writes[0].expectedRevision)).toBe(true);
 expect(writes[0].summary).toBe('想补充书店里的相处');expect(writes[0].evidenceRefs).toEqual(['owner:reading-note']);
 release();await expect(save).toBeEnabled();
 await expect(summary).toHaveValue('想补充书店里的相处');
 await expect(workspace.getByRole('alert')).toContainText('关系版本已变化');
 await expect(page).toHaveURL(new RegExp('/people/relation/'+fid+'-owner-relation$'));
});
