import assert from 'node:assert/strict';
import test from 'node:test';
import { profileSnapshotFromDetail } from '../src/v3/proto/data/profile.js';
import { ProfileGroups, ProfileHeader, PersonaSection } from '../src/v3/proto/components/profile.js';
function snapshot(active = 'warm', runtimeRevision = 2) {
  return profileSnapshotFromDetail({
    id: 'fixture-role',
    status: 'active',
    current_revision: 4,
    identity: {
      name: '<角色>',
      background: '真实背景',
      timezone: 'Asia/Shanghai'
    },
    core_persona: {
      initial_state: {
        appearance: '旧初始外观'
      },
      personality_system: {
        active_profile_id: 'old',
        profiles: [{
          id: 'warm',
          name: '日常人格'
        }, {
          id: 'focused',
          name: '专注人格'
        }]
      }
    },
    personality_runtime: {
      active_profile_id: active,
      revision: runtimeRevision
    },
    current_state: {
      appearance: null,
      context: {
        location: '有效地点'
      }
    },
    working_personas: [{
      id: 'warm',
      source_revision: 4
    }],
    developing_self: {
      claims: []
    }
  });
}
// Minimal DOM binding harness; markup semantics are checked separately below.

test('profile adapter preserves cleared effective appearance and runtime authority', () => {
  const value = snapshot();
  assert.equal(value.activeProfileId, 'warm');
  assert.equal(value.groups.find(g => g.key === 'current').value.appearance, null);
  assert.equal(value.foundationRevision, 4);
  assert.equal(value.runtimeRevision, 2);
  assert.equal(profileSnapshotFromDetail({
    id: 'empty',
    core_persona: {
      personality_system: {
        active_profile_id: 'initial-only'
      }
    }
  }).activeProfileId, null);
  const html = ProfileGroups(value);
  assert.match(html, /未记录 \/ 已清除/);
  assert.match(html, /工作人格/);
  assert.match(ProfileHeader(value, true), /&lt;角色&gt;/);
  assert.doesNotMatch(html, /<角色>/);
});
test('unwired command visibly disables switching without fabricating a current persona', () => {
  const html = PersonaSection(snapshot(), false);
  assert.match(html, /profile-switch-unavailable/);
  assert.match(html, /profile-switch-submit" disabled/);
  const empty = profileSnapshotFromDetail({
    id: 'empty'
  });
  assert.equal(empty.name, '');
  assert.equal(empty.profiles.length, 0);
  assert.match(ProfileHeader(empty, false), /姓名尚未记录/);
});
test('owner persona projection limits legal switch targets and retains bounded history metadata', () => {
  const value = profileSnapshotFromDetail({
    id: 'fixture',
    persona_compilation: {
      status: 'ready',
      working_persona: {
        name: '工作人格'
      },
      owner_switch_targets: [{
        target_profile_id: 'focused'
      }]
    },
    persona_switch_history_has_more: true,
    persona_switch_history: [{
      previous_profile_id: 'warm',
      active_profile_id: 'focused',
      switch_reason: '工作',
      revision: 3
    }]
  });
  assert.deepEqual(value.switchTargetIds, ['focused']);
  assert.equal(value.historyHasMore, true);
  assert.equal(value.history[0].from_profile_id, 'warm');
  assert.equal(value.history[0].to_profile_id, 'focused');
  assert.equal(value.history[0].reason, '工作');
  assert.equal(value.groups.find(g => g.key === 'working').value.status, 'ready');
  const role = snapshot();
  role.switchTargetIds = [];
  assert.match(PersonaSection(role, true), /profile-switch-submit" disabled/);
});
