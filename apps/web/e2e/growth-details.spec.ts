import {test,expect,type Page} from '@playwright/test';
import {readManifest,verifyBaselines} from './support/baselines.mjs';
import {expectNoLayoutDefects} from './support/layout';
const origin='http://127.0.0.1:5197',fid='fluctlight-fixture-one';
const memory={id:'memory-detail-fixture',owner_fluctlight_id:fid,type:'episodic',status:'active',revision:2,content:'我们沿着河边散步，聊起了那本旧书。',importance:0,confidence:.8,provenance_status:'valid',created_at:'2026-11-23T01:44:08Z',extra:{sample:[null,false,0]}};
const reflection={id:'reflection-detail-fixture',fluctlight_id:fid,title:'关于倾听的一次观察',summary:'她发现先听完对方的想法，交流会更轻松。',body:'下次准备给建议前，先确认对方需要什么。',status:'applied',at:'2026-11-23T01:44:08Z',source_window:{evidence:[{kind:'conversation.turn',occurred_at:'2026-11-23T01:40:00Z'}]},proposal:{memory_candidates:[{operation:'create',content:'记下先倾听的感受。'}]},dispositions:[{domain:'memory',candidate_index:0,disposition:'accepted'}],extra:{sample:[null,false,0]}};
async function fixtures(page:Page){
 await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
 await page.route(/\/api\/fluctlights\/[^/]+\/(memories|self\/claims|reflections)(?:\?|$)/,route=>route.fulfill({json:{items:[],next_cursor:''}}));
 await page.route(/\/api\/memories\/memory-detail-fixture(?:\?|$)/,route=>route.fulfill({json:memory}));
 await page.route(/\/api\/memories\/memory-detail-fixture\/revisions/,route=>route.fulfill({json:{items:[{...memory,revision:1,content:'较早记得我们在看雨。',operation:'revise'}],next_cursor:''}}));
 await page.route(/\/api\/memories\/memory-detail-fixture\/sources/,route=>route.fulfill({json:{items:[{source_ref:'source-fixture',source_kind:'message',source_id:'message-fixture',conversation_id:'conversation-fixture',memory_revision:Number(new URL(route.request().url()).searchParams.get('revision')),live:true}],next_cursor:''}}));
 await page.route(/\/api\/fluctlights\/[^/]+\/reflections\/reflection-detail-fixture(?:\?|$)/,route=>route.fulfill({json:reflection}));
 await page.route(/\/api\/fluctlights\/[^/]+\/reflections\/reflection-detail-fixture\/feedback/,route=>route.fulfill({json:{items:[],next_cursor:''}}));
}
async function screenshot(page:Page,name:string){
 const file=test.info().project.name+'/'+name;
 if(!process.env.UI_BASELINE_APPROVAL_REASON&&(verifyBaselines().status!=='ok'||!readManifest()?.files?.[file]))throw Error('UI_BASELINE_UNVERIFIED: '+file);
 await expect(page.getByRole('dialog')).toHaveScreenshot(name);
}
test.beforeEach(async({page})=>{await page.clock.setFixedTime(new Date('2026-11-23T10:30:00+08:00'));await fixtures(page);});
test('memory object uses shared detail outside the list, selects historical sources and returns to its origin',{tag:['@memory','@visual']},async({page})=>{
 const query=new URLSearchParams({returnTo:'#/c/'+fid+'/people/owner',returnLabel:'返回关于你'});
 await page.goto(origin+'/#/c/'+fid+'/growth/memory/'+memory.id+'?'+query);
 const detail=page.getByRole('dialog',{name:'记忆详情'});
 await expect(detail.getByText(memory.content,{exact:true})).toBeVisible();
 await expect(detail.getByRole('button',{name:'记错了，纠正一下'})).toBeEnabled();
 await expectNoLayoutDefects(page,detail,{minTarget:page.viewportSize()!.width<640?40:0});
 await screenshot(page,'memory-details-object.png');
 await detail.getByRole('tab',{name:'原始来源',exact:true}).click();
 await expect(detail.getByText('来源有效',{exact:true})).toBeVisible();
 await detail.getByRole('tab',{name:'修订记录',exact:true}).click();
 await detail.getByRole('button',{name:'查看此版来源'}).click();
 await expect(detail.getByRole('tab',{name:'原始来源',exact:true})).toHaveAttribute('aria-selected','true');
 await expect(detail.getByText(/这次历史修订的来源/)).toBeVisible();
 await detail.getByRole('button',{name:'返回关于你',exact:true}).click();
 await expect(page).toHaveURL(new RegExp('/c/'+fid+'/people/owner$'));
});
test('reflection object reuses proposals and feedback form while retaining timeline navigation',{tag:['@memory','@visual']},async({page})=>{
 await page.goto(origin+'/#/c/'+fid+'/growth/reflection/'+reflection.id);
 const detail=page.getByRole('dialog',{name:reflection.title});
 await expect(detail.getByText(reflection.summary,{exact:true})).toBeVisible();
 await expect(detail.getByRole('link',{name:'反思时间线与处理进度'})).toHaveAttribute('href','#/c/'+fid+'/growth/reflection');
 await detail.getByRole('button',{name:'提议与实际改变',exact:true}).click();
 await expect(detail.getByText('记下先倾听的感受。',{exact:true})).toBeVisible();
 await expectNoLayoutDefects(page,detail,{minTarget:page.viewportSize()!.width<640?40:0});
 await screenshot(page,'reflection-details-object.png');
 await detail.getByRole('button',{name:'赞同这次观察',exact:true}).click();
 const form=page.getByRole('dialog',{name:'赞同这次观察',exact:true});
 await expect(form.getByText(/提交真实反馈请求/)).toBeVisible();
 await form.getByRole('textbox',{name:'反思反馈内容'}).fill('我也想先听完。');
 await expect(form.getByRole('button',{name:'提交反馈',exact:true})).toBeEnabled();
 await form.getByRole('button',{name:'取消',exact:true}).click();
 await expect(detail).toBeVisible();
 await page.keyboard.press('Escape');
 await expect(page).toHaveURL(new RegExp('/c/'+fid+'/growth\\?tab=reflection$'));
});
test('unread memory failure stays readonly and retry reads the same ID with long content',{tag:'@memory'},async({page})=>{
 let reads=0;
 const content='我们想把当时的谈话记得更完整。'.repeat(20)+'long-unbroken-source-'.repeat(20);
 await page.route(/\/api\/memories\/memory-detail-fixture$/,async route=>{reads++;await route.fulfill(reads===1?{status:503,json:{code:'unavailable',message:'暂时读不到记忆'}}:{json:{...memory,content}});});
 await page.goto(origin+'/#/c/'+fid+'/growth/memory/'+memory.id);
 const detail=page.getByRole('dialog',{name:'记忆详情'});
 await expect(detail.getByRole('button',{name:'记错了，纠正一下'})).toHaveCount(0);
 await detail.getByRole('button',{name:/重新读取/}).click();
 await expect(detail.getByText(content,{exact:true})).toBeVisible();
 await expectNoLayoutDefects(page,detail,{minTarget:page.viewportSize()!.width<640?40:0});
 expect(reads).toBeGreaterThanOrEqual(2);
});
