function objectValue(value) {
  return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
}

function arrayValue(value) {
  return Array.isArray(value) ? value : [];
}

function integerOrNull(value) {
  return Number.isSafeInteger(value) ? value : null;
}

/**
 * Project a character detail into the profile page snapshot.
 * The live persona is personality_runtime.active_profile_id. An initial
 * personality_system id alone is not a current persona.
 * @param {Record<string, any>} [detail]
 * @param {Record<string, any>} [fallback]
 */
export function profileSnapshotFromDetail(detail = {}, fallback = {}) {
  const source = objectValue(detail);
  const extra = objectValue(fallback);
  const identity = objectValue(source.identity);
  const system = objectValue(objectValue(source.core_persona).personality_system);
  const runtime = objectValue(source.personality_runtime);
  const compilation = objectValue(source.persona_compilation);
  const current = objectValue(source.current_state);
  const profiles = arrayValue(system.profiles).filter(item => item && typeof item === 'object').map(item => ({ ...item }));
  const activeProfileId = typeof runtime.active_profile_id === 'string' && runtime.active_profile_id !== '' ? runtime.active_profile_id : null;
  const name = typeof identity.name === 'string' ? identity.name : (typeof extra.name === 'string' ? extra.name : '');
  const bio = typeof identity.background === 'string'
    ? identity.background
    : (typeof identity.background_story === 'string' ? identity.background_story : (typeof extra.bio === 'string' ? extra.bio : ''));
  const snapshot = {
    id: typeof source.id === 'string' ? source.id : (typeof extra.id === 'string' ? extra.id : ''),
    status: source.status ?? extra.status,
    name,
    bio,
    avatar: source.avatar || extra.avatar || '',
    mode: source.mode,
    readOnly: source.readOnly === true,
    activeProfileId,
    foundationRevision: integerOrNull(source.current_revision),
    runtimeRevision: integerOrNull(runtime.revision),
    profiles,
    historyHasMore: source.persona_switch_history_has_more === true,
    history: arrayValue(source.persona_switch_history).map(item => {
      const entry = objectValue(item);
      return { ...entry, from_profile_id: entry.previous_profile_id, to_profile_id: entry.active_profile_id, reason: entry.switch_reason };
    }),
    revisions: arrayValue(source.revisions ?? source.foundation_revisions),
    groups: [
      { key: 'identity', title: '身份', value: identity },
      { key: 'current', title: '当前状态', value: current },
      { key: 'working', title: '工作人格', value: Object.keys(compilation).length ? compilation : objectValue(source.working_persona) },
    ],
  };
  if (Array.isArray(compilation.owner_switch_targets)) {
    snapshot.switchTargetIds = compilation.owner_switch_targets
      .map(item => objectValue(item).target_profile_id)
      .filter(id => typeof id === 'string' && id !== '');
  }
  return snapshot;
}
