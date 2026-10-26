// One-time source migration. This tool is never shipped in the browser.
import fs from 'node:fs';import path from 'node:path';import {createRequire} from 'node:module';
const root=process.cwd(),src=path.join(root,'apps/web/src/approved-ui'),out=path.join(root,'apps/web/src/react-app');
const req=createRequire(path.join(root,'apps/web/package.json'));
const babel=createRequire(createRequire(req.resolve('@vitejs/plugin-react')).resolve('@babel/core'));
const {parse}=babel('@babel/parser'), traverse=babel('@babel/traverse').default, generate=babel('@babel/generator').default,t=babel('@babel/types');
const sfc=req('vue/compiler-sfc');
const parseJS=s=>parse(s,{sourceType:'module',plugins:['jsx','typescript']});
const id=t.identifier, call=(name,args)=>t.callExpression(id(name),args), str=t.stringLiteral;
const icons=Object.fromEntries([...fs.readFileSync(path.join(src,'shared/icons.mjs'),'utf8').matchAll(/export const (\w+|\$\w+)=icon\("(\w+)"/g)].map(m=>[m[1],m[2]]));
const controls=['ActionButton','Surface','FieldInput','ChoiceSelect','Textarea','Disclosure','MenuDisclosure','DialogFrame','Badge'];
const generic=['displayText','readValue','eventModifiers','keyModifiers','listValues','setElement','Dynamic','Fragment'];
const aliases=new Map();
const lifecycleNames={ref:'cell',reactive:'model',watch:'observe',onMounted:'onStart',onUnmounted:'onDispose',onBeforeUnmount:'onDispose'};
const renameAttr={class:'className',for:'htmlFor',tabindex:'tabIndex',datetime:'dateTime',maxlength:'maxLength',minlength:'minLength',readonly:'readOnly',autofocus:'autoFocus',novalidate:'noValidate',spellcheck:'spellCheck',srcset:'srcSet','stroke-width':'strokeWidth','stroke-linecap':'strokeLinecap','stroke-linejoin':'strokeLinejoin','fill-rule':'fillRule','clip-rule':'clipRule',viewbox:'viewBox',onCompositionstart:'onCompositionStart',onCompositionend:'onCompositionEnd',onKeydown:'onKeyDown',onKeyup:'onKeyUp',onMouseenter:'onMouseEnter',onMouseleave:'onMouseLeave',onMousedown:'onMouseDown','test-id':'testId','show-status':'showStatus','exclude-operation-prefix':'excludeOperationPrefix'};
const jsxName=n=>t.isIdentifier(n)?t.jsxIdentifier(n.name):t.isMemberExpression(n)&&!n.computed?t.jsxMemberExpression(jsxName(n.object),jsxName(n.property)):null;
function element(tag,attrs=[],children=[]){return t.jsxElement(t.jsxOpeningElement(tag,attrs,children.length===0),children.length?t.jsxClosingElement(t.cloneNode(tag)):null,children,children.length===0)}
function child(n){if(!n||t.isNullLiteral(n))return null;if(t.isJSXElement(n)||t.isJSXFragment(n))return n;return t.jsxExpressionContainer(n)}
function children(n){if(!n||t.isNullLiteral(n))return [];if(t.isArrayExpression(n))return n.elements.filter(Boolean).map(x=>t.isSpreadElement(x)?t.jsxExpressionContainer(x.argument):child(x)).filter(Boolean);return [child(n)].filter(Boolean)}
function attr(name,value){return t.jsxAttribute(t.jsxIdentifier(name),value==null?null:t.isStringLiteral(value)?value:t.jsxExpressionContainer(value))}
function prop(node,key){return node?.properties?.find(p=>(p.key.name||p.key.value)===key)}
function attrValue(node,name){const a=node.openingElement.attributes.find(a=>t.isJSXAttribute(a)&&a.name.name===name);return a?.value?.expression||a?.value}
function removeAttr(node,name){node.openingElement.attributes=node.openingElement.attributes.filter(a=>!t.isJSXAttribute(a)||a.name.name!==name)}
function addAttr(node,name,value){removeAttr(node,name);node.openingElement.attributes.push(attr(name,value))}
function textOf(node){return t.isStringLiteral(node)?node.value:''}
function remapImport(value,file){
 if(value==='vue')return null;
 if(value==='./LegacySurface.vue')return '@/react-app/LegacySurface';
 if(value.endsWith('/shared/icons.mjs')||value==='./icons.mjs')return 'lucide-react';
 if(value.includes('shared/primitives')||value==='./primitives.mjs')return '@/react-app/controls';
 if(value.includes('shared/components')||value==='./components.mjs')return '@/react-app/components';
 if(value.endsWith('/workspace.mjs'))return '@/state/workspace.mjs';
 if(value.includes('creation-controller.mjs'))return '@/state/creation-controller.mjs';
 const resolved=path.resolve(path.dirname(file),value);
 if(value.startsWith('.') && resolved.startsWith(src)){
   const rel=path.relative(src,resolved);
   if(rel.startsWith('pages/')||rel.startsWith('shared/')||rel.startsWith('renderers/'))return '@/react-app/'+rel.replace(/\.(vue|mjs)$/,'');
   return '@/approved-ui/'+rel;
 }
 if(value.startsWith('.'))return '@/'+path.relative(path.join(root,'apps/web/src'),resolved).replaceAll(path.sep,'/');
 return value;
}
function transform(code,file,isSfc=false){
 const ast=parseJS(code),helpers=new Map(),constants=new Map();
 for(const statement of ast.program.body){
  if(t.isImportDeclaration(statement)&&statement.source.value==='vue')for(const spec of statement.specifiers)helpers.set(spec.local.name,spec.imported?.name);
  if(t.isVariableDeclaration(statement))for(const dec of statement.declarations)if(t.isIdentifier(dec.id))constants.set(dec.id.name,dec.init);
 }
 // Inline render attribute constants; eliminate compiler cache identities.
 function resolve(n){return t.isIdentifier(n)&&constants.has(n.name)?t.cloneNode(constants.get(n.name),true):n}
 function attrs(n){n=resolve(n);if(!n||t.isNullLiteral(n))return [];if(!t.isObjectExpression(n))return [t.jsxSpreadAttribute(n)];return n.properties.filter(p=>t.isSpreadElement(p)||!['ref_key','ref_for','__scopeId'].includes(p.key?.name||p.key?.value)).map(p=>{
  if(t.isSpreadElement(p))return t.jsxSpreadAttribute(p.argument);
  let name=p.key.name||p.key.value,value=p.value;
  if(name==='ref')return attr('ref',call('elementRef',[value]));
  if(name.startsWith('onUpdate:'))name='onUpdate'+name.slice(9).replace(/(^|[-:])(\w)/g,(_,a,b)=>b.toUpperCase());
  name=renameAttr[name]||name;
  return attr(name,value);
 })}
 function jsxNode(args,kind){
  let [tag,props,content]=args;
  if(kind==='button'){content=props;props=tag;tag=id('ActionButton')}
  if(kind==='surface'){content=args[2];props=t.objectExpression([t.objectProperty(id('as'),tag),...(t.isObjectExpression(resolve(props))?resolve(props).properties:[t.spreadElement(props||t.objectExpression([]))])]);tag=id('Surface')}
  if(kind==='h'&&args.length===2&&!t.isObjectExpression(props)){content=props;props=null}
  if(t.isObjectExpression(content)){
   const slot=prop(content,'default')?.value;
   const fn=t.isCallExpression(slot)&&slot.callee.name==='slotContent'?slot.arguments[0]:slot;
   content=t.isArrowFunctionExpression(fn)||t.isFunctionExpression(fn)?(t.isBlockStatement(fn.body)?t.callExpression(fn,[]):fn.body):null;
  }
  if(t.isCallExpression(tag)&&tag.callee.name==='readValue')tag=tag.arguments[0];
  let tagName=t.isStringLiteral(tag)?tag.value:helpers.get(tag?.name)==='Fragment'?'Fragment':null;
  let attributes=attrs(props);
  const propObject=resolve(props),classText=textOf(prop(propObject,'class')?.value);
  if(tagName==='button')tagName='ActionButton';
  if(tagName==='input')tagName='FieldInput';
  if(tagName==='textarea')tagName='Textarea';
  if(tagName==='select')tagName='ChoiceSelect';
  if(tagName==='span'&&/(^| )(pill|current-badge)( |$)/.test(classText))tagName='Badge';
  if(tagName==='a'&&/(^| )(button|text-button)( |$)/.test(classText)){
   const node=element(t.jsxIdentifier('a'),attributes,children(content));
   return element(t.jsxIdentifier('ActionButton'),[attr('asChild'),attr('className',str(classText))],[node]);
  }
  if(tagName==='Transition'||helpers.get(tag?.name)==='Transition')tagName='Fragment';
  if(tagName==='Teleport'||helpers.get(tag?.name)==='Teleport'){tagName='Fragment';attributes=[];}
  let name=tagName?t.jsxIdentifier(tagName):jsxName(tag);
  if(!name){name=t.jsxIdentifier('Dynamic');attributes.unshift(attr('as',tag));}
  const node=element(name,attributes,children(content));
  if(tagName==='details'){
   const summary=node.children.find(n=>t.isJSXElement(n)&&n.openingElement.name.name==='summary');
   if(summary){node.openingElement.name=t.jsxIdentifier(classText.includes('actions-menu')?'MenuDisclosure':'Disclosure');node.closingElement.name=t.cloneNode(node.openingElement.name);addAttr(node,'title',t.jsxFragment(t.jsxOpeningFragment(),t.jsxClosingFragment(),summary.children));
    const label=attrValue(summary,'aria-label');if(label)addAttr(node,'aria-label',label);
    node.children=node.children.filter(n=>n!==summary);
   }
  }
  if(['overlay','modal-backdrop','dialog-backdrop'].includes(classText)){
    node.openingElement.name=t.jsxIdentifier('DialogFrame');node.closingElement.name=t.jsxIdentifier('DialogFrame');
    const click=attrValue(node,'onClick');if(click){removeAttr(node,'onClick');addAttr(node,'onClose',t.isCallExpression(click)&&click.callee.name==='eventModifiers'?click.arguments[0]:click)}
  }
  return node;
 }
 traverse(ast,{
  ImportDeclaration(p){
   const value=p.node.source.value;
   if(value==='vue'||value.includes('render-runtime')){p.remove();return}
   const next=remapImport(value,file);p.node.source.value=next;
   if(next==='lucide-react')for(const spec of p.node.specifiers){const original=spec.imported?.name;if(icons[original]){p.scope.rename(spec.local.name,icons[original]);spec.imported=id(icons[original]);}}
   if(value.endsWith('/workspace.mjs')){p.node.specifiers=p.node.specifiers.filter(s=>!['useWorkspace','WorkspaceKey','createWorkspace'].includes(s.imported?.name));if(!p.node.specifiers.length)p.remove()}
   if(value.includes('primitives.mjs')){p.remove()}
  },
  LogicalExpression:{exit(p){const n=p.node;if(n.operator==='||'&&t.isAssignmentExpression(n.right)&&t.isMemberExpression(n.left)&&n.left.computed&&t.isNumericLiteral(n.left.property))p.replaceWith(n.right.right)}},
  SequenceExpression:{exit(p){const kept=p.node.expressions.filter(n=>!t.isNullLiteral(n));if(kept.length===1)p.replaceWith(kept[0]);else p.node.expressions=kept;}},
  MemberExpression:{exit(p){if(p.node.property.name==='$emit')p.replaceWith(t.memberExpression(id('lifecycle'),id('emit')))}},
  ObjectMethod(p){if(p.node.key.name==='setup'){p.node.key=id('create');p.node.params=[p.node.params[0]||id('props'),p.node.params[1]||id('context'),id('lifecycle')]}},
  ObjectProperty(p){if(p.node.key?.name==='__scopeId')p.remove()},
  CallExpression:{exit(p){
   const n=p.node,name=t.isIdentifier(n.callee)?n.callee.name:null,helper=helpers.get(name)||name;
   if(name==='useWorkspace'||name==='createWorkspace'){p.replaceWith(t.memberExpression(id('lifecycle'),id('workspace')));return}
   if(name==='provide'||name==='__expose'){p.remove();return}
   if(helper in lifecycleNames){n.callee=t.memberExpression(id('lifecycle'),id(lifecycleNames[helper]));return}
   if(helper==='computed'){n.callee=id('selector');return}
   if(helper==='nextTick'){n.callee=id('afterPaint');return}
   if(helper==='defineComponent'){n.callee=id('definePage');return}
   if(helper==='unoptimizedRenderer'){p.replaceWith(n.arguments[0]);return}
   if(helper==='normalizeClass'){n.callee=id('cn');return}
   if(['normalizeStyle','normalizeProps','guardReactiveProps','resolveDynamicComponent'].includes(helper)){p.replaceWith(n.arguments[0]);return}
   if(helper==='unref'){n.callee=id('readValue');return}
   if(helper==='isRef'){n.callee=id('isCell');return}
   if(helper==='toDisplayString'){n.callee=id('displayText');return}
   if(helper==='withModifiers'){n.callee=id('eventModifiers');return}
   if(helper==='withKeys'){n.callee=id('keyModifiers');return}
   if(helper==='withCtx'){n.callee=id('slotContent');return}
   if(helper==='createCommentVNode'||helper==='openBlock'){p.replaceWith(t.nullLiteral());return}
   if(helper==='createTextVNode'){p.replaceWith(n.arguments[0]||str(''));return}
   if(helper==='mergeProps'){p.replaceWith(t.objectExpression(n.arguments.map(a=>t.spreadElement(a))));return}
   if(helper==='renderList'){p.replaceWith(t.callExpression(t.memberExpression(call('listValues',[n.arguments[0]]),id('map')),[n.arguments[1]]));return}
   if(helper==='renderSlot'){p.replaceWith(t.memberExpression(id('props'),id('children')));return}
   if(['createElementVNode','createElementBlock','createVNode','createBlock','h'].includes(helper)||['approvedButtonVNode','approvedButtonBlock','approvedSurfaceVNode','approvedSurfaceBlock'].includes(name)){
    p.replaceWith(jsxNode(n.arguments,name?.includes('Button')?'button':name?.includes('Surface')?'surface':helper==='h'?'h':null));return
   }
   if(helper==='withDirectives'){
    const node=n.arguments[0];if(!t.isJSXElement(node))throw Error('Unsupported directive target '+file);
    for(const directive of n.arguments[1].elements){const [kind,value,,mods]=directive.elements;const modelKind=helpers.get(kind.name)||kind.name;
      if(modelKind==='vShow'){addAttr(node,'hidden',t.unaryExpression('!',value));continue}
      const update=attrValue(node,'onUpdateModelValue');if(!update)throw Error('Missing model setter '+file);
      removeAttr(node,'onUpdateModelValue');
      if(modelKind==='vModelCheckbox'){
       addAttr(node,'modelValue',value);addAttr(node,'onModelChange',update);
      }else{
       const numeric=!!prop(mods,'number')||textOf(attrValue(node,'type'))==='number';
       addAttr(node,'value',value);
       const e=id('event'),input=t.memberExpression(t.memberExpression(e,id('target')),id('value'));
       const expression=numeric?call('numberInput',[input]):input;
       const existing=attrValue(node,'onChange'),inputHandler=attrValue(node,'onInput');removeAttr(node,'onInput');
       const statements=[t.expressionStatement(t.callExpression(update,[expression]))];if(inputHandler)statements.push(t.expressionStatement(t.callExpression(inputHandler,[e])));if(existing)statements.push(t.expressionStatement(t.callExpression(existing,[e])));
       addAttr(node,'onChange',t.arrowFunctionExpression([e],t.blockStatement(statements)));
      }
    }
    p.replaceWith(node);return;
   }
   if(name==='createCreationController'&&t.isObjectExpression(n.arguments[0]))n.arguments[0].properties.push(t.objectProperty(id('cell'),t.memberExpression(id('lifecycle'),id('cell'))),t.objectProperty(id('model'),t.memberExpression(id('lifecycle'),id('model'))));
  }},
  ReferencedIdentifier(p){if(p.node.name!=='Fragment'&&helpers.get(p.node.name)==='Fragment')p.replaceWith(id('Fragment'));},
 });
 // compileScript emits the options object directly.
 if(isSfc){const exp=ast.program.body.find(n=>t.isExportDefaultDeclaration(n));if(t.isObjectExpression(exp?.declaration))exp.declaration=call('definePage',[exp.declaration]);}
 // Static attrs constants are folded above. Remove unused compiler-only arrays.
 traverse(ast,{Program(p){p.scope.crawl()},VariableDeclarator(p){if(t.isIdentifier(p.node.id)&&p.parentPath.parentPath.isProgram()&&!p.scope.getBinding(p.node.id.name)?.referenced)p.remove()}});
 const imports=parseJS(`import { definePage, afterPaint } from '@/react-app/page';\nimport { selector, readValue, setElement, elementRef, valueCell } from '@/state/view-model';\nimport { cn } from '@/lib/utils';\nimport { ${controls.join(', ')} } from '@/react-app/controls';\nimport { displayText, eventModifiers, keyModifiers, listValues, Dynamic, Fragment, slotContent, isCell, numberInput } from '@/react-app/render-helpers';`).program.body;
 ast.program.body.unshift(...imports);
 const output=generate(ast,{comments:false,jsescOption:{minimal:true}}).code;
 return output+'\n';
}
function write(rel,code){const destination=path.join(out,rel);fs.mkdirSync(path.dirname(destination),{recursive:true});fs.writeFileSync(destination,code)}
for(const folder of ['pages','shared'])for(const filename of fs.readdirSync(path.join(src,folder))){
 if(!filename.endsWith('.vue'))continue;
 if(['CharacterAvatar.vue','SettingsField.vue','ProfileField.vue'].includes(filename))continue;
 const file=path.join(src,folder,filename),descriptor=sfc.parse(fs.readFileSync(file,'utf8'),{filename:file}).descriptor;
 const compiled=sfc.compileScript(descriptor,{id:'react-source-migration',inlineTemplate:true,templateOptions:{compilerOptions:{hoistStatic:false,cacheHandlers:false}}});
 write(folder+'/'+filename.replace('.vue','.jsx'),transform(compiled.content,file,true));
}
for(const folder of ['pages','renderers'])for(const filename of fs.readdirSync(path.join(src,folder))){
 if(!filename.endsWith('.mjs')||filename==='DetailDrawer.mjs')continue;
 const file=path.join(src,folder,filename);
 write(folder+'/'+filename.replace('.mjs','.jsx'),transform(fs.readFileSync(file,'utf8'),file));
}

write('ApprovedApp.jsx',transform(fs.readFileSync(path.join(src,'ApprovedApp.mjs'),'utf8'),path.join(src,'ApprovedApp.mjs')));
