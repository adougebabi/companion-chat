<script setup>
import {onMounted,onBeforeUnmount,ref,watch,nextTick} from 'vue';
import {useWorkspace} from '../workspace.mjs';
import {mountMediaUpload} from '../../v3/proto/components/media-upload.js';
import {errorMessage} from '../../v3/domain-presentation.mjs';
const w=useWorkspace(),rows=ref([]),cursor=ref(''),query=ref(''),kind=ref(''),fid=ref(''),origin=ref('all'),hidden=ref(false),busy=ref(false),error=ref(''),uploadHost=ref(null),uploadOpen=ref(false);
const owner=w.state.session.actorId;let active=true,epoch=0,upload;
const live=()=>active&&w.state.session?.actorId===owner;
async function load(more=false){const token=++epoch;busy.value=true;error.value='';try{const page=await w.services.mediaClient.list({fluctlightId:fid.value||undefined,scope:origin.value,kind:kind.value||undefined,q:query.value||undefined,hidden:hidden.value,cursor:more?cursor.value:undefined});if(!live()||token!==epoch)return;rows.value=more?[...rows.value,...page.items]:page.items;cursor.value=page.next_cursor||'';}catch(e){if(live()&&token===epoch)error.value=errorMessage(e);}finally{if(live()&&token===epoch)busy.value=false;}}
const open=asset=>w.navigatePath('/media/'+encodeURIComponent(asset.id));
const url=id=>w.services.mediaClient.url(id);
const source=asset=>({owner_upload:'我上传的素材',studio:'创作素材',fluctlight:'角色生成'})[asset.origin]||'已登记素材';
watch(uploadHost,host=>{upload?.dispose();if(host)upload=mountMediaUpload(host,{onSaved:()=>load()});},{flush:'post'});
onMounted(()=>{fid.value=w.state.route?.query.get('fid')||'';void load();});onBeforeUnmount(()=>{active=false;epoch++;upload?.dispose();});
</script>
<template><section class="media-library stack">
  <div class="row"><p class="muted">上传与角色生成的素材都在这里。画面和素材来源分别保留，不作为已发生的生活事实。</p><span class="spacer"/><button class="button primary" :aria-expanded="uploadOpen" @click="uploadOpen=!uploadOpen">{{uploadOpen?'收起上传':'上传素材'}}</button></div>
  <section v-if="uploadOpen" ref="uploadHost" class="approved-domain" aria-label="上传素材"/>
  <form class="domain-filter row" role="search" @submit.prevent="load()"><label>搜索素材<input v-model="query" placeholder="素材名称"/></label><label>角色<select v-model="fid"><option value="">全部角色</option><option v-for="character in w.state.characters" :key="character.id" :value="character.id">{{character.name}}</option></select></label><label>类型<select v-model="kind"><option value="">全部类型</option><option value="image">图片</option><option value="video">视频</option><option value="audio">音频</option></select></label><label>来源<select v-model="origin"><option value="all">全部来源</option><option value="owner_upload">我的上传</option><option value="studio">创作素材</option><option value="fluctlight">角色生成</option></select></label><label class="check-field"><input v-model="hidden" type="checkbox"/>只看已隐藏</label><button class="button" :disabled="busy">查询</button></form>
  <p v-if="error" class="inline-error" role="alert">{{error}}</p><p class="muted">已读取 {{rows.length}} 项{{cursor?'，还有更多':''}}</p>
  <div class="media-grid"><article v-for="asset in rows" :key="asset.id" class="card media-card"><button class="media-open" :aria-label="'查看'+(asset.title||'未命名素材')" @click="open(asset)"><div class="media-preview"><img v-if="asset.kind==='image'&&asset.readable!==false" :src="url(asset.id)" :alt="asset.title||'图片素材'" class="approved-media-content" loading="lazy"/><video v-else-if="asset.kind==='video'" :src="url(asset.id)" class="approved-media-content" preload="metadata"/><span v-else>{{asset.kind==='audio'?'音频素材':'素材暂不可预览'}}</span></div><b>{{asset.title||asset.source_filename||'未命名素材'}}</b><small>{{source(asset)}}</small></button><audio v-if="asset.kind==='audio'" :src="url(asset.id)" controls preload="metadata" :aria-label="asset.title||'音频素材'"/></article></div>
  <p v-if="busy" role="status">正在读取素材…</p><p v-else-if="!rows.length" class="empty-note">还没有符合条件的素材，可以上传图片、音频或视频。</p><button v-if="cursor" class="button" :disabled="busy" @click="load(true)">加载更多素材</button>
</section></template>
