import fs from 'node:fs';import path from 'node:path';import {createRequire} from 'node:module';
const req=createRequire(path.resolve('apps/web/package.json')),b=createRequire(createRequire(req.resolve('@vitejs/plugin-react')).resolve('@babel/core'));
const {parse}=b('@babel/parser'),traverse=b('@babel/traverse').default,t=b('@babel/types'),generate=b('@babel/generator').default;
const extra={LifePage:{ye:'moveDiary',q:'clearFilters'},WardrobePage:{he:'removeTag',ee:'saveTags',I:'filteredGarments'},BrowsePage:{ue:'onCommentKeyDown',te:'libraryFilters',a:'openNotificationOrCharacter',r:'openIssue'},ManagementPage:{ye:'addEndpoint',q:'selectKind',V:'selectRecord',he:'recordTime',j:'referenceSection'}};
for(const [name,mapping]of Object.entries(extra))for(const folder of ['pages','renderers']){
 const file=`apps/web/src/react-app/${folder}/${name}.jsx`;const ast=parse(fs.readFileSync(file,'utf8'),{sourceType:'module',plugins:['jsx']});
 traverse(ast,{ObjectProperty(p){if(folder==='pages'&&mapping[p.node.key.name]&&p.node.key.name!==p.node.value?.name)p.node.key=t.identifier(mapping[p.node.key.name]);},MemberExpression(p){if(folder==='renderers'&&p.node.object.name==='view'&&mapping[p.node.property.name])p.node.property=t.identifier(mapping[p.node.property.name]);}});
 fs.writeFileSync(file,generate(ast,{comments:false,jsescOption:{minimal:true}}).code+'\n');
}
for(const filename of fs.readdirSync('apps/web/src/react-app/renderers')){
 if(!filename.endsWith('.jsx'))continue;const file='apps/web/src/react-app/renderers/'+filename,ast=parse(fs.readFileSync(file,'utf8'),{sourceType:'module',plugins:['jsx']});
 const body=ast.program.body.find(n=>t.isFunctionDeclaration(n)&&/^render.*Body$/.test(n.id.name));
 if(body){const view=filename.slice(0,-4)+'View',inner=body.body.body.find(n=>t.isReturnStatement(n))?.argument;
  if(t.isArrowFunctionExpression(inner)){const statements=t.isBlockStatement(inner.body)?inner.body:t.blockStatement([t.returnStatement(inner.body)]);const component=t.exportNamedDeclaration(t.functionDeclaration(t.identifier(view),[t.objectPattern([t.objectProperty(t.identifier('view'),body.params[0])])],statements));
   const oldName=body.id.name;const factory=t.functionDeclaration(t.identifier(oldName),body.params,t.blockStatement([t.returnStatement(t.arrowFunctionExpression([],t.jsxElement(t.jsxOpeningElement(t.jsxIdentifier(view),[t.jsxAttribute(t.jsxIdentifier('view'),t.jsxExpressionContainer(body.params[0]))],true),null,[],true)))]));
   ast.program.body.splice(ast.program.body.indexOf(body),1,component,factory);
  }
 }
 traverse(ast,{Program(p){p.scope.crawl();},ImportDeclaration(p){p.node.specifiers=p.node.specifiers.filter(spec=>p.scope.getBinding(spec.local.name)?.referenced);if(!p.node.specifiers.length)p.remove();}});
 fs.writeFileSync(file,generate(ast,{comments:false,jsescOption:{minimal:true}}).code+'\n');
}
// Report unresolved compiler identifiers rather than hiding them in runtime shims.
const globals=new Set(['undefined','console','window','document','location','localStorage','sessionStorage','history','navigator','Date','JSON','Object','Array','Map','Set','String','Number','Boolean','Math','Intl','Promise','Error','URL','URLSearchParams','AbortController','TextEncoder','encodeURIComponent','decodeURIComponent','setTimeout','clearTimeout','setInterval','clearInterval','getComputedStyle','Element','File']);
for(const folder of ['pages','shared','renderers'])for(const filename of fs.readdirSync('apps/web/src/react-app/'+folder)){
 if(!filename.endsWith('.jsx'))continue;const file='apps/web/src/react-app/'+folder+'/'+filename,ast=parse(fs.readFileSync(file,'utf8'),{sourceType:'module',plugins:['jsx']});const unresolved=new Set();
 traverse(ast,{ReferencedIdentifier(p){if(!p.scope.hasBinding(p.node.name)&&!globals.has(p.node.name))unresolved.add(p.node.name)},Program:{exit(p){p.scope.crawl();for(const s of p.get('body'))if(s.isImportDeclaration()){s.node.specifiers=s.node.specifiers.filter(spec=>p.scope.getBinding(spec.local.name)?.referenced);if(!s.node.specifiers.length)s.remove();}}}});
 if(unresolved.size)console.log(file,[...unresolved]);
 fs.writeFileSync(file,generate(ast,{comments:false,jsescOption:{minimal:true}}).code+'\n');
}
