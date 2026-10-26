import test from 'node:test';
import assert from 'node:assert/strict';
import { configurationPlan, weekdays } from '../src/approved-ui/configuration-plan.mjs';
import { wardrobePreparationStage } from '../src/approved-ui/wardrobe-preparation.mjs';
import { readApprovedCreationProgress } from '../src/approved-ui/creation-progress.mjs';
import { notificationGroups, notificationIssueGroups, notificationPath } from '../src/approved-ui/presentation.mjs';
import { personaPresentation, semanticEntries, portraitPresentation, workingPersonaPresentation } from '../src/approved-ui/profile-presentation.mjs';
import { profileFields, profileChanges } from '../src/approved-ui/profile-fields.mjs';
import { creationFeedback } from '../src/v3/creation-feedback.mjs';
import { errorMessage } from '../src/v3/domain-presentation.mjs';
test('optional model timeout uses 120 seconds while malformed explicit values stay invalid', () => {
  const input = {
    catalog: {
      versions: {},
      effective_values: {}
    },
    settings: [],
    endpoints: [{
      id: 'e',
      name: 'e',
      url: 'https://model.example/v1',
      protocol: 'openai-compatible'
    }],
    bindings: [{
      role: 'generic_llm',
      name: '通用模型',
      endpoint: 'e',
      model: 'm',
      timeout: ''
    }],
    quiet: {
      enabled: false,
      start: '',
      end: '',
      days: [...weekdays],
      behavior: '暂停主动联系',
      character: '全部角色'
    },
    characters: []
  };
  assert.equal(configurationPlan(input).find(w => w.operation === 'binding').body.timeoutSeconds, 120);
  for (const timeout of [0, -1, 1.5, 'not-a-number']) {
    input.bindings[0].timeout = timeout;
    assert.throws(() => configurationPlan(input), /超时/);
  }
});
test('exhausted wardrobe authority is shared by both pages and blocks successors visibly', async () => {
  for (const status of ['failed', 'generating', 'queued']) assert.equal(wardrobePreparationStage({
    status,
    can_recover: true
  }).state, 'failed');
  assert.equal(wardrobePreparationStage({
    status: 'awaiting_wear'
  }).state, 'running');
  const progress = await readApprovedCreationProgress({
    visualIdentityManagement: async () => ({
      fluctlight_id: 'A',
      session: {
        status: 'queued'
      }
    }),
    displayAvatar: async () => ({
      fluctlight_id: 'A',
      avatar: null
    }),
    visualIdentityTimeline: async () => ({
      items: []
    }),
    inventoryPreparation: async () => ({
      status: 'generating',
      can_recover: true,
      task_id: 'original',
      result: {
        failure_reason: '自动重试已经结束'
      }
    }),
    schedules: async () => ({
      days: []
    }),
    lifecycleDiagnostics: async () => ({
      workflowIntents: []
    }),
    world: async () => ({
      world: {
        id: 'W'
      }
    }),
    detail: async () => ({
      persona_preparation: {
        status: 'ready'
      }
    })
  }, {
    activatedId: 'A',
    worldRuntimeId: 'W',
    groupConfirmed: true,
    activationDate: '2026-10-24',
    mode: 'blank'
  });
  assert.equal(progress.blocker.key, 'wardrobe');
  assert.equal(progress.complete, false);
  assert.equal(progress.stages[2].retryable, true);
  assert.match(progress.stages[2].href, /original/);
  for (const stage of progress.stages.slice(3)) {
    assert.equal(stage.state, 'blocked');
    assert.equal(stage.blockedBy, 'wardrobe');
    assert.match(stage.note, /初始衣柜/);
  }
});
test('notice grouping preserves actual IDs, hash destinations and original failure records', () => {
  const rows = [{
    id: 'one',
    fluctlight_id: 'A',
    link: '#/c/A/chat?conversation=C&message=M'
  }, {
    id: 'two',
    fluctlight_id: 'A',
    link: '#/feed/moment'
  }, {
    id: 'three',
    fluctlight_id: 'A',
    link: '#/c/A/chat?conversation=C&message=M'
  }];
  const groups = notificationGroups(rows, [{
    id: 'A',
    name: '角色'
  }]);
  assert.equal(groups.length, 1);
  assert.equal(groups[0].notices.length, 3);
  assert.equal(notificationPath(rows[1].link), '/feed/moment');
  assert.equal(notificationPath(rows[0].link), '/c/A/chat?conversation=C&message=M');
  for (const link of ['//foreign', 'https://foreign', '#//foreign', '/\\foreign']) assert.equal(notificationPath(link), null);
  const issues = notificationIssueGroups([1, 2, 3].map(n => ({
    id: 'attempt' + n,
    label: '角色 · 人格整理',
    code: 'invalid_json',
    fid: 'A',
    reason: '失败'
  })));
  assert.equal(issues.length, 1);
  assert.equal(issues[0].items.length, 3);
});
test('profile projection removes exact aliases while retaining conflicting and custom facts', () => {
  assert.deepEqual(semanticEntries({
    agreeableness: .5,
    traits: {
      agreeableness: .5
    },
    custom: '保留'
  }), [['traits', {
    agreeableness: .5
  }], ['custom', '保留']]);
  assert.equal(semanticEntries({
    agreeableness: .2,
    traits: {
      agreeableness: .5
    }
  }).length, 2);
  const shared = {
      personality: {
        agreeableness: .5
      },
      identity: {
        name: '角色'
      }
    },
    profile = {
      id: 'p',
      name: '人格',
      ...shared,
      personality: {
        agreeableness: .5,
        curiosity: .8
      },
      custom: '自定义'
    };
  assert.deepEqual(personaPresentation(profile, shared), {
    id: 'p',
    name: '人格',
    personality: {
      curiosity: .8
    },
    custom: '自定义'
  });
  assert.equal(profile.personality.agreeableness, .5);
  assert.deepEqual(workingPersonaPresentation({
    status: 'ready',
    working_persona: {
      profile_id: 'p',
      working_persona: {
        profile_id: 'p',
        portrait_text: '原画像'
      }
    }
  }), {
    status: 'ready',
    profile_id: 'p',
    portrait_text: '原画像'
  });
  assert.deepEqual(workingPersonaPresentation({
    status: 'ready',
    working_persona: {
      status: 'conflicting',
      portrait_text: '原画像'
    }
  }), {
    status: 'ready',
    working_persona: {
      status: 'conflicting',
      portrait_text: '原画像'
    }
  });
  const lines = portraitPresentation('角色\nagreeableness: 0.5\nagreeableness: 0.5\n原文里提到 agreeableness: 0.5');
  assert.equal(lines.length, 3);
  assert.deepEqual(lines[1], {
    field: 'agreeableness',
    value: .5
  });
  assert.match(lines[2].text, /原文/);
  const base = {
    identity: {},
    behavioral_policy: {
      message_length: 'short'
    },
    life_profile: {}
  };
  const fields = profileFields(base),
    length = fields.find(f => f.path.at(-1) === 'message_length');
  assert.equal(length.kind, 'select');
  assert.equal(length.options.find(o => o.value === 'short').label, '简短');
  assert.deepEqual(profileChanges(base, fields), {});
  length.value = 'long';
  assert.deepEqual(profileChanges(base, fields), {
    behavioral_policy: {
      message_length: 'long'
    }
  });
  const unknown = profileFields({
    ...base,
    behavioral_policy: {
      message_length: 'custom'
    }
  }).find(f => f.path.at(-1) === 'message_length');
  assert.ok(unknown.options.some(o => o.value === 'custom'));
});
test('model output errors do not blame user input and missing embedding explains saved memory', () => {
  const feedback = creationFeedback({
    code: 'initialization_response_invalid_json',
    details: {
      validation_error: {
        path: 'provider_response'
      }
    }
  });
  assert.doesNotMatch(feedback.message, /修正原输入|问题字段/);
  assert.match(feedback.message, /原描述已保留/);
  const semantic = creationFeedback({
    code: 'initialization_persona_invalid',
    details: {
      validation_error: {
        path: 'initial_intentions[0].goal_index'
      }
    }
  });
  assert.doesNotMatch(semantic.message, /修正原输入/);
  assert.match(semantic.message, /模型返回.*字段校验/);
  assert.equal(semantic.path, 'initial_intentions[0].goal_index');
  assert.match(errorMessage({
    code: 'memory_embedding_unconfigured'
  }), /正文已保存.*向量索引未完成/);
});
