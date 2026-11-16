import test from 'node:test';
import assert from 'node:assert/strict';
import {momentBodyText,noticeSummaryText,noticeBodyText,HIDDEN_NOTICE_BODY,NOTICE_SUMMARY_LIMIT} from '../src/lib/public-body.mjs';
import {momentReading} from '../src/v3/moment-reading.mjs';

const leakedProtocol='私聊运行协议 你以当前生效的人格通过聊天软件与对方打字交流。人格设定 {"working_persona":{"interpretation_boundary":"原文完整保留。"},"portrait_text":"FT-陈默是一名男性咖啡店店长"} 需要外貌细节时使用 persona.detail 查询。';
const assistantVoice='协议测试 看起来你直接发起了交流，没有传递协议相关指令。 我是一个开放的人工智能助手，可以聊天交流。作为AI，我每天都在处理各种对话。';

test('notice summaries hide leaked chat protocol, raw persona JSON and assistant voice',()=>{
 for(const body of [leakedProtocol,assistantVoice]){
  for(const reply of [true,false]){
   const summary=noticeSummaryText(body,reply),detail=noticeBodyText(body,reply);
   assert.equal(detail,HIDDEN_NOTICE_BODY);
   assert.doesNotMatch(summary+detail,/运行协议|working_persona|interpretation_boundary|portrait_text|persona\.detail|人工智能助手|\{"/);
  }
 }
});

test('notice summaries are single-line and bounded while ordinary replies stay intact',()=>{
 assert.equal(noticeSummaryText('（把咖啡杯放回架子）\n\n你要是也喜欢这类活动，可以一起？',true),'（把咖啡杯放回架子） 你要是也喜欢这类活动，可以一起？');
 const long='今天的书架整理得差不多了。'.repeat(30),summary=noticeSummaryText(long,true);
 assert.equal([...summary].length,NOTICE_SUMMARY_LIMIT+1);assert.ok(summary.endsWith('…'));
 assert.equal(noticeSummaryText('可以去相册看看。',false),'可以去相册看看。');
 const json='{"phase":"generating","description":"正常业务通知"}';assert.equal(noticeSummaryText(json,false),json);
});

const wrapped=`我正在生成一条面向动态参与者的生活动态。根据当前人格、目标和生活状态，我将发布以下内容：

---

**FT-苏晴** 20:08 · 图书馆日常

桂花季又到了，想起了那年在旧书架底发现的手抄本。桂花糕的香气和发黄的纸页，这两样古旧物事，总让人生出些微的怀乡之情。

晚间在三楼整理古籍目录。明天的展览需要调整布置，新到的一套明代刻本需要特别标注保护提示。

---

这条动态基于当前时间（20:08），结合了FT-苏晴作为图书馆管理员的身份特征，提及了她的偏好（旧书与桂花糕），以及当前的实际工作状态。动态正文自主生成，不包含未经授权的图片或媒体内容。`;
const body='桂花季又到了，想起了那年在旧书架底发现的手抄本。桂花糕的香气和发黄的纸页，这两样古旧物事，总让人生出些微的怀乡之情。\n\n晚间在三楼整理古籍目录。明天的展览需要调整布置，新到的一套明代刻本需要特别标注保护提示。';

test('moment body drops model preamble, Markdown wrapper and generation note',()=>{
 assert.equal(momentBodyText(wrapped),body);
 assert.equal(momentBodyText(body),body,'clean moments stay unchanged');
 assert.equal(momentBodyText('今天把**桂花糕**分给了同事。\n\n**真好**'),'今天把桂花糕分给了同事。\n\n真好');
 const reading=momentReading({author_type:'fluctlight',text:wrapped});
 assert.equal(reading.text,body);assert.equal(reading.corrected,false);assert.equal(reading.original,'');
 assert.equal(momentReading({author_type:'user',text:wrapped}).text,wrapped,'owner-written moments are never rewritten');
});
