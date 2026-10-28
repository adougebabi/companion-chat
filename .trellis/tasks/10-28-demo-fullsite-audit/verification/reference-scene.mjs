// Reproducible reference UI precondition, never editing source/store/DOM.
// Demo's initial worn array has four items while its runtime caption has three.
// Applying its existing first plan synchronizes both through the native UI.
export async function primeReferenceWearing(page){
 await page.goto('http://127.0.0.1:4179/fluctlight-offline.html#/c/linwan/life/wardrobe',{waitUntil:'networkidle'});
 await page.getByRole('tab',{name:/穿搭方案/}).click();
 await page.locator('.r-outfit-card').first().getByRole('button',{name:'请换上这套',exact:true}).click();
 await page.getByRole('button',{name:'发送换装请求',exact:true}).click();
 await page.getByText('换装已完成',{exact:true}).waitFor();
 await page.getByRole('button',{name:'收起换装结果',exact:true}).click();
 const toast=page.locator('[data-sonner-toast]').first();
 if(await toast.count()){await toast.locator('button').first().click();await toast.waitFor({state:'hidden'});}
 await page.getByRole('tab',{name:/全部衣服/}).click();
 return ['打开原Demo衣柜方案库','请求原有第一套四件方案','等待原Demo确认完成','用原生按钮收起结果及提示','保留原方案和四件确认穿着'];
}
