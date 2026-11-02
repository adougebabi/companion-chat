// Shared ambient executions have no character Life Event. Accept only the
// explicit Core projection for this exact world; unknown/foreign attribution
// must not pass the character-scoped read fence.
export function worldEventInScope(row, fluctlightId, worldId) {
  if (!row?.id || (row.world_id && row.world_id !== worldId)) return false;
  if (row.fluctlight_id === fluctlightId) return true;
  return row.fluctlight_id === null && row.world_id === worldId &&
    row.record_kind === 'world_event_execution' && row.context_scope === 'ambient' &&
    row.source === 'world_execution' && row.status === 'applied' && row.life_write === false;
}
