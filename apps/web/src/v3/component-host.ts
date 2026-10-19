import { createApp, h, onMounted, onBeforeUnmount, ref, type Component } from 'vue';
import ActorContextEditor from '../components/instances/ActorContextEditor.vue';
import ChatView from '../views/ChatView.vue';
import { useConversationStore } from '../stores/conversations';
import { workspacePinia } from '../workspace-state';
import { useControlCenterStore } from '../stores/control-center';
import SettingsView from '../views/SettingsView.vue';
import DiagnosticsView from '../views/DiagnosticsView.vue';
import InstancesView from '../views/InstancesView.vue';
import LifeNowView from '../views/life/LifeNowView.vue';
import LifeGoalsView from '../views/life/LifeGoalsView.vue';
import LifeScheduleView from '../views/life/LifeScheduleView.vue';
import LifeWardrobeView from '../views/life/LifeWardrobeView.vue';
import ProfileView from '../views/life/ProfileView.vue';
import GrowthView from '../views/life/GrowthView.vue';
import PeopleView from '../views/life/PeopleView.vue';
import WorldWorkspace from '../views/world/WorldWorkspace.vue';
import AuthPanel from '../components/auth/AuthPanel.vue';

export function mountActorBackground(host: HTMLElement, props: {
  fluctlightId: string;
  ownerActorId: string;
  preferredActorId: string;
  actors: { id: string; label: string }[];
  onSaved: (fluctlightId: string, actorId: string) => void;
}): () => void {
  const app = createApp({ render: () => h(ActorContextEditor, { ...props, nativeControls: true, humanOnly: true }) });
  host.classList.add('v3-component', 'card');
  app.mount(host);
  return () => app.unmount();
}

export function mountConversation(host: HTMLElement, props: {
  fluctlightId: string;
  actorId: string;
  navigate: (path: string) => void;
}): () => void {
  const store = useConversationStore(workspacePinia);
  const app = createApp({ setup() {
    const ready = ref(false), failure = ref('');
    let disposed = false;
    onMounted(async () => {
      try {
        if (store.authenticatedActorId !== props.actorId) await store.initialize();
        if (disposed) return;
        if (!store.authenticated || store.authenticatedActorId !== props.actorId) throw new Error(store.authError || '交流身份未就绪');
        await store.selectFluctlight(props.fluctlightId);
        if (disposed) return;
        if (store.fluctlightId !== props.fluctlightId) throw new Error(store.error || '无法读取这个角色的会话');
        ready.value = true;
      } catch (error) { if (!disposed) failure.value = error instanceof Error ? error.message : '会话加载失败'; }
    });
    onBeforeUnmount(() => { disposed = true; if (store.fluctlightId === props.fluctlightId) store.invalidateRequest(); });
    return () => ready.value ? h(ChatView, {
      onOpenInstances: () => props.navigate('#/library'),
      onOpenDetails: () => props.navigate(`#/c/${props.fluctlightId}/profile`),
      onOpenSection: (view: string) => props.navigate(`#/c/${props.fluctlightId}/${view === 'life' ? 'life/now' : view}`),
    }) : h('div', { class: failure.value ? 'banner err' : 'card', role: failure.value ? 'alert' : 'status' }, failure.value || '正在读取真实会话…');
  } });
  app.use(workspacePinia);
  host.classList.add('v3-component', 'v3-chat');
  app.mount(host);
  return () => app.unmount();
}

export function currentSenderActorId(fluctlightId: string, ownerActorId: string): string {
  const store = useConversationStore(workspacePinia);
  return store.authenticatedActorId === ownerActorId && store.fluctlightId === fluctlightId ? store.senderActorId || ownerActorId : ownerActorId;
}

export function clearProductSession(): void {
  const store = useConversationStore(workspacePinia);
  store.clearActiveConversation();
  store.authenticated = false;
  store.authenticatedActorId = null;
  store.fluctlights = [];
  store.retryTurn = null;
  store.queuedTurn = null;
  const control = useControlCenterStore(workspacePinia);
  control.$reset();
}

