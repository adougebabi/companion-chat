import {reactive, ref} from 'vue';
import {defineStore} from 'pinia';
import {
  commentActivity,
  listActivityPage,
  markActivitiesRead,
  setActivityHidden,
  setActivityLike
} from '../api/activities';
import type {Activity, ActivityPage} from '../types';

const PAGE_SIZE = 20;
export const ALL_ACTIVITIES_KEY = '__all__';
export const HIDDEN_ACTIVITIES_PREFIX = '__hidden__:';

export interface ActivityState {
  items: Activity[];
  nextCursor: string | null;
  hasMore: boolean;
  loading: boolean;
  loadingMore: boolean;
  error: string | null;
  pagesLoaded: number;
}

function createState(): ActivityState {
  return {items: [], nextCursor: null, hasMore: false, loading: false, loadingMore: false, error: null, pagesLoaded: 0};
}

function keyFor(personaId?: string | null): string {
  return personaId || ALL_ACTIVITIES_KEY;
}

function hiddenKeyFor(personaId: string): string {
  return `${HIDDEN_ACTIVITIES_PREFIX}${personaId}`;
}

function merge(current: Activity[], incoming: Activity[]): Activity[] {
  const seen = new Set(current.map(item => item.id));
  return [...current, ...incoming.filter(item => !seen.has(item.id))];
}

export const useActivitiesStore = defineStore('activities', () => {
  const feeds = reactive<Record<string, ActivityState>>({});
  const hiddenFeeds = reactive<Record<string, ActivityState>>({});
  const commentingId = ref<string | null>(null);

  function ensure(personaId?: string | null): ActivityState {
    const key = keyFor(personaId);
    if (!feeds[key]) feeds[key] = createState();
    return feeds[key];
  }

  function get(personaId?: string | null): ActivityState {
    return ensure(personaId);
  }

  function ensureHidden(personaId: string): ActivityState {
    const key = hiddenKeyFor(personaId);
    if (!hiddenFeeds[key]) hiddenFeeds[key] = createState();
    return hiddenFeeds[key];
  }

  function getHidden(personaId?: string | null): ActivityState {
    return personaId ? ensureHidden(personaId) : createState();
  }

  function applyPage(state: ActivityState, page: ActivityPage, append: boolean): void {
    state.items = append ? merge(state.items, page.items) : page.items;
    state.nextCursor = page.nextCursor;
    state.hasMore = page.nextCursor !== null;
    if (!append) state.pagesLoaded = 1;
    else state.pagesLoaded += 1;
  }

  async function loadInitial(personaId?: string | null, signal?: AbortSignal): Promise<ActivityPage> {
    const state = ensure(personaId);
    state.loading = true;
    state.error = null;
    try {
      const page = await listActivityPage({personaId, limit: PAGE_SIZE, signal});
      applyPage(state, page, false);
      return page;
    } catch (caught) {
      if ((caught as Error)?.name !== 'AbortError') state.error = caught instanceof Error ? caught.message : '动态加载失败';
      throw caught;
    } finally {
      state.loading = false;
    }
  }

  async function loadMore(personaId?: string | null, signal?: AbortSignal): Promise<ActivityPage | null> {
    const state = ensure(personaId);
    if (state.loadingMore || !state.hasMore || !state.nextCursor) return null;
    state.loadingMore = true;
    state.error = null;
    try {
      const page = await listActivityPage({personaId, cursor: state.nextCursor, limit: PAGE_SIZE, signal});
      applyPage(state, page, true);
      return page;
    } catch (caught) {
      if ((caught as Error)?.name !== 'AbortError') state.error = caught instanceof Error ? caught.message : '动态加载失败';
      throw caught;
    } finally {
      state.loadingMore = false;
    }
  }

  async function loadHiddenInitial(personaId: string, signal?: AbortSignal): Promise<ActivityPage> {
    const state = ensureHidden(personaId);
    state.loading = true;
    state.error = null;
    try {
      const page = await listActivityPage({personaId, visibility: 'hidden', limit: PAGE_SIZE, signal});
      applyPage(state, page, false);
      return page;
    } catch (caught) {
      if ((caught as Error)?.name !== 'AbortError') state.error = caught instanceof Error ? caught.message : '已隐藏动态加载失败';
      throw caught;
    } finally {
      state.loading = false;
    }
  }

  async function loadHiddenMore(personaId: string, signal?: AbortSignal): Promise<ActivityPage | null> {
    const state = ensureHidden(personaId);
    if (state.loadingMore || !state.hasMore || !state.nextCursor) return null;
    state.loadingMore = true;
    state.error = null;
    try {
      const page = await listActivityPage({personaId, visibility: 'hidden', cursor: state.nextCursor, limit: PAGE_SIZE, signal});
      applyPage(state, page, true);
      return page;
    } catch (caught) {
      if ((caught as Error)?.name !== 'AbortError') state.error = caught instanceof Error ? caught.message : '已隐藏动态加载失败';
      throw caught;
    } finally {
      state.loadingMore = false;
    }
  }

  async function restore(activityId: string, personaId: string): Promise<void> {
    await setActivityHidden(activityId, false);
    const state = ensureHidden(personaId);
    state.items = state.items.filter(activity => activity.id !== activityId);
  }

  async function like(activityId: string, liked: boolean, personaId?: string | null): Promise<void> {
    const response = await setActivityLike(activityId, liked);
    const item = ensure(personaId).items.find(activity => activity.id === activityId);
    if (item) item.liked = response && typeof response.liked === 'boolean' ? response.liked : liked;
  }

  async function hide(activityId: string, hidden: boolean, personaId?: string | null): Promise<void> {
    await setActivityHidden(activityId, hidden);
    if (hidden) {
      const state = ensure(personaId);
      state.items = state.items.filter(activity => activity.id !== activityId);
    }
  }

  async function comment(activityId: string, content: string, personaId?: string | null): Promise<unknown> {
    const response = await commentActivity(activityId, content);
    const item = ensure(personaId).items.find(activity => activity.id === activityId);
    if (item && response && typeof response === 'object') {
      const candidate = response as {comment?: Activity['comments'][number]};
      if (candidate.comment?.id) item.comments = [...item.comments, candidate.comment];
    }
    commentingId.value = null;
    return response;
  }

  function startComment(activityId: string): void {
    commentingId.value = activityId;
  }

  function cancelComment(): void {
    commentingId.value = null;
  }

  async function markRead(): Promise<void> {
    await markActivitiesRead();
  }

  return {feeds, hiddenFeeds, commentingId, ensure, get, getHidden, loadInitial, loadMore, loadHiddenInitial, loadHiddenMore, restore, like, hide, comment, startComment, cancelComment, markRead};
});

export {PAGE_SIZE as ACTIVITY_PAGE_SIZE};
export default useActivitiesStore;
