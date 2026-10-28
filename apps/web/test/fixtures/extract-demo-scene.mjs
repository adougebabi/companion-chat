// Explicit test-data extraction from the read-only supplied Demo. Never imported
// by the product or fixture server. The generated data retains source hashes.
import {createRequire} from 'node:module';
import {readFile,writeFile,mkdir,mkdtemp} from 'node:fs/promises';
import {resolve,join} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath} from 'node:url';
import {createHash} from 'node:crypto';
const require=createRequire(new URL('../../package.json',import.meta.url));
const {build}=createRequire(require.resolve('vite'))('esbuild');
const ts=require('typescript');
const demo=resolve(process.argv[2]||'/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo');
const output=resolve(process.argv[3]||fileURLToPath(new URL('./demo-parity-data.json',import.meta.url)));
const exposed={
 'CompanionPages.tsx':['profileOf','SmallArt','seedMessages','seedPersonas','ABOUT_SEED','PEOPLE_SEED','FEED_SEED','COMPANION_NOTICE_SEED'],
 'LifePages.tsx':['SETTINGS_SEED','GOAL_SEED','DIARY_SEED','MEMORY_SEED','REFLECTION_SEED','CLAIM_SEED','TASK_SEED','seedSchedule'],
 'ResourcePages.tsx':['WORLD_SEEDS','RESOURCE_SEEDS','GARMENT_SEEDS','OUTFIT_SEEDS','GarmentArt','mediaSeeds'],
};
const hashes={};
const directory=await mkdtemp(join(tmpdir(),'fluctlight-demo-extract-'));
await build({stdin:{contents:`import * as React from 'react'; import {renderToStaticMarkup} from 'react-dom/server';
import {SEED_CHARACTERS,DEFAULT_RUNTIME} from './src/lib/demo';import * as W from './src/lib/creation';
import * as C from './src/pages/CompanionPages'; import * as L from './src/pages/LifePages'; import * as R from './src/pages/ResourcePages';
import {CharacterArt,WorldArt} from './src/components/illustrations';
export const data={creationFoundation:W.seedFoundation(),creationDescription:W.DESCRIPTION,inputTemplate:W.INPUT_TEMPLATE,profiles:Object.fromEntries(SEED_CHARACTERS.map(c=>[c.id,C.profileOf(c)])),smallArt:renderToStaticMarkup(React.createElement(C.SmallArt,{compact:true})),smallArts:Object.fromEntries(['books','river','garden'].map(kind=>[kind,renderToStaticMarkup(React.createElement(C.SmallArt,{kind}))])),characters:SEED_CHARACTERS,runtime:DEFAULT_RUNTIME,chats:Object.fromEntries(SEED_CHARACTERS.map(c=>[c.id,C.seedMessages(c)])),personas:Object.fromEntries(SEED_CHARACTERS.map(c=>[c.id,C.seedPersonas(c)])),about:C.ABOUT_SEED,people:C.PEOPLE_SEED,feed:C.FEED_SEED,notices:C.COMPANION_NOTICE_SEED,settings:L.SETTINGS_SEED,goals:L.GOAL_SEED,diaries:L.DIARY_SEED,memories:L.MEMORY_SEED,reflections:L.REFLECTION_SEED,claims:L.CLAIM_SEED,tasks:L.TASK_SEED,schedules:Object.fromEntries(SEED_CHARACTERS.map(c=>[c.id,L.seedSchedule(c)])),worlds:R.WORLD_SEEDS,resources:R.RESOURCE_SEEDS,garments:R.GARMENT_SEEDS,outfits:R.OUTFIT_SEEDS,media:Object.fromEntries(SEED_CHARACTERS.map(c=>[c.id,R.mediaSeeds(c)])),svg:{portrait:renderToStaticMarkup(React.createElement(CharacterArt)),...Object.fromEntries(['city','forest','space'].map(kind=>[kind,renderToStaticMarkup(React.createElement(WorldArt,{kind}))])),garments:Object.fromEntries(R.GARMENT_SEEDS.map(g=>[g.id,renderToStaticMarkup(React.createElement(R.GarmentArt,{type:g.type,color:g.color,name:g.name}))]))}};`,resolveDir:demo,sourcefile:'test-only-extraction.tsx',loader:'tsx'},bundle:true,nodePaths:[fileURLToPath(new URL('../../node_modules',import.meta.url))],platform:'node',format:'cjs',outfile:join(directory,'scene.cjs'),alias:{'@':join(demo,'src')},loader:{'.css':'empty'},plugins:[{name:'read-only-seed-exports',setup(b){b.onLoad({filter:/\.tsx?$/},async args=>{if(!args.path.startsWith(join(demo,'src')))return;const source=await readFile(args.path,'utf8');hashes[args.path.slice(demo.length+1)]=createHash('sha256').update(source).digest('hex');const names=exposed[args.path.split('/').at(-1)]||[],ast=ts.createSourceFile(args.path,source,ts.ScriptTarget.Latest,true);const existing=new Set();for(const node of ast.statements){if(!node.modifiers?.some(m=>m.kind===ts.SyntaxKind.ExportKeyword))continue;if(node.name)existing.add(node.name.text);if(node.declarationList)for(const d of node.declarationList.declarations)existing.add(d.name.getText(ast));}const extra=names.filter(n=>!existing.has(n));return {contents:source+(extra.length?'\nexport {'+extra.join(',')+'};':''),loader:args.path.endsWith('tsx')?'tsx':'ts',resolveDir:resolve(args.path,'..')};});}}]});
const {data}=require(join(directory,'scene.cjs'));
data.svg.avatar=`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">${data.svg.portrait.replace('<svg ','<svg x="-38%" y="-35%" width="180%" height="215%" ')}</svg>`;
const sceneCSS=await readFile(join(demo,'src/pages/companion.css'),'utf8');
data.svg.bookshop=`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 225 150"><foreignObject width="225" height="150"><div xmlns="http://www.w3.org/1999/xhtml" style="font-family:Inter,-apple-system,BlinkMacSystemFont,Segoe UI,PingFang SC,Microsoft YaHei,sans-serif"><style>*{box-sizing:border-box}body{margin:0}${sceneCSS}</style>${data.smallArt}</div></foreignObject></svg>`;
for(const [kind,markup] of Object.entries(data.smallArts))data.svg['scene-'+kind]=`<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%"><foreignObject width="100%" height="100%"><div xmlns="http://www.w3.org/1999/xhtml" style="font-family:Inter,-apple-system,BlinkMacSystemFont,Segoe UI,PingFang SC,Microsoft YaHei,sans-serif"><style>*{box-sizing:border-box}body{margin:0}${sceneCSS}</style><div class="companion-feed-image">${markup}</div></div></foreignObject></svg>`;
delete data.smallArt;delete data.smallArts;
await mkdir(resolve(output,'..'),{recursive:true});
await writeFile(output,JSON.stringify({source:{root:demo,hashes},data},null,2)+'\n');
console.log(`Extracted ${data.characters.length} characters, ${data.resources.length} resources and exact SVG artwork into ${output}`);