export function mountAuthentication(host: HTMLElement, options: { navigate: (path: string) => void; refresh: () => Promise<void> }): () => void {
  const store = useConversationStore(workspacePinia);
  const app = createApp({ setup() {
    onMounted(() => { void store.initialize(); });
    async function authenticate(token: string | null, password: string) {
      if (token !== null) await store.setup(token, password); else await store.login(password);
      if (store.authenticated) { await options.refresh(); options.navigate('#/library'); }
    }
    return () => store.authenticated
      ? h('div', { class: 'card stack' }, [h('p', '已登录所有者会话。'), h('a', { class: 'btn primary', href: '#/library' }, '进入角色库'), h('button', { class: 'btn', onClick: async () => { await store.logout(); clearProductSession(); await options.refresh(); } }, '退出')])
      : h(AuthPanel, { setupAvailable: store.setupAvailable, loading: store.authLoading, error: store.authError, onSignIn: (password: string) => void authenticate(null, password), onSetup: (token: string, password: string) => void authenticate(token, password) });
  } });
  app.use(workspacePinia); host.classList.add('v3-component'); app.mount(host); return () => app.unmount();
}

const coreViews: Record<string, Component> = {
  settings: SettingsView, diagnostics: DiagnosticsView, library: InstancesView,
  now: LifeNowView, goals: LifeGoalsView, schedule: LifeScheduleView,
  wardrobe: LifeWardrobeView, profile: ProfileView, growth: GrowthView, people: PeopleView,
};

export function mountWorldWorkspace(host: HTMLElement, props: {
  fluctlightId: string; view: string; npcId?: string; placeId?: string;
  onNavigate: (path: string) => void;
}): () => void {
  const app = createApp({ render: () => h(WorldWorkspace, props) });
  app.use(workspacePinia); host.classList.add('v3-component', 'v3-core-surface');
  app.mount(host); return () => app.unmount();
}

export function mountCoreSurface(host: HTMLElement, options: {
  view: string; fluctlightId?: string; actorId: string; section?: string;
  create?: boolean; navigate: (path: string) => void; refresh: () => Promise<void>;
}): () => void {
  const store = useConversationStore(workspacePinia), control = useControlCenterStore(workspacePinia);
  const app = createApp({ setup() {
    const ready = ref(false), failure = ref(''); let disposed = false;
    onMounted(async () => {
      try {
        if (store.authenticatedActorId !== options.actorId) await store.initialize();
        if (disposed) return;
        if (options.fluctlightId && store.fluctlightId !== options.fluctlightId) await store.selectFluctlight(options.fluctlightId);
        if (disposed) return;
        if (options.fluctlightId && store.fluctlightId !== options.fluctlightId) throw new Error(store.error || '当前角色不可用');
        if (options.view === 'library') await control.loadActorGroups();
        if (!disposed) ready.value = true;
      } catch (error) { if (!disposed) failure.value = error instanceof Error ? error.message : '读取失败'; }
    });
    onBeforeUnmount(() => { disposed = true; });
    async function openCharacter(suffix: string) {
      const id = store.fluctlightId;
      if (!id) return;
      await options.refresh(); options.navigate(`#/c/${id}/${suffix}`);
    }
    return () => ready.value ? h(coreViews[options.view], {
      section: options.section || null, openCreate: options.create ? 1 : 0,
      onOpenChat: () => void openCharacter('chat'), onOpenDetails: () => void openCharacter('profile'),
      onOpenMedia: () => void openCharacter('media'), onOpenAutonomy: () => void openCharacter('settings/autonomy'),
      onOpen: (view: string, section?: string) => options.navigate(`#/c/${store.fluctlightId}/${view === 'life' ? `life/${section || 'now'}` : view}`),
      onNavigateSection: (section: string) => options.navigate(`#/${options.view === 'diagnostics' ? 'diag' : 'settings'}${section ? `?section=${encodeURIComponent(section)}` : ''}`),
      onOpenDiagnostics: (correlation: string) => options.navigate(`#/diag?section=model-runs&correlation_id=${encodeURIComponent(correlation)}`),
      onLogout: () => { void store.logout().then(async () => { clearProductSession(); await options.refresh(); options.navigate('#/auth'); }); },
    }) : h('div', { class: failure.value ? 'banner err' : 'card', role: failure.value ? 'alert' : 'status' }, failure.value || '正在读取…');
  } });
  app.use(workspacePinia); host.classList.add('v3-component', 'v3-core-surface');
  app.mount(host); return () => app.unmount();
}
