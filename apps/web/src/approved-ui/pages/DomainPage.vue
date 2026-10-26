<script setup>
import {ref,watch,onBeforeUnmount} from 'vue';
import {useWorkspace} from '../workspace.mjs';
import {approvedHref} from '../routing.mjs';
import {errorMessage} from '../../v3/domain-presentation.mjs';
import {closeOverlays} from '../../v3/proto/components/overlay.js';
import {installFieldValidation} from '../../v3/proto/components/forms.js';
import {refreshProduct} from '../../v3/providers/real.ts';
const props=defineProps({route:{type:Object,required:true}});
const w=useWorkspace(),host=ref(null),failure=ref('');
let generation=0,dispose,validation;
async function open(){
  if(!host.value)return;
  const token=++generation,owner=w.state.session?.actorId;
  dispose?.();validation?.();closeOverlays();dispose=null;failure.value='';
  const route=props.route,feature=route.feature;
  host.value.replaceChildren();
  const target=document.createElement('div');host.value.append(target);
  try{
    const render=await feature.load();
    if(route.fid)await refreshProduct();
    if(token!==generation||owner!==w.state.session?.actorId)return;
    const params={...feature.params};
    const context={charId:route.fid||w.state.selectedId,params,query:Object.fromEntries(route.query),path:route.path,raw:route.path+(route.query.size?'?'+route.query:''),
      def:{title:feature.title,mockSection:params.section},navigate:path=>w.navigatePath(String(path).replace(/^#/,'')),refresh:open};
    let cleanup;
    if(feature.render==='renderRealGovernance'&&context.query.tab==='developing'){
      const self=await import('../../v3-domain/self.js');cleanup=self.renderRealSelfGovernance(target,context);
    }else cleanup=await render(target,context);
    if(token!==generation||owner!==w.state.session?.actorId){cleanup?.();return;}
    dispose=cleanup;validation=installFieldValidation(target);
  }catch(cause){if(token===generation)failure.value=errorMessage(cause,'这个功能暂时无法读取，请重试。');}
}
watch(()=>[host.value,props.route],open,{flush:'post'});
onBeforeUnmount(()=>{generation++;dispose?.();validation?.();closeOverlays();});
</script>
<template>
  <section class="approved-domain" data-testid="domain-workspace">
    <header class="domain-heading"><button class="button" @click="w.navigatePath(approvedHref(route.page,route.fid).slice(1))">返回{{route.page}}</button><h2>{{route.feature.title}}</h2></header>
    <div v-if="failure" class="inline-error" role="alert">{{failure}} <button class="button" @click="open">重新读取</button></div>
    <div ref="host" class="domain-content"/>
  </section>
</template>
