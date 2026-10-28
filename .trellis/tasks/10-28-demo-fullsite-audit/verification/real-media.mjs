import {pathToFileURL} from 'node:url';
const {runScenario,check}=await import(pathToFileURL(process.cwd()+'/scripts/e2e/v3-regression/runtime.mjs'));
const {ownerMediaRegression,ownerAvatarRegression}=await import(pathToFileURL(process.cwd()+'/scripts/e2e/v3-regression/media-library.mjs'));
await runScenario('L09','本轮媒体专项：真实上传字节、元数据和显示头像',async t=>{
 const actor=t.object('fixture:fullsite-audit');check(actor?.id,'Use the actual already-created task actor');
 await ownerMediaRegression(t);await ownerAvatarRegression(t,actor);
 t.review('scoped-run','这里只执行现有L09的上传、字节授权、元数据、独立显示头像分支。视觉候选/规范参考/设定图、生成任务和浏览器验证仍未执行，不以该专项替代完整L09。');
});
