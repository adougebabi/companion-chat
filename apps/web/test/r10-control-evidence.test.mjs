import test from 'node:test';
import assert from 'node:assert/strict';
import {verifyMeasurements} from '../scripts/verify-r10-controls.mjs';

// Observations from the actual 390x844 pages, including the stale audit tab.
const transparent={content:'""',background:'rgba(0, 0, 0, 0)',width:'44px',height:'44px'};
const hit=Object.fromEntries(['top','bottom','left','right'].map(direction=>[direction,{hit:true}]));
const icon={viewport:'mobile',path:'inbox',name:'重新读取通知',kind:'icon',size:'icon',rect:{width:36,height:36},padding:['0px','0px','0px','0px'],after:transparent,hits:hit};
const focus={viewport:'desktop',path:'library',name:'先连接模型',kind:'focus',focusVisible:true,shadow:'rgb(41, 84, 66) 0px 0px 0px 3px',ringShadow:'0 0 0 3px rgb(41, 84, 66)'};

test('audit rejects the stale tab ring even when real Tab reached the correct asChild link',()=>{
  verifyMeasurements([focus]);
  assert.throws(()=>verifyMeasurements([{...focus,shadow:'none'}]),/ring must survive/);
  assert.throws(()=>verifyMeasurements([{...focus,shadow:'rgba(0, 0, 0, 0.05) 0px 1px 2px 0px'}]),/computed shadow/);
  assert.throws(()=>verifyMeasurements([{...focus,focusVisible:false}]),/real Tab/);
});

test('audit rejects exact legacy page sizes without a tolerance or class-name substitute',()=>{
  verifyMeasurements([icon]);
  for(const[width,height]of[[28,32],[32,32],[36,44],[28,39]])assert.throws(()=>verifyMeasurements([{...icon,rect:{width,height}}]),/visual size/);
});

test('a computed 44px pseudo element does not certify the home filter 10px hit',()=>{
  const filter={viewport:'mobile',path:'library',name:'全部',kind:'filter',rect:{width:46,height:28.5},after:transparent,hits:hit};
  verifyMeasurements([filter]);
  for(const direction of ['top','bottom','left','right'])assert.throws(()=>verifyMeasurements([{...filter,hits:{...hit,[direction]:{hit:false}}}]),new RegExp('10px '+direction));
});

test('reload audit checks actual pointer dispatch at all four outside coordinates',()=>{
  for(const direction of ['top','bottom','left','right']){
    assert.throws(()=>verifyMeasurements([{...icon,hits:{...hit,[direction]:{hit:false}}}]),new RegExp('10px '+direction));
    assert.throws(()=>verifyMeasurements([{...icon,clicks:{top:true,bottom:true,left:true,right:true,[direction]:false}}]),/actual pointer target/);
  }
});

test('radio and checkbox tests reject enlarged visuals and clipped transparent hit planes',()=>{
  for(const kind of ['radio','checkbox']){
    const row={viewport:'mobile',path:'worlds',name:kind,kind,rect:{width:16,height:16},after:transparent,hits:hit};
    verifyMeasurements([row]);
    assert.throws(()=>verifyMeasurements([{...row,rect:{width:44,height:44}}]),/original visual size/);
    assert.throws(()=>verifyMeasurements([{...row,after:{...transparent,height:'30px'}}]),/minimum 44px/);
    assert.throws(()=>verifyMeasurements([{...row,hits:{...hit,top:{hit:false}}}]),/10px top/);
  }
});
