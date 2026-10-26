import fs from 'node:fs';import path from 'node:path';import {createRequire} from 'node:module';
const req=createRequire(path.resolve('apps/web/package.json')),b=createRequire(createRequire(req.resolve('@vitejs/plugin-react')).resolve('@babel/core'));
const {parse}=b('@babel/parser'),traverse=b('@babel/traverse').default,t=b('@babel/types'),generate=b('@babel/generator').default;
const id=t.identifier,prop=(object,name)=>object.properties.find(p=>(p.key.name||p.key.value)===name);
const pure=new Set(['SemanticTree','ProfileForm','PreparationSteps','NotificationGroup','NotificationIssues']);
for(const folder of ['shared','pages'])for(const filename of fs.readdirSync('apps/web/src/react-app/'+folder)){
 if(!filename.endsWith('.jsx'))continue;
 const file='apps/web/src/react-app/'+folder+'/'+filename,ast=parse(fs.readFileSync(file,'utf8'),{sourceType:'module',plugins:['jsx']});
 const exported=ast.program.body.find(n=>t.isExportDefaultDeclaration(n)&&t.isCallExpression(n.declaration)&&n.declaration.callee.name==='definePage');if(!exported)continue;
 const options=exported.declaration.arguments[0],name=filename.slice(0,-4),create=prop(options,'create'),propsName=create.params[0].name;
 const defaults=[];const propsSpec=prop(options,'props')?.value;
 for(const p of propsSpec?.properties||[]){const spec=p.value;if(t.isObjectExpression(spec)){const def=prop(spec,'default')?.value;if(def)defaults.push(t.objectProperty(t.cloneNode(p.key),t.isArrowFunctionExpression(def)?t.callExpression(def,[]):def));}else if(t.isIdentifier(spec,{name:'Boolean'}))defaults.push(t.objectProperty(t.cloneNode(p.key),t.booleanLiteral(false)));}
 const input=t.objectExpression([...defaults,t.spreadElement(id('input'))]);
 let replacement;
 if(pure.has(name)){
  const body=create.body.body,ret=body.at(-1),render=ret.argument;
  const renderStatements=t.isBlockStatement(render.body)?render.body.body:[t.returnStatement(render.body)];
  const statements=[t.variableDeclaration('const',[t.variableDeclarator(id(propsName),input)]),...body.slice(0,-1),...renderStatements];
  replacement=t.exportDefaultDeclaration(t.functionDeclaration(id(name),[t.assignmentPattern(id('input'),t.objectExpression([]))],t.blockStatement(statements)));
 }else{
  const factory='create'+name+'Controller';
  const factoryNode=t.functionDeclaration(id(factory),create.params,create.body);
  const wrapper=t.exportDefaultDeclaration(t.functionDeclaration(id(name),[t.assignmentPattern(id('input'),t.objectExpression([]))],t.blockStatement([t.returnStatement(t.callExpression(id('useController'),[id(factory),input]))])));
  const index=ast.program.body.indexOf(exported);ast.program.body.splice(index,1,factoryNode,wrapper);replacement=null;
 }
 if(replacement)ast.program.body[ast.program.body.indexOf(exported)]=replacement;
 traverse(ast,{
  ImportSpecifier(p){if(p.node.imported.name==='definePage'){p.node.imported=id('useController');p.node.local=id('useController')}},
  CallExpression(p){
   const c=p.node.callee;
   if(t.isIdentifier(c,{name:'_resolveComponent'}))p.replaceWith(id(name));
   if(t.isMemberExpression(c)&&c.object.name==='lifecycle'&&c.property.name==='emit'&&pure.has(name)){
     const event=p.node.arguments[0].value;const handler='on'+event[0].toUpperCase()+event.slice(1);p.replaceWith(t.optionalCallExpression(t.memberExpression(id(propsName),id(handler)),p.node.arguments.slice(1),true));
   }
  }
 });
 // Meaningful view property names replace the compiler's abbreviated boundary.
 const viewMaps={};
 traverse(ast,{CallExpression(p){if(!t.isIdentifier(p.node.callee)||!/^render\w+Page$/.test(p.node.callee.name))return;let object=p.node.arguments[0];if(t.isIdentifier(object))object=p.scope.getBinding(object.name)?.path.node.init;if(!t.isObjectExpression(object))return;const mapping={};for(const item of object.properties){if(t.isObjectProperty(item)&&t.isIdentifier(item.value)&&item.key.name!==item.value.name){mapping[item.key.name]=item.value.name;item.key=id(item.value.name);item.shorthand=true;}}viewMaps[p.node.callee.name]=mapping;}});
 for(const [render,mapping]of Object.entries(viewMaps)){
  const renderFile='apps/web/src/react-app/renderers/'+name+'.jsx';if(!fs.existsSync(renderFile))continue;
  const ra=parse(fs.readFileSync(renderFile,'utf8'),{sourceType:'module',plugins:['jsx']});
  traverse(ra,{MemberExpression(p){if(p.node.object.name==='__view'&&!p.node.computed&&mapping[p.node.property.name])p.node.property=id(mapping[p.node.property.name]);},JSXMemberExpression(p){if(p.node.object.name==='__view'&&mapping[p.node.property.name])p.node.property=t.jsxIdentifier(mapping[p.node.property.name]);},Identifier(p){if(p.node.name==='__view')p.node.name='view';}});
  fs.writeFileSync(renderFile,generate(ra,{comments:false,jsescOption:{minimal:true}}).code+'\n');
 }
 traverse(ast,{Program(p){p.scope.crawl();},ImportDeclaration(p){p.node.specifiers=p.node.specifiers.filter(spec=>p.scope.getBinding(spec.local.name)?.referenced);if(!p.node.specifiers.length)p.remove();}});
 fs.writeFileSync(file,generate(ast,{comments:false,jsescOption:{minimal:true}}).code+'\n');
}
