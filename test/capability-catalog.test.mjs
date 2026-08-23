import assert from 'node:assert/strict';
import test from 'node:test';

import {
    CAPABILITY_CATALOG,
    CAPABILITY_DESCRIPTOR_ORDER,
    CAPABILITY_DESCRIPTORS,
    CAPABILITY_TOOLS,
    getAllCapabilityTools,
    getAllTools
} from '../server/application/capability-catalog.js';

const TOOL_NAMES = [
    'scene_event',
    'appearance_event',
    'media_event',
    'pending_event',
    'memory_event',
    'affect_event',
    'drive_signal'
];

function assertDeeplyFrozen(value) {
    assert.equal(Object.isFrozen(value), true);
    if (!value || typeof value !== 'object') return;
    for (const child of Object.values(value)) assertDeeplyFrozen(child);
}

test('catalog exposes the seven universal native tools in stable order', () => {
    assert.deepEqual(Object.keys(CAPABILITY_DESCRIPTORS), TOOL_NAMES);
    assert.deepEqual(CAPABILITY_DESCRIPTOR_ORDER.map(descriptor => descriptor.name), TOOL_NAMES);
    assert.deepEqual(CAPABILITY_TOOLS.map(tool => tool.function.name), TOOL_NAMES);
    assert.strictEqual(CAPABILITY_CATALOG, CAPABILITY_DESCRIPTORS);
    assert.strictEqual(getAllCapabilityTools(), CAPABILITY_TOOLS);
    assert.strictEqual(getAllTools(), CAPABILITY_TOOLS);
});

test('catalog descriptors are deeply frozen and OpenAI-compatible', () => {
    assertDeeplyFrozen(CAPABILITY_CATALOG);
    assertDeeplyFrozen(CAPABILITY_DESCRIPTOR_ORDER);
    assertDeeplyFrozen(CAPABILITY_TOOLS);

    for (const tool of CAPABILITY_TOOLS) {
        assert.deepEqual(Object.keys(tool), ['type', 'function']);
        assert.equal(tool.type, 'function');
        assert.equal(tool.function.name, tool.function.parameters && CAPABILITY_DESCRIPTORS[tool.function.name].name);
        assert.equal(typeof tool.function.description, 'string');
        assert.equal(tool.function.parameters.type, 'object');
    }
});

test('catalog preserves the current native tool schema semantics', () => {
    assert.deepEqual(CAPABILITY_TOOLS, [
        {
            type: 'function',
            function: {
                name: 'scene_event',
                description: 'Persist a material shared-scene start, switch, or end.',
                parameters: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['operation'],
                    properties: {
                        operation: {type: 'string', enum: ['start', 'switch', 'end']},
                        location: {type: 'string', maxLength: 160},
                        room: {type: 'string', maxLength: 120},
                        activity: {type: 'string', maxLength: 160},
                        situation: {type: 'string', maxLength: 240},
                        mood: {type: 'string', maxLength: 80},
                        objects: {type: 'array', maxItems: 12, items: {type: 'string', maxLength: 80}},
                        participants: {type: 'array', maxItems: 2, items: {type: 'string', enum: ['user', 'persona']}}
                    }
                }
            }
        },
        {
            type: 'function',
            function: {
                name: 'appearance_event',
                description: 'Persist a material outfit change for the current persona.',
                parameters: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['operation'],
                    properties: {
                        operation: {type: 'string', enum: ['set', 'clear']},
                        outfit: {type: 'string', maxLength: 240},
                        reason: {type: 'string', maxLength: 240}
                    }
                }
            }
        },
        {
            type: 'function',
            function: {
                name: 'media_event',
                description: 'Queue a validated image or video delivery.',
                parameters: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['kind', 'request', 'count', 'personaMediaConcept'],
                    properties: {
                        kind: {type: 'string', enum: ['image', 'video']},
                        request: {type: 'string', maxLength: 500},
                        count: {type: 'integer', minimum: 1, maximum: 3},
                        personaMediaConcept: {type: 'object'}
                    }
                }
            }
        },
        {
            type: 'function',
            function: {
                name: 'pending_event',
                description: 'Register one bounded, explicit follow-up fact for durable later evaluation.',
                parameters: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['schemaVersion', 'summary', 'notBefore', 'expiresAt', 'dedupeKey'],
                    properties: {
                        schemaVersion: {type: 'integer', enum: [1]},
                        summary: {type: 'string', minLength: 1, maxLength: 280},
                        notBefore: {type: 'string', maxLength: 80},
                        expiresAt: {type: 'string', maxLength: 80},
                        dedupeKey: {type: 'string', minLength: 1, maxLength: 120}
                    }
                }
            }
        },
        {
            type: 'function',
            function: {
                name: 'memory_event',
                description: 'Explicitly record one persona-private memory from the current user message.',
                parameters: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['memory'],
                    properties: {
                        memory: {
                            type: 'object',
                            additionalProperties: false,
                            required: ['operation', 'key', 'value', 'confidence', 'idempotencyKey'],
                            properties: {
                                schemaVersion: {type: 'integer', enum: [1]},
                                operation: {type: 'string', enum: ['insert', 'upsert']},
                                key: {type: 'string', minLength: 1, maxLength: 120},
                                value: {type: 'string', minLength: 1, maxLength: 2_000},
                                confidence: {type: 'number', minimum: 0, maximum: 1},
                                sourceType: {type: 'string', maxLength: 80},
                                sourceId: {type: 'string', maxLength: 240},
                                sourceMessageId: {type: 'string', maxLength: 160},
                                idempotencyKey: {type: 'string', minLength: 1, maxLength: 240}
                            }
                        }
                    }
                }
            }
        },
        {
            type: 'function',
            function: {
                name: 'affect_event',
                description: 'Report one bounded hidden affect event; the server owns PAD deltas.',
                parameters: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['event'],
                    properties: {
                        event: {
                            type: 'object',
                            additionalProperties: false,
                            required: ['type', 'confidence', 'idempotencyKey'],
                            properties: {
                                type: {type: 'string', enum: ['social_connection', 'social_friction', 'exploration_discovery', 'exploration_blocked', 'restored', 'fatigue']},
                                confidence: {type: 'number', minimum: 0, maximum: 1},
                                idempotencyKey: {type: 'string', minLength: 1, maxLength: 240}
                            }
                        }
                    }
                }
            }
        },
        {
            type: 'function',
            function: {
                name: 'drive_signal',
                description: 'Report one bounded drive pressure change; the server owns the magnitude.',
                parameters: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['signal'],
                    properties: {
                        signal: {
                            type: 'object',
                            additionalProperties: false,
                            required: ['drive', 'direction', 'confidence', 'idempotencyKey'],
                            properties: {
                                drive: {type: 'string', pattern: '^[a-z][a-z0-9_:-]{0,79}$'},
                                direction: {type: 'string', enum: ['increase_pressure', 'decrease_pressure', 'neutral']},
                                confidence: {type: 'number', minimum: 0, maximum: 1},
                                idempotencyKey: {type: 'string', minLength: 1, maxLength: 240}
                            }
                        }
                    }
                }
            }
        }
    ]);
});

test('tool descriptions do not expose compatibility adapter names', () => {
    const promptText = CAPABILITY_TOOLS
        .map(tool => tool.function.description)
        .join('\n');

    assert.equal(promptText.includes('media-intent'), false);
    assert.equal(promptText.includes('pending-event'), false);
    assert.equal(promptText.includes('scene-event'), false);
    assert.equal(promptText.includes('marker'), false);
});
