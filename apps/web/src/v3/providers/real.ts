import { BrowserClient, type BrowserActorContextUpdate, type BrowserFluctlightCreationAnalysisRequest, type BrowserFluctlightCreationAnalysis, type BrowserFluctlightActivationRequest, type BrowserPersonaSwitchRequest, type BrowserPersonaSwitchTerminal, type MessageViewFields } from '@fluctlight/browser-client';
import { useConversationStore } from '../../stores/conversations';
import { workspacePinia } from '../../workspace-state';
import { installWorldProvider } from './world';
import { installLifeProvider } from './life';
import { installMemoryProvider } from './memory';
import { installReflectionProvider } from './reflection';
import { installOperationsProvider } from './operations';
import { installAutonomyProvider } from './autonomy';
import { installDiaryProvider } from './diary';
import {createGovernanceProvider,installGovernanceProvider} from './governance';
import {installVisualProvider} from './visual';
import {installInventoryProvider} from './inventory';
import { installSelfProvider } from './self';
import {installStudioProvider} from './studio';
import {installMediaProvider} from './media';
import {installMomentsProvider} from './moments';
import {installConfigurationProvider} from './configuration';
import {installDataToolsProvider} from './data-tools';
import {installDiagnosticsProvider} from './diagnostics';
// The JS renderers consume one centralized projection, never wire DTO copies.
// @ts-expect-error JS projection
import { viewFromBundle, directoryGroupProjection } from '../contract-map.mjs';
// @ts-expect-error JS reader
import { readProduct } from '../read-product.mjs';
// @ts-expect-error shared optional progress reader
import {readInitializationProgress} from '../initialization-progress.mjs';
// @ts-expect-error JS facade
import { replaceSnapshot, get, configureReadProjection } from '../proto/store.js';
// @ts-expect-error JS projection
import { profileSnapshotFromDetail } from '../proto/data/profile.js';
export const client = new BrowserClient(import.meta.env.VITE_API_ORIGIN || '');
const governance = createGovernanceProvider(client,refreshProduct);
export const conversation = () => useConversationStore(workspacePinia);
let refreshEpoch = 0;
let latestRefresh: Promise<unknown> = Promise.resolve();
export function refreshProduct(options:{initialOnly?:boolean}={}) {
  const epoch = ++refreshEpoch;
  const task = readProduct(client,options).then((bundle:any) => {
    if (epoch !== refreshEpoch) return latestRefresh;
    const snapshot = viewFromBundle(bundle);
    snapshot.session = bundle.session.authenticated ? bundle.session : null;
    snapshot.meta.surfaces = bundle.surfaces || {};
    snapshot.meta.details = bundle.details || {};
    replaceSnapshot(snapshot);
    return bundle.session;
  });
  latestRefresh=task;return task;
}
function refreshInBackground(){void refreshProduct().catch((error:unknown)=>{const snapshot=get();replaceSnapshot({...snapshot,meta:{...snapshot.meta,bootstrapWarning:error instanceof Error?error.message:'部分资料读取尚未完成，请重试'}});});}

