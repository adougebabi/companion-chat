import { mountWorldWorkspace } from '../../component-host';
export const worldSurface = view => (host, ctx) => mountWorldWorkspace(host, {
  fluctlightId: ctx.charId, view, npcId: ctx.params.npcId, placeId: ctx.params.placeId, onNavigate: ctx.navigate,
});
