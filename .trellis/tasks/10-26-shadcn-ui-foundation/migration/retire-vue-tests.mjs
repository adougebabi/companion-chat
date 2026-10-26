import fs from 'node:fs';import path from 'node:path';import{createRequire}from'node:module';
const r=createRequire(path.resolve('apps/web/package.json')),b=createRequire(createRequire(r.resolve('@vitejs/plugin-react')).resolve('@babel/core'));const{parse}=b('@babel/parser'),t=b('@babel/types'),tr=b('@babel/traverse').default,g=b('@babel/generator').default;
const retireFiles=['goal-panel','goal-planning-panel','ui-interactions','diagnostics','layout','accessibility','detail-display','life-pages','skeleton','phase45-routes'];
const names={
 'approved-ui':['shared primitives preserve native DOM and native button types','extracted render primitives update classes, children and listeners after reactive changes','library renderer unwraps computed filter groups and renders real directory records','extracted app replaces page branches repeatedly without stale block references'],
 'approved-v62':['v6.2 detail and creation retain canonical routes and life navigation'],
 'ui-contracts':['real SFC rendering and store actions honor loading, accessibility and error scope contracts'],
 'r4-recovery':['real Management page stays busy through command readback and keeps a local saved receipt','actual profile and notice components render Chinese traits and collapsed groups'],
 'r8-recovery':['actual world category save persists through reload and world edits keep their identity'],
 'actor-user-background':['creation retains actor_user through edited preview and activation','user background is viewed separately and edited only through owner governance'],
 'capability-requests':['governance exposes the owner capability request pool'],
 'chat-surface':['chat components keep streaming, cancellation and history separate from the store'],
};
const retired=[];
for(const name of [...retireFiles,...Object.keys(names)]){
 const file='apps/web/test/'+name+'.test.mjs';if(!fs.existsSync(file))continue;
 const source=fs.readFileSync(file,'utf8'),ast=parse(source,{sourceType:'module'});
 const tests=ast.program.body.filter(n=>t.isExpressionStatement(n)&&t.isCallExpression(n.expression)&&n.expression.callee.name==='test');
 for(const statement of tests){const title=statement.expression.arguments[0].value;if(retireFiles.includes(name)||names[name]?.includes(title))retired.push({file,test:title,reason:'Removed obsolete Vue component/renderer harness; current behavior coverage moves to react-ui/react-flows and retained domain/provider tests.'});}
 if(retireFiles.includes(name)){fs.unlinkSync(file);continue;}
 ast.program.body=ast.program.body.filter(n=>!tests.includes(n)||!names[name]?.includes(n.expression.arguments[0].value));
 let changed=true;
 while(changed){changed=false;tr(ast,{Program(p){p.scope.crawl();for(const st of p.get('body')){
 if(st.isImportDeclaration()){if(st.node.source.value==='vue'&&st.node.specifiers.length===0){st.remove();changed=true;continue;}const keep=st.node.specifiers.filter(s=>p.scope.getBinding(s.local.name)?.referenced);if(keep.length!==st.node.specifiers.length){changed=true;st.node.specifiers=keep;if(!keep.length)st.remove();}}
 if(st.isFunctionDeclaration()&&!p.scope.getBinding(st.node.id.name)?.referenced){st.remove();changed=true;}
 if(st.isVariableDeclaration()){for(const d of st.get('declarations'))if(t.isIdentifier(d.node.id)&&!p.scope.getBinding(d.node.id.name)?.referenced){d.remove();changed=true;}}
 }}});}
 fs.writeFileSync(file,g(ast,{comments:true,jsescOption:{minimal:true}}).code+'\n');
}
fs.writeFileSync('.trellis/tasks/10-26-shadcn-ui-foundation/research/retired-vue-tests.json',JSON.stringify(retired,null,2)+'\n');console.log('Retired obsolete UI-specific cases:',retired.length);