export const auth = {
  session: () => client.session(), setupStatus: () => client.setupStatus(),
  async login(password: string) { await client.login(password); await refreshProduct({initialOnly:true}); refreshInBackground(); },
  async setup(token: string, password: string) { await client.setup(token,password); await refreshProduct({initialOnly:true}); refreshInBackground(); },
  async logout() { await conversation().logout(); await refreshProduct(); },
};
export const directory = {
  groups: () => client.listActorGroups(),
  async move(characterId: string, groupId: string) {
    const owner=get('session')?.actorId;
    const desired = groupId === 'ungrouped' ? '' : groupId;
    const groups = await client.listActorGroups();
    const memberships = groups.filter(g=>(g.actor_ids ?? g.members ?? []).includes(characterId));
    if(desired && !groups.some(g=>g.id===desired))throw new Error('所选分组已不存在，请重新读取');
    // Add first to retain a valid group if a later removal fails. Every failure
    // stays explicit; retry reads actual memberships and resumes idempotently.
    try {
      if(desired)await client.assignActorGroupMember(desired,characterId);
      for(const group of memberships)if(group.id!==desired)await client.removeActorGroupMember(group.id,characterId);
      const confirmed=await client.listActorGroups();
      const actual=confirmed.filter(g=>(g.actor_ids??g.members??[]).includes(characterId)).map(g=>g.id);
      if(actual.length!==(desired?1:0)||(desired&&actual[0]!==desired))throw new Error('分组命令后读回不一致，请重新读取并核对原选择');
      if(get('session')?.actorId===owner){const snapshot=get();replaceSnapshot({...snapshot,...directoryGroupProjection(snapshot.characters,confirmed)});refreshInBackground();}
    } catch(error) { refreshInBackground(); throw error; }
  },
  async createGroup(name: string) { const result = await client.createActorGroup(name); await refreshProduct(); return result; },
  async lifecycle(id: string, status: 'active'|'paused'|'retired', expectedRevision: number, reason: string) {
    if (status === 'retired') await client.retireFluctlight(id,{expectedRevision,reason});
    else await client.setStatus(id,{status,expectedRevision,reason});
    const detail = await client.detail(id);
    if (detail.status !== status) throw new Error('命令已提交，但最新生命周期状态尚未确认一致');
    await refreshProduct();
  },
  async removeGroup(id: string, idempotencyKey: string) { await client.deleteActorGroup(id,{expectedRevision:0,idempotencyKey}); await refreshProduct(); },
  analyze: (input: string | BrowserFluctlightCreationAnalysisRequest, options: {signal?:AbortSignal} = {}) => client.analyzeFluctlightCreation(input,options),
  templates: (options:{q?:string;cursor?:string}={}) => client.listStudioResources({...options,kind:'character_template',published:true}),
  publication: (id:string,version:string) => client.studioPublication(id,version),
  async activate(body: BrowserFluctlightActivationRequest, groupId?: string) {
    const result = await client.activateFluctlightCreation(body);
    let warning='';
    if (groupId && groupId !== 'ungrouped') {
      try { await client.assignActorGroupMember(groupId,result.id); }
      catch(error) { warning=`角色已激活；分组未保存：${error instanceof Error ? error.message : '读取失败'}`; }
    }
    refreshInBackground(); return {...result,warning};
  },
};
export function activationFromAnalysis(analysis: BrowserFluctlightCreationAnalysis, name: string, requestId: string, timezone: string): BrowserFluctlightActivationRequest {
  return { requestId, initializationMode:'llm_defined', initializationTimezone:timezone, analysisId:analysis.analysis_id, schemaVersion:analysis.schema_version, name, corePersona:analysis.core_persona, developingSelf:analysis.developing_self, extensions:analysis.extensions, initialGoals:analysis.initial_goals, initialIntentions:analysis.initial_intentions, initialRelationships:analysis.initial_relationships, ...(analysis.actor_user ? {actorUser:analysis.actor_user} : {}) };
}
export const aboutYou = {
  read: (characterId: string, actorId: string) => client.actorContext(characterId,actorId),
  save: (characterId: string, actorId: string, command: BrowserActorContextUpdate) => client.updateActorContext(characterId,actorId,command),
};
export const profile = {
  progress:(id:string)=>readInitializationProgress(client,id),
  async switchPersona(id: string, command: BrowserPersonaSwitchRequest, options?: { signal?: AbortSignal }) {
    let result = await client.switchPersona(id,command);
    if (result.status === 'completed') return result;
    for (let attempt=0; attempt<90; attempt++) {
      options?.signal?.throwIfAborted();
      await new Promise<void>((resolve,reject) => {
        const done = () => { options?.signal?.removeEventListener('abort',abort); resolve(); };
        const timer = setTimeout(done,2000);
        const abort = () => { clearTimeout(timer); reject(new DOMException('已停止观察；已接受任务继续执行','AbortError')); };
        options?.signal?.addEventListener('abort',abort,{once:true});
      });
      const request = await client.ownerRequest<BrowserPersonaSwitchTerminal>(result.request_id);
      if (request.fluctlight_id !== id || request.kind !== 'persona_switch') throw new Error('人格切换任务范围不匹配');
      if (request.status === 'failed' || request.status === 'rejected') throw new Error(request.result?.error_code || request.decision_reason || '人格切换未获执行');
      if (request.status === 'completed') {
        if (request.result?.status !== 'completed' || request.result.active_profile_id !== command.targetPersonaId) throw new Error('任务已结束，但目标人格尚未确认生效');
        return request.result;
      }
    }
    throw new Error('人格切换仍在处理；请重新读取。已接受任务继续执行。');
  },
  async editBio(id: string, bio: string, expectedRevision?: number | null) {
    const detail = await client.detail(id);
    if (!Number.isSafeInteger(detail.current_revision)) throw new Error('资料版本缺失，请重新读取');
    if (expectedRevision !== detail.current_revision) throw Object.assign(new Error('资料版本已变化，简介草稿已保留，请重新读取后确认'),{status:409});
    const reason='所有者编辑角色简介';
    const identity = detail.identity && typeof detail.identity === 'object' ? detail.identity : {};
    const proposed = await governance.propose(id,{changes:{identity:{...identity,background_story:bio}},expectedRevision:Number(detail.current_revision),reason});
    if (typeof proposed.id !== 'string') throw new Error('修订已提交但未返回可确认的修订标识，请重新读取');
    await client.acceptFoundationRevision(id,proposed.id,{expectedRevision:Number(detail.current_revision),reason});
    const current = await profile.read(id);
    if (current.bio !== bio) throw new Error('修订已提交，但最新简介尚未确认一致');
  },
  async read(id: string) { const detail = await client.detail(id); return profileSnapshotFromDetail({...detail,id},get('characters',id)); },
};
let providersRegistered=false;
export function registerProviders() {
  if(providersRegistered)return;
  providersRegistered=true;
  installWorldProvider({ client, refreshProduct });
  installLifeProvider(client);
  installMemoryProvider(client,async(id,text)=>{const state=await openConversation(id);await state.send(text);if(state.error)throw new Error(state.error);});
  installReflectionProvider(client);
  installOperationsProvider(client,refreshProduct);
  installAutonomyProvider(client);
  installDiaryProvider(client);
  installSelfProvider(client);
  installStudioProvider(client);
  installMediaProvider(client,refreshProduct);
  installMomentsProvider(client);
  installConfigurationProvider(client,refreshProduct);
  installDiagnosticsProvider(client);
 installDataToolsProvider(client,refreshProduct);
  installInventoryProvider(client);
  installVisualProvider(client);
  installGovernanceProvider(governance);
  configureReadProjection('messages',(snapshot: Record<string,unknown>) => {
    const state=conversation();
    if (!state.fluctlightId) return snapshot;
    const inactive=Object.fromEntries(Object.entries(snapshot).filter(([,message]) => (message as {charId:string}).charId !== state.fluctlightId));
    return {...inactive,...Object.fromEntries(state.messages.map(message => [message.id,{...message,charId:state.fluctlightId,from:message.kind === 'user' ? 'user' : message.kind === 'system' ? 'sys' : 'char',at:message.createdAt,refs:message.attachmentRefs,memoryRef:(message as typeof message & MessageViewFields).memory_ref,goalRef:(message as typeof message & MessageViewFields).goal_ref}]))};
  });
}
export async function openConversation(id: string, current:()=>boolean = ()=>true) {
  const store = conversation();
  const session = await client.session();
  if(!current())throw new DOMException('页面已离开','AbortError');
  if (!session.authenticated || !session.actorId) throw new Error('请先登录');
  store.authenticated = true; store.authenticatedActorId = session.actorId;
  const fluctlights = await client.listFluctlights();
  if(!current())throw new DOMException('页面已离开','AbortError');
  store.fluctlights = fluctlights;
  if (!store.fluctlights.some(row => row.id === id)) throw new Error('当前角色不可用');
  await store.selectFluctlight(id,{current});
  if(!current())throw new DOMException('页面已离开','AbortError');
  if (store.fluctlightId !== id || !store.conversation) throw new Error(store.error || '读取会话失败');
  return store;
}
export async function readBackground(characterId: string, actorId: string) {
  const owner = actorId === get('session')?.actorId;
  const [context, detail] = await Promise.all([client.actorContext(characterId,actorId), owner ? client.detail(characterId) : Promise.resolve(null)]);
  if (context.actor_type !== 'human') throw new Error('关于你仅编辑人类交流参与者，不能修改角色或 NPC 设定');
  return { ...context, background: detail?.actor_user?.background ?? context.background, owner, factsRevision: owner ? String(detail?.current_facts_revision ?? '') : null };
}
export async function saveBackground(characterId: string, actorId: string, body: BrowserActorContextUpdate, factsRevision: string | null) {
  if (actorId === get('session')?.actorId) {
    if (!factsRevision) throw new Error('当前事实版本缺失，请重新读取');
    const {expectedContextVersion: _, ...command} = body;
    return client.updateActorUserBackground(characterId,{...command,expectedCurrentFactsRevision:factsRevision});
  }
  return aboutYou.save(characterId,actorId,body);
}
