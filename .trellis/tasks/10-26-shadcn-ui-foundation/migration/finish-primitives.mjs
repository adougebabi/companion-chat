import fs from'node:fs';import path from'node:path';import{createRequire}from'node:module';const r=createRequire(path.resolve('apps/web/package.json')),b=createRequire(createRequire(r.resolve('@vitejs/plugin-react')).resolve('@babel/core'));const{parse}=b('@babel/parser'),tr=b('@babel/traverse').default,t=b('@babel/types'),g=b('@babel/generator').default;
const id=t.identifier;
for(const folder of ['renderers','shared','pages'])for(const name of fs.readdirSync('apps/web/src/react-app/'+folder)){
 if(!name.endsWith('.jsx'))continue;const file='apps/web/src/react-app/'+folder+'/'+name,ast=parse(fs.readFileSync(file,'utf8'),{sourceType:'module',plugins:['jsx']});let drawer=false;
 tr(ast,{JSXElement(p){const attrs=p.node.openingElement.attributes,cls=attrs.find(a=>a.name?.name==='className');
 if(cls?.value?.value==='drawer-backdrop'){
  drawer=true;p.node.openingElement.name=t.jsxIdentifier('DrawerFrame');p.node.closingElement.name=t.jsxIdentifier('DrawerFrame');
  const click=attrs.find(a=>a.name?.name==='onClick');if(click){click.name=t.jsxIdentifier('onClose');if(click.value.expression?.callee?.name==='eventModifiers')click.value.expression=click.value.expression.arguments[0];}
 }
 if(attrs.some(a=>a.name?.name==='role'&&a.value?.value==='dialog')){p.node.openingElement.attributes=attrs.filter(a=>!['role','aria-modal'].includes(a.name?.name));if(cls&&t.isStringLiteral(cls.value))cls.value.value=cls.value.value.replace('detail-drawer','drawer-form');}
 },CallExpression(p){if(p.node.callee.property?.name!=='map')return;const fn=p.node.arguments[0];if(!t.isArrowFunctionExpression(fn))return;const rendered=t.isBlockStatement(fn.body)?fn.body.body.find(n=>t.isReturnStatement(n))?.argument:fn.body;
 if(!t.isJSXElement(rendered)||rendered.openingElement.attributes.some(a=>a.name?.name==='key'))return;
 let index=fn.params[1];if(!index){index=id('rowIndex');fn.params.push(index);}const first=fn.params[0];const key=t.isIdentifier(first)?t.conditionalExpression(t.binaryExpression('===',t.unaryExpression('typeof',first),t.stringLiteral('object')),t.logicalExpression('??',t.optionalMemberExpression(first,id('id'),false,true),index),first):index;
 rendered.openingElement.attributes.push(t.jsxAttribute(t.jsxIdentifier('key'),t.jsxExpressionContainer(key)));
 }});
 if(drawer)ast.program.body.unshift(t.importDeclaration([t.importSpecifier(id('DrawerFrame'),id('DrawerFrame'))],t.stringLiteral('@/react-app/controls')));
 if(['ManagementPage.jsx','WardrobePage.jsx'].includes(name)&&folder==='renderers'){
  tr(ast,{FunctionDeclaration(p){if(!p.node.id.name.endsWith('View'))return;const ret=p.node.body.body.find(n=>t.isReturnStatement(n)),root=ret?.argument;if(!t.isJSXElement(root))return;
   const tab=root.children.find(n=>t.isJSXElement(n)&&n.openingElement.attributes.some(a=>a.name?.name==='className'&&a.value?.value==='tabs'));if(!tab)return;
   const subtree=t.file(t.program([t.expressionStatement(tab)]));tr(subtree,{JSXElement(q){if(q.node.openingElement.name.name!=='ActionButton')return;const click=q.node.openingElement.attributes.find(a=>a.name?.name==='onClick')?.value?.expression;if(!t.isArrowFunctionExpression(click)||!t.isAssignmentExpression(click.body))return;const value=click.body.right;q.node.openingElement.name=t.jsxIdentifier('TabsTrigger');q.node.closingElement.name=t.jsxIdentifier('TabsTrigger');q.node.openingElement.attributes=q.node.openingElement.attributes.filter(a=>a.name?.name!=='onClick');q.node.openingElement.attributes.push(t.jsxAttribute(t.jsxIdentifier('value'),t.jsxExpressionContainer(value)));}});
   tab.openingElement.name=t.jsxIdentifier('TabsList');tab.closingElement.name=t.jsxIdentifier('TabsList');
   const current=t.memberExpression(t.memberExpression(id('view'),id('tab')),id('value'));
   const body=root.children.filter(n=>n!==tab);const panel=t.jsxElement(t.jsxOpeningElement(t.jsxIdentifier('TabsContent'),[t.jsxAttribute(t.jsxIdentifier('value'),t.jsxExpressionContainer(current)),t.jsxAttribute(t.jsxIdentifier('className'),t.stringLiteral('min-w-0'))]),t.jsxClosingElement(t.jsxIdentifier('TabsContent')),body);
   const change=t.arrowFunctionExpression([id('value')],t.assignmentExpression('=',current,id('value')));
   ret.argument=t.jsxElement(t.jsxOpeningElement(t.jsxIdentifier('Tabs'),[t.jsxAttribute(t.jsxIdentifier('value'),t.jsxExpressionContainer(current)),t.jsxAttribute(t.jsxIdentifier('onValueChange'),t.jsxExpressionContainer(change))]),t.jsxClosingElement(t.jsxIdentifier('Tabs')),[tab,panel]);
  }});
  ast.program.body.unshift(t.importDeclaration(['Tabs','TabsList','TabsTrigger','TabsContent'].map(n=>t.importSpecifier(id(n),id(n))),t.stringLiteral('@/components/ui/tabs')));
 }
 fs.writeFileSync(file,g(ast,{jsescOption:{minimal:true}}).code+'\n');
}
