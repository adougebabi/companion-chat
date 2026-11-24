// Fluctlight UI Lab — dev/test only entry. It mounts real product components
// (not copies) with deterministic fixtures so Playwright Chromium can verify
// layout and interaction. It is never part of the production build.
import { createRoot } from 'react-dom/client';
import { useMemo, useState, type ComponentType } from 'react';
import { proxy, snapshot, subscribe } from 'valtio/vanilla';
import '@/styles/react.css';
import { WorkspaceContext } from '@/react-app/page';
import DomainPage from '@/react-app/pages/DomainPage';
import OverlayHost from '@/react-app/shared/OverlayHost';
import * as store from '@/v3/proto/store.js';
import { renderRealSchedule } from '@/v3-domain/life-schedule.js';
import { installScheduleFixture, events } from './fixtures/schedule-settings';

const params = new URLSearchParams(location.search);
const fixture = params.get('fixture') || 'schedule-settings';
const variant = (params.get('variant') || 'default') as 'default' | 'long' | 'empty';

(window as any).__uiLabEvents = events;

function ScheduleSettingsFixture() {
  const [path, setPath] = useState(() => '/c/A/life/schedule?' + new URLSearchParams({ ...(params.get('section') ? { section: params.get('section')! } : {}), ...(params.get('date') ? { date: params.get('date')! } : {}) }));
  const workspace = useMemo(() => {
    const state = proxy<any>({ session: { actorId: 'ui-lab-owner' }, selectedId: 'A', pageHeading: null });
    store.replaceSnapshot({ ...store.get(), session: { actorId: 'ui-lab-owner' } });
    return { state, subscribe: (fn: () => void) => subscribe(state, fn), getSnapshot: () => snapshot(state), navigatePath: (next: string) => { events.push({ action: 'navigate', args: [next] }); setPath(next.replace(/^\/?#?/, '/').replace(/^\/+/, '/')); } };
  }, []);
  const route = useMemo(() => {
    const [routePath, query = ''] = path.split('?');
    return { page: '日程', fid: null, path: routePath, query: new URLSearchParams(query), feature: { title: '安排与调整日程', params: {}, load: async () => renderRealSchedule } };
  }, [path]);
  return (
    <WorkspaceContext.Provider value={workspace}>
      <div className="react-workspace replica-workspace">
        <main className="workspace-page replica-page" data-ui-lab-page>
          <DomainPage route={route} />
        </main>
        <OverlayHost />
      </div>
    </WorkspaceContext.Provider>
  );
}

const fixtures: Record<string, ComponentType> = { 'schedule-settings': ScheduleSettingsFixture };
if (fixture === 'schedule-settings') installScheduleFixture(variant);
const Fixture = fixtures[fixture];
const host = document.getElementById('app')!;
createRoot(host).render(Fixture ? <Fixture /> : <p role="alert">未知 fixture：{fixture}</p>);
