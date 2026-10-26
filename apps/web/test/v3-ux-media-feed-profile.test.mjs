import test from 'node:test';
import assert from 'node:assert/strict';
import { profileSnapshotFromDetail } from '../src/v3/proto/data/profile.js';
import { ProfileHeader, PersonaSection } from '../src/v3/proto/components/profile.js';
test('profile keeps summary and biography distinct and folds long authored summary', () => {
  const detail = {
    id: 'role',
    identity: {
      name: '角色',
      background_story: '简'.repeat(300),
      biography: '独立完整经历'
    },
    core_persona: {
      personality_system: {
        profiles: []
      }
    }
  };
  const snapshot = profileSnapshotFromDetail(detail);
  assert.equal(snapshot.bio, detail.identity.background_story);
  assert.equal(snapshot.groups.find(g => g.key === 'identity').value.biography, '独立完整经历');
  assert.match(ProfileHeader(snapshot, true), /阅读完整简介 · 300 字/);
  assert.match(PersonaSection(snapshot, true), /目前没有可切换的其他人格/);
  const absent = profileSnapshotFromDetail({
    ...detail,
    identity: {
      name: '角色',
      biography: '只有完整经历'
    }
  });
  assert.equal(absent.bio, '');
  assert.match(ProfileHeader(absent, true), /已有完整经历保留/);
});
