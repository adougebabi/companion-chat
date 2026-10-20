// Projects an authoritative fluctlight detail into the profile page snapshot.
// The current persona is personality_runtime only. A cleared effective appearance
// stays null. Missing layers stay empty; this module does not fill demo or seed data.

function plain(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value : null;
}

function recordedText(source, primary, fallback) {
  if (!source) return '';
  if (Object.hasOwn(source, primary)) return typeof source[primary] === 'string' ? source[primary] : '';
  if (fallback && Object.hasOwn(source, fallback)) return typeof source[fallback] === 'string' ? source[fallback] : '';
  return '';
}

function revision(value) {
  return Number.isSafeInteger(value) ? value : null;
}

// fluctlights.status is copied to the top-level detail status. Lifecycle writes
// active, paused, and retired; archived is treated as closed the same way.
function readOnlyLifecycle(status) {
  return status === 'retired' || status === 'archived';
}

function avatarUrl(detail, character) {
  const directory = plain(character);
  for (const candidate of [detail.avatar, directory?.avatar]) {
    if (typeof candidate === 'string' && candidate) return candidate;
    const record = plain(candidate);
    if (record && typeof record.url === 'string' && record.url) return record.url;
  }
  return '';
}

function profilesFrom(core) {
  const system = plain(core.personality_system);
  if (!system || !Array.isArray(system.profiles)) return [];
  return system.profiles.filter(plain);
}

function activeProfileId(runtime) {
  if (!runtime || typeof runtime.active_profile_id !== 'string') return null;
  return runtime.active_profile_id.trim() ? runtime.active_profile_id : null;
}

function historyFrom(detail) {
  if (!Array.isArray(detail.persona_switch_history)) return [];
  return detail.persona_switch_history.flatMap(item => {
    const row = plain(item);
    if (!row) return [];
    const entry = { ...row };
    if (Object.hasOwn(row, 'previous_profile_id')) entry.from_profile_id = row.previous_profile_id;
    if (Object.hasOwn(row, 'active_profile_id')) entry.to_profile_id = row.active_profile_id;
    if (Object.hasOwn(row, 'switch_reason')) entry.reason = row.switch_reason;
    return [entry];
  });
}

function switchTargets(compilation) {
  if (!compilation || !Array.isArray(compilation.owner_switch_targets)) return undefined;
  return compilation.owner_switch_targets.flatMap(item => {
    const row = plain(item);
    return row && typeof row.target_profile_id === 'string' ? [row.target_profile_id] : [];
  });
}

function workingValue(detail, compilation) {
  const value = compilation ? { ...compilation } : {};
  if (Array.isArray(detail.working_personas)) value.working_personas = detail.working_personas;
  return value;
}

function groupsFrom(detail, identity, compilation) {
  const groups = [];
  const push = (key, title, value) => groups.push({ key, title, value });
  if (Object.keys(identity).length) push('identity', '身份', { ...identity });
  if (Object.hasOwn(detail, 'current_state')) {
    const current = detail.current_state;
    push('current', '当前状态', plain(current) ? { ...current } : current);
  }
  const personality = plain(detail.personality);
  if (personality && Object.keys(personality).length) push('personality', '人格定义', { ...personality });
  if (compilation || Array.isArray(detail.working_personas)) push('working', '工作人格', workingValue(detail, compilation));
  const core = plain(detail.core_persona);
  if (core && Object.keys(core).length) push('foundation', '身份核心', { ...core });
  const developing = plain(detail.developing_self);
  if (developing) push('developing', '发展中的自我', { ...developing });
  const life = plain(detail.life_profile);
  if (life && Object.keys(life).length) push('life', '生活资料', { ...life });
  return groups;
}

/** @param {Record<string, unknown>} detail Authoritative fluctlight detail. @param {Record<string, unknown> | null | undefined} [directoryCharacter] Optional directory row, used only for a display avatar the detail does not carry. */
export function profileSnapshotFromDetail(detail, directoryCharacter) {
  const source = plain(detail) ?? {};
  const identity = plain(source.identity) ?? {};
  const runtime = plain(source.personality_runtime);
  const compilation = plain(source.persona_compilation);
  const core = plain(source.core_persona) ?? {};
  const snapshot = {
    id: typeof source.id === 'string' ? source.id : '',
    name: typeof identity.name === 'string' ? identity.name : '',
    bio: recordedText(identity, 'background_story', 'background'),
    status: typeof source.status === 'string' ? source.status : '',
    readOnly: readOnlyLifecycle(source.status),
    avatar: avatarUrl(source, directoryCharacter),
    activeProfileId: activeProfileId(runtime),
    foundationRevision: revision(source.current_revision),
    runtimeRevision: runtime ? revision(runtime.revision) : null,
    profiles: profilesFrom(core),
    history: historyFrom(source),
    historyHasMore: source.persona_switch_history_has_more === true,
    revisions: Array.isArray(source.foundation_revisions) ? source.foundation_revisions.filter(plain) : [],
    groups: groupsFrom(source, identity, compilation),
  };
  const targets = switchTargets(compilation);
  if (targets) snapshot.switchTargetIds = targets;
  return snapshot;
}
