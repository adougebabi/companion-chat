// Project only explicit, unexpired place identities; text and home names are not locations.
export function residentsByPlace(snapshot, binding, character, now = Date.now()) {
  const worldId = snapshot?.world?.id, places = new Set((snapshot?.places || []).filter(p => p.status === 'active').map(p => p.id));
  const result = new Map();
  const add = (place, resident) => {
    if (!places.has(place)) return;
    if (!result.has(place)) result.set(place, []);
    result.get(place).push(resident);
  };
  const validUntil = value => !value || (Number.isFinite(Date.parse(value)) && Date.parse(value) > now);
  const context = snapshot?.current_context;
  if (worldId && binding?.world_id === worldId && context?.world_id === worldId && validUntil(context.expires_at)) {
    const label = context.source === 'schedule' ? '按日程' : context.source === 'event' && context.authority_status === 'confirmed' ? '当前记录' : context.source === 'event' ? '推测位置' : '';
    if (label) add(context.place_id, {id: character.id, name: character.name, label, fluctlight: true});
  }
  for (const npc of snapshot?.npcs || []) {
    if (npc.world_id !== worldId || npc.status !== 'active' || npc.enabled === false) continue;
    const actual = npc.current_presence;
    const actualValid = actual?.status === 'active' && Number.isFinite(Date.parse(actual.start_at)) && Date.parse(actual.start_at) <= now && Number.isFinite(Date.parse(actual.end_at)) && validUntil(actual.end_at);
    const plan = npc.planned_position;
    const planValid = plan && (!plan.window_start || Date.parse(plan.window_start) <= now) && validUntil(plan.window_end);
    if (actualValid) add(actual.place_id, {id: npc.id, name: npc.name, label: '当前记录'});
    else if (planValid) add(plan.place_id, {id: npc.id, name: npc.name, label: '预计位置'});
  }
  return result;
}
