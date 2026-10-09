import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import vm from 'node:vm';
import ts from 'typescript';
import test from 'node:test';
import * as navigation from '../src/app/navigation.ts';
const source=await readFile(new URL('../src/App.vue',import.meta.url),'utf8');
const parser=source.slice(source.indexOf('function sectionFromLocation('),source.indexOf('\nfunction syncDiagnosticsFilterFromLocation'));
for(const [view,section] of [['settings','kev'],['diagnostics','kev-decisions']]){
 test(`${section} survives direct URL and refresh`,()=>{
  const js=ts.transpileModule(parser,{compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText;
  const context=vm.createContext({...navigation,URLSearchParams,window:{location:{search:`?section=${section}`}}});
  vm.runInContext(js,context);assert.equal(context.sectionFromLocation(view),section);
 });
 test(`${section} can be selected from sidebar`,()=>{
  const name=view==='settings'?'activeSettingsSection':'activeDiagnosticsSection';
  const line=source.split('\n').find(line=>line.startsWith(`const ${name} =`));
  const expr=line.slice(line.indexOf('(() => ')+7,-2);
  const js=ts.transpileModule(`globalThis.result = ${expr}`,{compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText;
  const context=vm.createContext({...navigation,view:{value:view},activeSection:{value:section}});vm.runInContext(js,context);assert.equal(context.result,section);
 });
}
