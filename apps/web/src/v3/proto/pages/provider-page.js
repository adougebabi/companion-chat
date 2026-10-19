// Lazy page modules keep registry discovery side-effect free and dispose late reads.
export function providerPage(load, name, el, ctx) {
  let alive=true, dispose;
  void load().then(module=>{if(alive)dispose=module[name](el,ctx);}).catch(error=>{if(alive)el.textContent=error.message;});
  return()=>{alive=false;if(typeof dispose==='function')dispose();};
}
