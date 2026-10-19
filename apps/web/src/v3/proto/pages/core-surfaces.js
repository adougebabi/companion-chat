import * as store from '../store.js';
import { mountCoreSurface, mountAuthentication } from '../../component-host';

export const authenticationSurface = (host, ctx) => mountAuthentication(host, { navigate: ctx.navigate, refresh: store.hydrate });

export function coreSurface(view, extra = {}) {
  return (host, ctx) => {
    const actorId = store.get('session')?.actorId;
    if (!actorId) { host.innerHTML = '<div class="statebox"><a class="btn primary" href="#/auth">登录后继续</a></div>'; return; }
    return mountCoreSurface(host, { view, actorId, fluctlightId: ctx.params.charId,
      section: ctx.query.section, navigate: ctx.navigate, refresh: store.hydrate, ...extra });
  };
}
