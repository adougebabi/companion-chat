import assert from 'node:assert/strict';
import test from 'node:test';

import {
    contextPromptFor,
    systemCapabilityContracts,
    systemCapabilityPromptFor
} from '../server/application/context-contracts.js';

test('capability guidance stays concise and omits marker fallback instructions', () => {
    const guidance = systemCapabilityPromptFor();
    assert.equal(guidance, systemCapabilityContracts.join('\n'));
    assert.ok(guidance.length < 1_200);
    assert.doesNotMatch(guidance, /media-intent|pending-event|scene-event/);
});

test('an explicit prompt with an included capability layer is not appended twice', () => {
    const capability = systemCapabilityPromptFor();
    const prompt = contextPromptFor({
        prompt: `身份\n${capability}`,
        capabilityPromptIncluded: true,
        layers: {systemCapability: capability}
    });
    assert.equal(prompt, `身份\n${capability}`);
    assert.equal(prompt.split('media_event').length - 1, 1);
});

test('custom structured contexts still receive one capability layer', () => {
    const prompt = contextPromptFor({prompt: '身份', layers: {systemCapability: '自定义工具说明'}});
    assert.equal(prompt, '身份\n\n自定义工具说明');
});
