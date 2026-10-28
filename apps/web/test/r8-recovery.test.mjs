import test from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
import { profileFields, profileChanges } from '../src/approved-ui/profile-fields.mjs';
import { readApprovedCreationProgress } from '../src/approved-ui/creation-progress.mjs';
import { chatFailureText, chatWaitingPresentation } from '../src/v3/chat-feedback.mjs';
import { ProfileGroups } from '../src/v3/proto/components/profile.js';
test('male unknown body fields stay hidden while explicit stored values remain editable and readable', async () => {
  const core = {
    identity: {
      name: '周屿',
      gender: 'male'
    },
    behavioral_policy: {},
    life_profile: {
      appearance: {
        physical_features: {
          chest_cup: null,
          hair_color: '黑色'
        }
      }
    }
  };
  const fields = profileFields(core);
  assert.ok(fields.some(f => f.path.at(-1) === 'hair_color'));
  assert.ok(!fields.some(f => f.path.at(-1) === 'chest_cup'));
  assert.deepEqual(profileChanges(core, fields), {});
  assert.equal(core.life_profile.appearance.physical_features.chest_cup, null);
  const legacy = ProfileGroups({
    groups: [{
      key: 'identity',
      value: core.identity
    }, {
      key: 'current',
      value: {
        appearance: {
          body_fields: {
            chest_cup: {
              status: 'unknown',
              value: null
            }
          }
        }
      }
    }]
  });
  assert.doesNotMatch(legacy.split('<details class="card profile-technical">')[0], /胸部尺寸/);
  const vite = await createServer({
    root: fileURLToPath(new URL('../', import.meta.url)),
    appType: 'custom',
    logLevel: 'silent',
    server: {
      middlewareMode: true
    }
  });
  try {
    const {
      default: Tree
    } = await vite.ssrLoadModule('/src/react-app/shared/SemanticTree.jsx');
    const render = value => renderToStaticMarkup(React.createElement(Tree,{value,identity:core.identity}));
    let html = await render({
      body_fields: {
        chest_cup: {
          status: 'unknown',
          value: null
        },
        hair_color: {
          status: 'known',
          value: '黑色'
        }
      }
    });
    assert.doesNotMatch(html, /胸部尺寸/);
    assert.match(html, /黑色/);
    core.life_profile.appearance.physical_features.chest_cup = '用户原有记录';
    assert.ok(profileFields(core).some(f => f.path.at(-1) === 'chest_cup'));
    html = await render({
      chest_cup: '用户原有记录'
    });
    assert.match(html, /用户原有记录/);
  } finally {
    await vite.close();
  }
});
test('initial schedule precedes image stages and image failure does not relabel it blocked', async () => {
  const progress = await readApprovedCreationProgress({
    visualIdentityManagement: async () => ({
      session: {
        status: 'failed',
        error_code: 'image_unconfigured'
      }
    }),
    displayAvatar: async () => ({
      avatar: null
    }),
    visualIdentityTimeline: async () => ({
      items: []
    }),
    inventoryPreparation: async () => ({
      status: 'completed'
    }),
    schedules: async () => ({
      days: []
    }),
    lifecycleDiagnostics: async () => ({
      workflowIntents: [{
        intentType: 'schedule.current_day',
        status: 'pending',
        intentId: 'original-schedule'
      }]
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
    activationDate: '2026-10-25',
    mode: 'blank'
  });
  assert.deepEqual(progress.stages.map(s => s.key), ['analysis', 'world', 'wardrobe', 'schedule', 'reference', 'sheet', 'avatar']);
  assert.equal(progress.stages.find(s => s.key === 'schedule').state, 'pending');
  for (const code of ['agent_partial_committed', 'request_timeout', 'unavailable', 'unknown']) assert.doesNotMatch(chatFailureText(code), /她/);
  assert.equal(chatWaitingPresentation({
    sending: true
  }, 0, 40000, '周屿').detail, '周屿还在想…');
});

test('creation waiting for candidate review explains the human action without counting unread images as ready', async () => {
  const draft={activatedId:'A',worldRuntimeId:'W',groupConfirmed:true,activationDate:'2026-10-25',mode:'blank'};
  const client={
    visualIdentityManagement:async()=>({fluctlight_id:'A',session:{id:'session',status:'awaiting_review'}}),
    displayAvatar:async()=>({fluctlight_id:'A',avatar:null}),
    visualIdentityTimeline:async()=>({items:[]}),
    inventoryPreparation:async()=>({status:'completed'}),
    schedules:async()=>({days:[{local_date:draft.activationDate,schedule:{id:'S'}}]}),
    lifecycleDiagnostics:async()=>({workflowIntents:[]}),
    world:async()=>({world:{id:'W'}}),
    detail:async()=>({persona_preparation:{status:'ready'}}),
    ownerMediaAsset:async()=>({readable:true}),
  };
  const progress=await readApprovedCreationProgress(client,draft),reference=progress.stages.find(s=>s.key==='reference');
  assert.equal(reference.state,'pending');assert.match(reference.note,/等待明确评审/);assert.equal(reference.linkLabel,'前往审阅候选');assert.equal(reference.href,'#/c/A/media/identity');assert.equal(progress.complete,false);assert.equal(progress.stages.filter(s=>s.state==='ready').length,4);
  client.visualIdentityManagement=async()=>({fluctlight_id:'A',canonical_asset_id:'C',character_sheet_asset_id:'S',session:{id:'session',status:'awaiting_review'}});
  client.displayAvatar=async()=>({fluctlight_id:'A',avatar:{url:'/api/media/C'}});
  const confirmed=await readApprovedCreationProgress(client,draft);
  assert.equal(confirmed.complete,true);assert.equal(confirmed.stages.find(s=>s.key==='reference').linkLabel,undefined);
});
