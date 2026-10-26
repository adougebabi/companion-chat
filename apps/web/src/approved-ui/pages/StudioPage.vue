<script setup>
import {computed,onMounted,onBeforeUnmount,ref,watch} from 'vue';
import {useWorkspace} from '../workspace.mjs';
import {persistentCommand} from '../../v3/providers/persistent-command.mjs';
import {errorMessage} from '../../v3/domain-presentation.mjs';
const props=defineProps({kind:{type:String,default:''}}),w=useWorkspace();
const rows=ref([]),cursor=ref(''),query=ref(''),kind=ref(props.kind),archived=ref(false),busy=ref(false),error=ref(''),selected=ref(null),reason=ref(''),pending=ref(null);
const kinds={world:'世界',place:'地点',item:'物品',outfit:'服装方案',npc:'人物',event_template:'事件',character_template:'角色模板',persona_fragment:'人格资料',visual_style:'视觉参考'};
const owner=w.state.session.actorId;let alive=true,epoch=0;
const live=()=>alive&&w.state.session?.actorId===owner;
const command=persistentCommand({key:`approved-studio:${owner}:directory-command`,storage:()=>localStorage,execute:(value,idempotencyKey)=>value.operation==='restore'?w.client.restoreStudioResource(value.id,{expectedRevision:value.revision,reason:value.reason,idempotencyKey}):w.client.archiveStudioResource(value.id,{expectedRevision:value.revision,reason:value.reason,idempotencyKey})});
async function load(more=false){const token=++epoch;busy.value=true;error.value='';try{const page=await w.client.listStudioResources({kind:kind.value||undefined,q:query.value||undefined,archived:archived.value,cursor:more?cursor.value:undefined});if(!live()||token!==epoch)return;rows.value=more?[...rows.value,...page.items]:page.items;cursor.value=page.next_cursor||'';}catch(e){if(live()&&token===epoch)error.value=errorMessage(e);}finally{if(live()&&token===epoch)busy.value=false;}}
const create=type=>w.navigatePath('/studio/edit/new?kind='+encodeURIComponent(type||kind.value||'world'));
const edit=row=>w.navigatePath((row.kind==='npc'?'/studio/npc/':'/studio/edit/')+encodeURIComponent(row.id));
async function submit(retry=false){if(busy.value||!retry&&!reason.value.trim())return;busy.value=true;error.value='';try{if(retry)await command.retry();else await command.submit({operation:selected.value.status==='archived'?'restore':'archive',id:selected.value.id,revision:selected.value.revision,reason:reason.value.trim()});if(live()){selected.value=null;reason.value='';await load();}}catch(e){if(live())error.value=errorMessage(e);}finally{if(live()){busy.value=false;pending.value=command.pending();}}}
function manage(row){selected.value=row;reason.value='';}
onMounted(()=>{if(!props.kind)kind.value=w.state.route?.query.get('kind')||'';pending.value=command.pending();void load();});
watch(()=>props.kind,value=>{kind.value=value;void load();});
onBeforeUnmount(()=>{alive=false;epoch++;});
</script>
<template><section class="studio-directory stack">
  <div class="row"><p class="muted">{{props.kind==='world'?'先准备世界的背景和地点，再让新角色沿用这些设定开始生活。':'在这里准备世界、地点、物品与人物资料，保存后可继续完善，再选择用于哪个角色。'}}</p><span class="spacer"/><button class="button primary" @click="create()">新建{{kinds[kind]||'资源'}}</button></div>
  <form class="domain-filter row" role="search" @submit.prevent="load()"><label>搜索资源<input v-model="query" placeholder="资源名称"/></label><label v-if="!props.kind">类型<select v-model="kind"><option value="">全部类型</option><option v-for="(label,value) in kinds" :key="value" :value="value">{{label}}</option></select></label><label class="check-field"><input v-model="archived" type="checkbox"/>查看已归档资源</label><button class="button" :disabled="busy">查询</button></form>
  <div v-if="!props.kind" class="row"><button v-for="type in ['world','place','item','character_template']" :key="type" class="button" @click="create(type)">新建{{kinds[type]}}</button></div>
  <p v-if="error" class="inline-error" role="alert">{{error}}</p><aside v-if="pending" class="card"><p>上次操作结果尚未确认，原内容已保留。</p><button class="button" :disabled="busy" @click="submit(true)">重试原操作</button></aside>
  <div class="resource-grid"><article v-for="row in rows" :key="row.id" class="card resource-card"><div class="row"><h2>{{row.name}}</h2><span class="spacer"/><details class="actions-menu"><summary :aria-label="row.name+'的操作'">⋯</summary><div><button class="text-button" @click="edit(row)">编辑资源</button><button class="text-button" :disabled="!!pending" @click="manage(row)">{{row.status==='archived'?'恢复资源':'归档资源'}}</button></div></details></div><p class="muted">{{kinds[row.kind]||'创作资源'}} · {{row.status==='archived'?'已归档':row.published_version?'已有可用版本':'草稿'}}</p><button class="button" @click="edit(row)">查看与编辑</button></article></div>
  <p v-if="!rows.length&&!busy" class="empty-note">{{archived?'没有已归档的资源。':'还没有资源，可以先创建一份世界设定。'}}</p><p v-if="busy" role="status">正在读取资源…</p><button v-if="cursor" class="button" :disabled="busy" @click="load(true)">加载更多资源</button>
  <div v-if="selected" class="modal-backdrop" @click.self="selected=null"><form class="card domain-dialog stack" role="dialog" aria-modal="true" :aria-label="selected.status==='archived'?'恢复资源':'归档资源'" @submit.prevent="submit()"><h2>{{selected.status==='archived'?'恢复':'归档'}}「{{selected.name}}」</h2><p>归档会保留已保存版本和来源；仍被角色或其他定义引用时，服务端会提示先处理引用。恢复后不会自动应用。</p><label>操作原因<textarea v-model="reason" required maxlength="500"/></label><div class="row"><button class="button" type="button" :disabled="busy" @click="selected=null">取消</button><button class="button primary" :disabled="busy||!!pending">确认{{selected.status==='archived'?'恢复':'归档'}}</button></div></form></div>
</section></template>
