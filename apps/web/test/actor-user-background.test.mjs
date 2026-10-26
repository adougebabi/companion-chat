import assert from "node:assert/strict";
import test from "node:test";
import { parseActorUserSettings } from "../src/lib/actor-user-background.ts";
test("explicit user background preserves false, unknown and omitted attributes", () => {
  const input = {
    background: {
      name: "Vinson",
      timezone: null,
      meeting_confirmed: false
    }
  };
  assert.deepEqual(parseActorUserSettings(input), input);
  assert.equal(parseActorUserSettings({
    background: {
      meeting_confirmed: "false"
    }
  }), null);
  assert.equal(parseActorUserSettings({
    background: {
      actor_id: "foreign"
    }
  }), null);
  assert.equal(parseActorUserSettings({
    background: {
      timezone: ""
    }
  }), null);
  assert.equal(parseActorUserSettings(null), null);
  assert.deepEqual(parseActorUserSettings({
    background: {}
  }), {
    background: {}
  });
});
test("all confirmed about-you fields have independent canonical keys and preserve clears", async () => {
  const {
    actorUserBackgroundFields,
    backgroundFromDraft,
    backgroundDraft
  } = await import('../src/lib/actor-user-background.ts');
  const expected = {
    full_name: '姓名',
    age: '年龄',
    gender: '性别',
    personality: '性格',
    habits: '习惯',
    relationship_description: '你们的关系（自述）',
    shared_information: '可让她知道',
    wants_you_to_know: '希望她知道的事',
    rules: '给她的规则与边界'
  };
  for (const [key, label] of Object.entries(expected)) assert.ok(actorUserBackgroundFields.some(f => f.key === key && f.label === label));
  const background = Object.fromEntries(actorUserBackgroundFields.map(({
    key
  }) => [key, key === 'meeting_confirmed' ? false : key === 'age' ? null : key === 'rules' ? '先询问\n再建议' : key]));
  assert.deepEqual(parseActorUserSettings({
    background
  }), {
    background
  });
  assert.deepEqual(backgroundFromDraft(backgroundDraft(background)), background);
  for (const key of Object.keys(expected)) {
    assert.deepEqual(parseActorUserSettings({
      background: {
        [key]: null
      }
    }), {
      background: {
        [key]: null
      }
    });
    assert.notEqual(parseActorUserSettings({
      background: {
        [key]: '明'.repeat(1024)
      }
    }), null);
    assert.equal(parseActorUserSettings({
      background: {
        [key]: '明'.repeat(1025)
      }
    }), null);
    assert.equal(parseActorUserSettings({
      background: {
        [key]: ['unsupported']
      }
    }), null);
  }
});
