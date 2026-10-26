import { test } from 'node:test';
import assert from 'node:assert/strict';
import { worldDefinitionError, newWorldDefinition } from '../src/v3/proto/components/world-definition.js';
test('world source rejects meaningless shells without inventing defaults and accepts legacy authored narrative', () => {
  assert.match(worldDefinitionError({}), /空世界底稿/);
  assert.match(worldDefinitionError(newWorldDefinition()), /空世界底稿/);
  assert.match(worldDefinitionError({
    background: '  ',
    rules: '\n'
  }), /空世界底稿/);
  assert.equal(worldDefinitionError({
    background: '山间小城，居民过着平静生活',
    custom: {
      unknown: 'preserved'
    }
  }), '');
  assert.deepEqual(newWorldDefinition(), {
    background: '',
    rules: ''
  });
});
test('world framework validates actual category/place identity, limits and forbidden runtime references', () => {
  const body = {
    background: '由所有者编写的世界',
    framework: {
      schema: 'world-framework.v1',
      venues: [{
        key: 'home',
        name: '居所',
        places: [{
          key: 'room',
          name: '卧室',
          description: '安静的房间'
        }]
      }]
    }
  };
  assert.equal(worldDefinitionError(body), '');
  const change = fn => {
    const draft = structuredClone(body);
    fn(draft.framework);
    return worldDefinitionError(draft);
  };
  assert.match(change(f => f.schema = 'invented'), /world-framework.v1/);
  assert.match(change(f => f.venues = []), /1–32/);
  assert.match(change(f => f.venues[0].key = 123), /唯一/);
  assert.match(change(f => f.venues[0].places[0].name = 456), /唯一/);
  assert.match(change(f => f.venues.push(structuredClone(f.venues[0]))), /唯一/);
  assert.match(change(f => f.venues[0].places = []), /1–200/);
  assert.match(change(f => f.venues[0].places[0].place_id = 'runtime-place'), /运行世界ID/);
  assert.match(change(f => f.venues[0].places[0].description = '字'.repeat(8001)), /描述/);
  assert.equal(change(f => f.venues[0].places[0].atmosphere = '字'.repeat(2000)), '');
});
