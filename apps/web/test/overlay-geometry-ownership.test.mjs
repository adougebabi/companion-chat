// Static guard (not UI acceptance): only styles/overlay.css may set the
// geometry of a sheet itself. Business CSS styles sheet *content*, never the
// sheet box. Real layout is verified in Chromium (e2e/).
import test from 'node:test';
import assert from 'node:assert/strict';
import {readdir,readFile} from 'node:fs/promises';
import {join,relative} from 'node:path';
import {fileURLToPath} from 'node:url';

const src=fileURLToPath(new URL('../src/',import.meta.url));
const OWNER='styles/overlay.css';
const SHEET=/\[data-slot=["']?sheet-content["']?\]|\[data-responsive-sheet\]|\.product-sheet\b|\.responsive-sheet\b|\.schedule-settings-sheet\b|\.replica-creation-task-sheet\b|\.people-source-overlay\b/;
const GEOMETRY=/^(width|min-width|max-width|height|min-height|max-height|padding(-[a-z]+)?|overflow(-[xy])?|position|inset|top|right|bottom|left|display|flex-direction)$/;

async function cssFiles(dir){const out=[];for(const entry of await readdir(dir,{withFileTypes:true})){const path=join(dir,entry.name);if(entry.isDirectory())out.push(...await cssFiles(path));else if(entry.name.endsWith('.css'))out.push(path);}return out;}
function rules(css){
 const found=[];const clean=css.replace(/\/\*[\s\S]*?\*\//g,'');
 const re=/([^{}]+)\{([^{}]*)\}/g;let m;
 while((m=re.exec(clean)))found.push({selector:m[1].trim(),body:m[2]});
 return found;
}
const subject=selector=>selector.split(/\s+|>|\+|~/).filter(Boolean).at(-1)||'';

test('only overlay.css sets sheet box geometry',async()=>{
 const violations=[];
 for(const file of await cssFiles(src)){
  const rel=relative(src,file).replaceAll('\\','/');if(rel===OWNER)continue;
  for(const {selector,body} of rules(await readFile(file,'utf8'))){
   for(const part of selector.split(',')){
    if(!SHEET.test(subject(part.trim())))continue;
    const props=body.split(';').map(d=>d.split(':')[0].trim()).filter(p=>GEOMETRY.test(p));
    if(props.length)violations.push(`${rel}: ${part.trim()} { ${props.join(', ')} }`);
   }
  }
 }
 assert.deepEqual(violations,[],'Business CSS must not override sheet geometry:\n'+violations.join('\n'));
});

test('guard catches a reintroduced business width override',()=>{
 const css='@media (width < 640px) { .product-sheet.schedule-settings-sheet { width: 75vw; } }';
 const hit=rules(css).some(({selector,body})=>SHEET.test(subject(selector))&&/width/.test(body));
 assert.equal(hit,true);
});
