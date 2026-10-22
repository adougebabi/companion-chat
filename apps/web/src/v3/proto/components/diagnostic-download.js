import {diagnosticsClient} from '../services.js';

// Prefer writing to the user-selected file while streaming. The fallback Blob
// has an explicit bound and is offered only after the terminal footer passed.
export async function downloadDiagnostics(filters,{alive=()=>true,feedback=()=>{}}={}){
 let file,writer,response,reader,bytes=0,tail='',chunks=[],url;
 try{
  if(window.showSaveFilePicker){try{file=await window.showSaveFilePicker({suggestedName:'fluctlight-diagnostics.jsonl',types:[{description:'诊断 JSONL',accept:{'application/x-ndjson':['.jsonl']}}]});if(!alive())return;writer=await file.createWritable();}catch(error){if(error.name==='AbortError'){feedback('已取消文件选择，没有开始导出。');return;}if(!['SecurityError','NotAllowedError','NotSupportedError'].includes(error.name))throw error;feedback('当前浏览器无法选择保存位置，将在完整读取后下载文件。');}}
  response=await diagnosticsClient.export(filters);if(!response.body)throw new Error('浏览器没有返回可读取的导出流');reader=response.body.getReader();const decoder=new TextDecoder();
  for(;;){if(!alive())throw new Error('已停止当前页面的导出观察');const {value,done}=await reader.read();if(done)break;bytes+=value.byteLength;tail=(tail+decoder.decode(value,{stream:true})).slice(-65536);if(writer)await writer.write(value);else{if(bytes>128*1024*1024)throw new Error('导出超过浏览器内存上限；请使用支持选择文件并流式写入的浏览器。部分数据不会作为完整文件下载。');chunks.push(value);}feedback(`已接收 ${(bytes/1024/1024).toFixed(1)} MiB，仍在等待完整性回执…`);}
  tail+=decoder.decode();const lines=tail.trim().split('\n');let terminal;try{terminal=JSON.parse(lines.at(-1));}catch{throw new Error('导出缺少完整性回执；文件可能被中断，请重新导出。');}
  if(terminal.type!=='end'||terminal.complete!==true)throw new Error('服务端没有确认导出完整。'+(terminal.message||terminal.code||''));
  if(writer){await writer.close();writer=null;}else{url=URL.createObjectURL(new Blob(chunks,{type:'application/x-ndjson'}));const anchor=document.createElement('a');anchor.href=url;anchor.download='fluctlight-diagnostics.jsonl';document.body.append(anchor);anchor.click();anchor.remove();}
  feedback('完整导出已完成。实际各来源记录数：'+JSON.stringify(terminal.counts));return terminal;
 }catch(error){if(writer)await writer.abort().catch(()=>{});if(reader)await reader.cancel().catch(()=>{});throw error;}finally{if(url)setTimeout(()=>URL.revokeObjectURL(url),60000);}
}
